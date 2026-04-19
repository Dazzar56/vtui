//go:build darwin

package vtui

import (
	"os"
	"os/exec"
)

func RedirectStderr(f *os.File) error {
	// Standard Unix-like redirect via Dup2 works on Darwin
	return os.NewSyscallError("dup2", nil) // Placeholder or implement via syscall
}

func countOpenFDs() int {
	// macOS doesn't have /proc/self/fd, would need lsof or proc_pidinfo
	return -1
}