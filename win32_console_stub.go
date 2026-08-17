//go:build !windows

package vtui

import "sync"

func isWineOS() bool {
	return false
}

// Win32ConsoleRenderer is a fallback stub for non-Windows platforms.
type Win32ConsoleRenderer struct {
	mu sync.Mutex
}

func NewWin32ConsoleRenderer(parent *ScreenBuf) *Win32ConsoleRenderer {
	return &Win32ConsoleRenderer{}
}

func (r *Win32ConsoleRenderer) SetPalette(pal *[256]uint32)                               {}
func (r *Win32ConsoleRenderer) SetCursor(x, y int, visible bool, shape CursorShape)       {}
func (r *Win32ConsoleRenderer) SetWindowTitle(title string)                               {}
func (r *Win32ConsoleRenderer) Render(buf, shadow []CharInfo, w, h int, forceRedraw bool) {}
func (r *Win32ConsoleRenderer) Flush()                                                    {}
