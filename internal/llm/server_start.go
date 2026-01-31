package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"aichatplayers/internal/config"
	"aichatplayers/internal/logging"
)

const defaultServerCommand = "llama-server"
const serverStateFilename = "llm_server_state.json"

var errServerStateMissing = errors.New("llm server state missing")

type ServerProcess struct {
	cmd    *exec.Cmd
	exitCh chan error
	url    string
}

type serverState struct {
	URL     string   `json:"url"`
	Command string   `json:"command"`
	Args    []string `json:"args"`
	PID     int      `json:"pid"`
}

func EnsureServerReady(cfg config.LLMConfig) (*ServerProcess, error) {
	serverURL := strings.TrimSpace(cfg.ServerURL)
	modelPath := strings.TrimSpace(resolveModelPath(&cfg))
	if serverURL == "" || modelPath == "" {
		logging.Debugf("llm_server_start_skipped server_url=%q model_path=%q", serverURL, modelPath)
		return nil, nil
	}

	command := strings.TrimSpace(cfg.ServerCommand)
	if command == "" {
		command = defaultServerCommand
	}
	resolvedCommand, ok := resolveCommandPath(command, defaultServerCommand, &cfg)
	if !ok {
		logging.Warnf("llm_server_command_missing command=%s", command)
	} else {
		logging.Debugf("llm_server_command_resolved command=%s path=%s", command, resolvedCommand)
		command = resolvedCommand
	}

	host, port, err := hostPortForURL(serverURL)
	if err != nil {
		return nil, err
	}

	args := []string{"--model", modelPath, "--host", host, "--port", port}
	if cfg.CtxSize > 0 {
		args = append(args, "--ctx-size", fmt.Sprint(cfg.CtxSize))
	}
	if cfg.NumThreads > 0 {
		args = append(args, "--threads", fmt.Sprint(cfg.NumThreads))
	}

	desiredState := serverState{
		URL:     serverURL,
		Command: command,
		Args:    args,
	}

	client := &http.Client{Timeout: 750 * time.Millisecond}
	if err := checkServerReady(client, serverURL); err == nil {
		restartNeeded, existingState, err := needsServerRestart(desiredState)
		if err != nil {
			if errors.Is(err, errServerStateMissing) {
				logging.Warnf("llm_server_state_missing url=%s path=%s", serverURL, serverStatePath())
				restartNeeded = true
			} else {
				logging.Warnf("llm_server_state_read_failed url=%s error=%v", serverURL, err)
			}
		}
		if !restartNeeded {
			logging.Infof("llm_server_detected url=%s status=ready", serverURL)
			if existingState != nil && existingState.PID > 0 {
				attachServerLogs(existingState.PID)
			} else {
				logging.Warnf("llm_server_log_attach_skipped url=%s reason=missing_pid", serverURL)
			}
			return nil, nil
		}

		logging.Infof("llm_server_restart_required url=%s", serverURL)
		if err := restartRunningServer(serverURL, existingState); err != nil {
			return nil, err
		}
	} else {
		logging.Debugf("llm_server_not_ready url=%s error=%v", serverURL, err)
	}

	if stat, err := os.Stat(modelPath); err != nil {
		logging.Warnf("llm_server_model_unavailable path=%s error=%v", modelPath, err)
	} else {
		logging.Debugf("llm_server_model_found path=%s size_bytes=%d", modelPath, stat.Size())
	}

	cmd := exec.Command(command, args...)
	configureCommand(cmd)
	cmd.Stdout = log.Writer()
	cmd.Stderr = log.Writer()

	logging.Infof("llm_server_starting command=%s args=%s url=%s", command, strings.Join(args, " "), serverURL)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("llm server start: %w", err)
	}
	if err := writeServerState(desiredState, cmd.Process.Pid); err != nil {
		logging.Warnf("llm_server_state_write_failed url=%s error=%v", serverURL, err)
	}

	proc := &ServerProcess{
		cmd:    cmd,
		exitCh: make(chan error, 1),
		url:    serverURL,
	}
	go func() {
		proc.exitCh <- cmd.Wait()
	}()

	timeout := cfg.ServerStartupTimeout
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	logging.Debugf("llm_server_waiting url=%s timeout=%s", serverURL, timeout)
	if err := waitForServerReady(serverURL, timeout, proc.exitCh); err != nil {
		_ = proc.Close()
		return nil, err
	}

	logging.Infof("llm_server_ready url=%s", serverURL)
	return proc, nil
}

