//go:build linux || openbsd || netbsd || dragonfly || darwin

package vtui

import (
	"fmt"
	"io"
	"os"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/unxed/vtinput"
)

// RunInGUIWindow detects the available display server (Wayland or X11)
// and launches the TUI within a native graphical window.
// If backend is empty, it attempts X11 first (current default), then Wayland.
func RunInGUIWindow(cols, rows int, backend string, setupApp func()) error {
	if backend == "wayland" {
		return runInWaylandWindow(cols, rows, setupApp)
	}
	if backend == "x11" {
		return runInX11Window(cols, rows, setupApp)
	}

	// Default logic: Try X11 first (stability priority), then Wayland
	if os.Getenv("DISPLAY") != "" {
		DebugLog("GUI: DISPLAY detected, starting X11 Host (default)")
		return runInX11Window(cols, rows, setupApp)
	}

	if os.Getenv("WAYLAND_DISPLAY") != "" {
		DebugLog("GUI: WAYLAND_DISPLAY detected, starting Wayland Host")
		return runInWaylandWindow(cols, rows, setupApp)
	}

	return fmt.Errorf("no GUI display found (neither DISPLAY nor WAYLAND_DISPLAY are set)")
}

func runInX11Window(cols, rows int, setupApp func()) error {
	fontSize := 22.0
	// Temporary host to detect DPI
	tempConn, _ := xgb.NewConn()
	dpi := 96.0
	if tempConn != nil {
		setup := xproto.Setup(tempConn)
		screen := setup.DefaultScreen(tempConn)
		if screen.WidthInMillimeters > 0 {
			dpi = (float64(screen.WidthInPixels) * 25.4) / float64(screen.WidthInMillimeters)
		}
		tempConn.Close()
	}

	face, cellW, cellH := loadBestFont(fontSize, dpi)

	host, err := NewX11Host(cols, rows, cellW, cellH)
	if err != nil {
		return err
	}
	defer host.Close()

	scr := NewScreenBuf()
	scr.AllocBuf(cols, rows)
	scr.Renderer = NewX11Renderer(host, face)

	FrameManager.Init(scr)

	pr, _ := io.Pipe()
	reader := vtinput.NewReader(pr)
	if reader.NativeEventChan == nil {
		reader.NativeEventChan = make(chan *vtinput.InputEvent, 1024)
	}
	host.reader = reader

	// Override global terminal size source
	GetTerminalSize = func() (int, int, error) {
		host.mu.Lock()
		defer host.mu.Unlock()
		return host.cols, host.rows, nil
	}

	go host.RunEventLoop()
	setupApp()
	FrameManager.Run(reader)

	return nil
}
