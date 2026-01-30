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
	"strings"
	"time"

	"aichatplayers/internal/config"
	"aichatplayers/internal/logging"
)

const defaultServerCommand = "llama-server"

type ServerProcess struct {
	cmd    *exec.Cmd
	exitCh chan error
	url    string
}

func EnsureServerReady(cfg config.LLMConfig) (*ServerProcess, error) {
	serverURL := strings.TrimSpace(cfg.ServerURL)
	modelPath := strings.TrimSpace(cfg.ModelPath)
	if serverURL == "" || modelPath == "" {
		logging.Debugf("llm_server_start_skipped server_url=%q model_path=%q", serverURL, modelPath)
		return nil, nil
	}

	client := &http.Client{Timeout: 750 * time.Millisecond}
	if err := checkServerReady(client, serverURL); err == nil {
		logging.Infof("llm_server_detected url=%s status=ready", serverURL)
		return nil, nil
	} else {
		logging.Debugf("llm_server_not_ready url=%s error=%v", serverURL, err)
	}

	command := strings.TrimSpace(cfg.ServerCommand)
	if command == "" {
		command = defaultServerCommand
	}
	if resolved, err := exec.LookPath(command); err != nil {
		logging.Warnf("llm_server_command_missing command=%s error=%v", command, err)
	} else {
		logging.Debugf("llm_server_command_resolved command=%s path=%s", command, resolved)
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
		return nil
	case <-time.After(5 * time.Second):
		if killErr := p.cmd.Process.Kill(); killErr != nil {
			return fmt.Errorf("llm server kill: %w", killErr)
		}
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
