package llm

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"aichatplayers/internal/config"
	"aichatplayers/internal/logging"
)

func resolveModelPath(cfg *config.LLMConfig) string {
	if cfg == nil {
		return ""
	}
	modelPath := strings.TrimSpace(cfg.ModelPath)
	if modelPath != "" {
		if fileExists(modelPath) {
			return modelPath
		}
		if alt := windowsTrimRootedPath(modelPath); alt != "" && alt != modelPath && fileExists(alt) {
			logging.Infof("llm_model_path_resolved original=%s resolved=%s", modelPath, alt)
			cfg.ModelPath = alt
			return alt
		}
		for _, dir := range modelDirCandidates(cfg) {
			candidate := filepath.Join(dir, modelPath)
			if fileExists(candidate) {
				logging.Infof("llm_model_path_resolved original=%s resolved=%s", modelPath, candidate)
				cfg.ModelPath = candidate
				return candidate
			}
		}
		logging.Debugf("llm_model_path_missing path=%s", modelPath)
	}

	for _, dir := range modelDirCandidates(cfg) {
		candidate := firstGGUF(dir)
		if candidate != "" {
			logging.Infof("llm_model_path_auto_detected path=%s", candidate)
			cfg.ModelPath = candidate
			return candidate
		}
	}
	return modelPath
}

func resolveCommandPath(command string, defaultName string, cfg *config.LLMConfig) (string, bool) {
	name := strings.TrimSpace(command)
	if name == "" {
		name = defaultName
	}
	if hasPathSeparator(name) || filepath.IsAbs(name) {
		if fileExists(name) {
			return name, true
		}
	}
	if resolved, err := exec.LookPath(name); err == nil {
		return resolved, true
	}

	for _, dir := range modelDirCandidates(cfg) {
		for _, candidate := range commandCandidates(dir, name) {
			if fileExists(candidate) {
				return candidate, true
			}
		}
	}
	return name, false
}

func modelDirCandidates(cfg *config.LLMConfig) []string {
	if cfg != nil && strings.TrimSpace(cfg.ModelsDir) != "" {
		dir := strings.TrimSpace(cfg.ModelsDir)
		candidates := []string{dir}
		if alt := windowsTrimRootedPath(dir); alt != "" && alt != dir {
			candidates = append(candidates, alt)
		}
		return candidates
	}
	return []string{"models", string(filepath.Separator) + "models"}
}

func commandCandidates(dir, name string) []string {
	candidates := []string{filepath.Join(dir, name)}
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".exe") {
		candidates = append(candidates, filepath.Join(dir, name+".exe"))
	}
	return candidates
}

func firstGGUF(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".gguf") {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	return filepath.Join(dir, names[0])
}

func hasPathSeparator(value string) bool {
	return value != filepath.Base(value)
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	if _, err := os.Stat(path); err != nil {
		return false
	}
	return true
}

func windowsTrimRootedPath(value string) string {
	if runtime.GOOS != "windows" {
		return ""
	}
	cleaned := filepath.Clean(value)
	if filepath.VolumeName(cleaned) != "" {
		return ""
	}
	if strings.HasPrefix(cleaned, string(filepath.Separator)) {
		return strings.TrimLeft(cleaned, string(filepath.Separator))
	}
	if strings.HasPrefix(cleaned, "/") {
		return strings.TrimLeft(cleaned, "/")
	}
	return ""
}