func (p *ServerProcess) Close() error {
	if p == nil || p.cmd == nil || p.cmd.Process == nil {
		return nil
	}

	logging.Infof("llm_server_stopping url=%s pid=%d", p.url, p.cmd.Process.Pid)
	if err := p.cmd.Process.Signal(interruptSignal()); err != nil {
		return fmt.Errorf("llm server signal: %w", err)
	}

	select {
	case err := <-p.exitCh:
		if err != nil {
			return fmt.Errorf("llm server stop: %w", err)
		}
		_ = removeServerState()
		return nil
	case <-time.After(5 * time.Second):
		if killErr := p.cmd.Process.Kill(); killErr != nil {
			return fmt.Errorf("llm server kill: %w", killErr)
		}
		_ = removeServerState()
		return nil
	}
}

func waitForServerReady(serverURL string, timeout time.Duration, exitCh <-chan error) error {
	client := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		if err := checkServerReady(client, serverURL); err == nil {
			return nil
		} else {
			lastErr = err
		}

		select {
		case err := <-exitCh:
			if err == nil {
				return errors.New("llm server exited before ready")
			}
			return fmt.Errorf("llm server exited: %w", err)
		case <-time.After(300 * time.Millisecond):
		}

		if time.Now().After(deadline) {
			if lastErr != nil {
				return fmt.Errorf("llm server start timeout after %s: last_error=%w", timeout, lastErr)
			}
			return fmt.Errorf("llm server start timeout after %s", timeout)
		}
	}
}

func checkServerReady(client *http.Client, serverURL string) error {
	healthURL := strings.TrimRight(serverURL, "/") + "/health"
	req, err := http.NewRequest(http.MethodGet, healthURL, nil)
	if err == nil {
		resp, err := client.Do(req)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
				return nil
			}
		}
	}

	payload := map[string]any{
		"prompt":    "ping",
		"n_predict": 1,
		"stream":    false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("llm server ready check encode: %w", err)
	}
	endpoint := strings.TrimRight(serverURL, "/") + "/completion"
	request, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("llm server ready check: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("llm server ready check: %w", err)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	return fmt.Errorf("llm server ready check status=%d", resp.StatusCode)
}

func needsServerRestart(desired serverState) (bool, *serverState, error) {
	state, err := readServerState()
	if err != nil || state == nil {
		if err == nil && state == nil {
			return false, nil, errServerStateMissing
		}
		return false, nil, err
	}
	return !state.matches(desired), state, nil
}

func restartRunningServer(serverURL string, state *serverState) error {
	if state == nil || state.PID == 0 {
		logging.Warnf("llm_server_restart_missing_pid url=%s", serverURL)
		return stopServerByURL(serverURL)
	}
	if err := stopServerByPID(state.PID, serverURL); err != nil {
		return err
	}
	return nil
}

func stopServerByPID(pid int, serverURL string) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("llm server find process: %w", err)
	}
	logging.Infof("llm_server_stopping pid=%d url=%s", pid, serverURL)
	if err := proc.Signal(interruptSignal()); err != nil {
		logging.Warnf("llm_server_signal_failed pid=%d error=%v", pid, err)
	}
	if err := waitForServerStop(serverURL, 5*time.Second); err == nil {
		_ = removeServerState()
		return nil
	}
	if err := proc.Kill(); err != nil {
		return fmt.Errorf("llm server kill: %w", err)
	}
	if err := waitForServerStop(serverURL, 5*time.Second); err != nil {
		return err
	}
	_ = removeServerState()
	return nil
}

