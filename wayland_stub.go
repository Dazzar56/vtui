//go:build darwin || freebsd || dragonfly || openbsd || netbsd || arm || mips || mipsle || mips64 || mips64le || riscv64 || loong64 || ppc64 || ppc64le

package vtui

import (
	"errors"
)

// runInWaylandWindow is a stub for macOS where Wayland is not supported.
func runInWaylandWindow(cols, rows int, fontName string, fontSize float64, setupApp func()) error {
	return errors.New("Wayland backend is not supported on macOS. Please use X11 or Terminal mode.")
}
