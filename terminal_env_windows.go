//go:build windows

package vtui

import "golang.org/x/sys/windows"

func initTerminalOS() {
	// Ensure that Windows Console handles UTF-8 output properly,
	// preventing Box Drawing characters from appearing as gibberish.
	windows.SetConsoleOutputCP(windows.CP_UTF8)
}