func stopServerByURL(serverURL string) error {
	client := &http.Client{Timeout: 1 * time.Second}
	endpoints := []string{"/shutdown", "/exit"}
	methods := []string{http.MethodPost, http.MethodGet}
	base := strings.TrimRight(serverURL, "/")
	for _, endpoint := range endpoints {
		for _, method := range methods {
			req, err := http.NewRequest(method, base+endpoint, nil)
			if err != nil {
				continue
			}
			resp, err := client.Do(req)
			if err != nil {
				continue
			}
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
				logging.Infof("llm_server_shutdown_requested url=%s method=%s endpoint=%s", serverURL, method, endpoint)
				if err := waitForServerStop(serverURL, 5*time.Second); err == nil {
					_ = removeServerState()
					return nil
				}
			} else {
				logging.Debugf("llm_server_shutdown_rejected url=%s method=%s endpoint=%s status=%d", serverURL, method, endpoint, resp.StatusCode)
			}
		}
	}
	return fmt.Errorf("llm server stop request failed url=%s", serverURL)
}

func waitForServerStop(serverURL string, timeout time.Duration) error {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := checkServerReady(client, serverURL); err != nil {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return fmt.Errorf("llm server stop timeout after %s", timeout)
}

func hostPortForURL(serverURL string) (string, string, error) {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return "", "", fmt.Errorf("llm server url parse: %w", err)
	}
	host := parsed.Hostname()
	if host == "" {
		return "", "", errors.New("llm server url missing host")
	}
	port := parsed.Port()
	if port == "" {
		switch parsed.Scheme {
		case "https":
			port = "443"
		default:
			port = "80"
		}
	}
	if net.ParseIP(host) == nil && !strings.EqualFold(host, "localhost") {
		logging.Warnf("llm_server_non_localhost url=%s host=%s", serverURL, host)
	}
	return host, port, nil
}

func (s serverState) matches(other serverState) bool {
	if s.URL != other.URL || s.Command != other.Command {
		return false
	}
	if len(s.Args) != len(other.Args) {
		return false
	}
	for i := range s.Args {
		if s.Args[i] != other.Args[i] {
			return false
		}
	}
	return true
}

func serverStatePath() string {
	return filepath.Join("logs", serverStateFilename)
}

func readServerState() (*serverState, error) {
	path := serverStatePath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var state serverState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

func writeServerState(state serverState, pid int) error {
	state.PID = pid
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(serverStatePath(), data, 0o644)
}

func removeServerState() error {
	err := os.Remove(serverStatePath())
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func attachServerLogs(pid int) {
	if pid <= 0 {
		logging.Warnf("llm_server_log_attach_skipped reason=invalid_pid pid=%d", pid)
		return
	}
	if runtime.GOOS != "linux" {
		logging.Warnf("llm_server_log_attach_skipped reason=unsupported_os os=%s pid=%d", runtime.GOOS, pid)
		return
	}
	if _, err := os.Stat("/proc"); err != nil {
		logging.Warnf("llm_server_log_attach_skipped reason=missing_proc pid=%d error=%v", pid, err)
		return
	}
	for _, fd := range []string{"1", "2"} {
		path := fmt.Sprintf("/proc/%d/fd/%s", pid, fd)
		file, err := os.Open(path)
		if err != nil {
			logging.Warnf("llm_server_log_attach_failed pid=%d fd=%s error=%v", pid, fd, err)
			continue
		}
		logging.Infof("llm_server_log_attached pid=%d fd=%s", pid, fd)
		go func(f *os.File, fd string) {
			defer f.Close()
			if _, err := io.Copy(log.Writer(), f); err != nil {
				logging.Warnf("llm_server_log_stream_failed pid=%d fd=%s error=%v", pid, fd, err)
			}
		}(file, fd)
	}
}
