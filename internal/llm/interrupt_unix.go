//go:build !windows

package llm

import "os"

func interruptSignal() os.Signal {
	return os.Interrupt
}
