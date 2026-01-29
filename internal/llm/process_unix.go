//go:build !windows

package llm

import "os/exec"

func configureCommand(_ *exec.Cmd) {}
