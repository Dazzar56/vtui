//go:build !freebsd && !dragonfly && !openbsd && !netbsd && !illumos && !solaris && !arm

package vtui

import (
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gogpu/gg/text"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
	"github.com/unxed/vtinput"
)

var (
	debugLastMouseX, debugLastMouseY float64 = -1, -1
	debugLastCtxW, debugLastCtxH     int     = -1, -1
)

type GogpuHost struct {
	mu              sync.Mutex
	app             *gogpu.App
	reader          *vtinput.Reader
	scr             *ScreenBuf
	cols, rows      int
	cellW, cellH    int
	face            text.Face
	mouseBtn        uint32
	currentMods     vtinput.ControlKeyState
	pendingKeyEvent *vtinput.InputEvent
	pendingKeyTimer *time.Timer
	lastRuneForVK   map[uint16]rune
	lastVK          uint16
	// suppressTextInput drops the text belonging to the keystroke just
	// handled. A keypad key in navigation mode has already been delivered as
	// a virtual key, and some platforms hand us its digit anyway.
	suppressTextInput bool
	lCtrl, rCtrl      bool
	lAlt, rAlt        bool
	lShift, rShift    bool

	// Cached sizes to prevent deadlocks and speed up GetTerminalSize
	lastAppW, lastAppH int
	resizePending      bool
	// dragOut is the gesture waiting for the main loop to hand it to
	// gogpu, or nil. One pointer, so one gesture at a time.
	dragOut *gogpuDragRequest
}

func (h *GogpuHost) sendEvent(ev *vtinput.InputEvent) {
	if h.reader == nil || h.reader.EventChan == nil {
		return
	}
	select {
	case h.reader.EventChan <- ev:
	default:
		// Drop intermediate mouse move events when queue is full to prevent clogging
		if ev.Type == vtinput.MouseEventType && (ev.MouseEventFlags&vtinput.MouseMoved) != 0 {
			return
		}
		// For non-move critical events, attempt a secondary non-blocking send without spawning goroutines
		select {
		case h.reader.EventChan <- ev:
		default:
			DebugLog("GOGPU_HOST: dropped event due to full buffer: %s", ev.String())
		}
	}
}

func isSpecialOrModifiedKey(vk uint16, mods vtinput.ControlKeyState) bool {
	if (mods & (vtinput.LeftCtrlPressed | vtinput.RightCtrlPressed | vtinput.LeftAltPressed | vtinput.RightAltPressed)) != 0 {
		return true
	}
	switch vk {
	case vtinput.VK_ESCAPE, vtinput.VK_RETURN, vtinput.VK_TAB, vtinput.VK_BACK, vtinput.VK_DELETE, vtinput.VK_INSERT,
		vtinput.VK_UP, vtinput.VK_DOWN, vtinput.VK_LEFT, vtinput.VK_RIGHT,
		vtinput.VK_HOME, vtinput.VK_END, vtinput.VK_PRIOR, vtinput.VK_NEXT,
		// Keypad 5 with NumLock off. It produces no character, so waiting for
		// text that never comes would only delay it by the pairing timeout.
		vtinput.VK_CLEAR,
		vtinput.VK_CONTROL, vtinput.VK_LCONTROL, vtinput.VK_RCONTROL,
		vtinput.VK_SHIFT, vtinput.VK_LSHIFT, vtinput.VK_RSHIFT,
		vtinput.VK_MENU, vtinput.VK_LMENU, vtinput.VK_RMENU,
		vtinput.VK_LWIN, vtinput.VK_RWIN, vtinput.VK_APPS,
		vtinput.VK_CAPITAL, vtinput.VK_NUMLOCK, vtinput.VK_SCROLL:
		return true
	}
	if vk >= vtinput.VK_F1 && vk <= vtinput.VK_F24 {
		return true
	}
	return false
}

func (h *GogpuHost) syncMods(vk uint16, mods gpucontext.Modifiers, isDown bool) vtinput.ControlKeyState {
	if isDown {
		if vk == vtinput.VK_LCONTROL {
			h.lCtrl = true
		}
		if vk == vtinput.VK_RCONTROL {
			h.rCtrl = true
		}
		if vk == vtinput.VK_LMENU {
			h.lAlt = true
		}
		if vk == vtinput.VK_RMENU {
			h.rAlt = true
		}
		if vk == vtinput.VK_LSHIFT {
			h.lShift = true
		}
		if vk == vtinput.VK_RSHIFT {
			h.rShift = true
		}
	} else {
		if vk == vtinput.VK_LCONTROL {
			h.lCtrl = false
		}
		if vk == vtinput.VK_RCONTROL {
			h.rCtrl = false
		}
		if vk == vtinput.VK_LMENU {
			h.lAlt = false
		}
		if vk == vtinput.VK_RMENU {
			h.rAlt = false
		}
		if vk == vtinput.VK_LSHIFT {
			h.lShift = false
		}
		if vk == vtinput.VK_RSHIFT {
			h.rShift = false
		}
	}

	var sysMods vtinput.ControlKeyState
	if mods.HasShift() {
		sysMods |= vtinput.ShiftPressed
	}
	if mods.HasControl() {
		if h.rCtrl {
			sysMods |= vtinput.RightCtrlPressed
		} else {
			sysMods |= vtinput.LeftCtrlPressed
		}
	}
	if mods.HasAlt() {
		if h.rAlt {
			sysMods |= vtinput.RightAltPressed
		} else {
			sysMods |= vtinput.LeftAltPressed
		}
	}

	// Lock states. gpucontext has no Has* helper for these, but the platform
	// layer fills both bits on every event. Without them isNumLockEffectiveGogpu
	// always sees NumLock as off, and the keypad can never reach numeric mode.
	// X11 and Ebitengine already report the locks; the framework masks them out
	// where they must not affect matching (see FrameManager and Edit).
	if mods&gpucontext.ModCapsLock != 0 {
		sysMods |= vtinput.CapsLockOn
	}
	if mods&gpucontext.ModNumLock != 0 {
		sysMods |= vtinput.NumLockOn
	}

	h.currentMods = sysMods
	return sysMods
}

func RunGogpuHost(cols, rows int, fontName string, fontSize float64, setupApp func()) error {
	// DX12: use naga DXIL backend instead of HLSL->FXC
	// to avoid 2-6s shader compilation via d3dcompiler_47.dll
	if os.Getenv("GOGPU_DX12_DXIL") == "" {
		api := os.Getenv("GOGPU_GRAPHICS_API")
		if api == "" || strings.EqualFold(api, "dx12") || strings.EqualFold(api, "d3d12") || strings.EqualFold(api, "directx") {
			os.Setenv("GOGPU_DX12_DXIL", "1")
		}
	}
	face, cellW, cellH := loadGogpuFont(fontName, fontSize)

	fmt.Fprintf(os.Stdout, "GOGPU_HOST: Starting RunGogpuHost %dx%d (Cell: %dx%d)\n", cols, rows, cellW, cellH)
	DebugLog("GOGPU_HOST: Starting RunGogpuHost %dx%d (Cell: %dx%d)", cols, rows, cellW, cellH)

	config := gogpu.DefaultConfig().
		WithTitle(AppName).
		WithSize(cols*cellW, rows*cellH)

	fmt.Fprintln(os.Stdout, "GOGPU_HOST: Creating gogpu.App...")
	app := gogpu.NewApp(config)
	fmt.Fprintln(os.Stdout, "GOGPU_HOST: gogpu.App created successfully")

	host := &GogpuHost{
		app:      app,
		cols:     cols,
		rows:     rows,
		cellW:    cellW,
		cellH:    cellH,
		face:     face,
		lastAppW: cols * cellW,
		lastAppH: rows * cellH,
	}

	scr := NewScreenBuf()
	host.scr = scr
	scr.AllocBuf(cols, rows)
	renderer := NewGogpuRenderer(host, face, cellW, cellH)
	scr.Renderer = renderer
	scr.Graphics().SetProtocol(GraphicsNative)
	scr.Graphics().SetCellSize(cellW, cellH)

	FrameManager.Init(scr)

	pr, _ := io.Pipe()
	reader := vtinput.NewReader(pr, true)
	host.reader = reader

	// Not a close request. gogpu's App.OnClose runs inside App.shutdown(),
	// on the render thread, after the main loop has already ended and just
	// before the renderer is destroyed (gogpu app.go:444). Emitting CmQuit
	// from here made the application open its exit confirmation dialog
	// during teardown — including teardown caused by a panic, which made a
	// crash on the draw thread look like a user initiated quit in the logs.
	//
	// Nothing has to be emitted: closing the window clears the last window,
	// gogpu stops the main loop (quitOnLastWindowClosed defaults to true),
	// App.Run returns, and RunGogpuHost returns to its caller, which is the
	// ordinary exit path with its deferred cleanup intact.
	app.OnClose(func() {
		DebugLog("GOGPU_HOST: app is shutting down, renderer teardown follows")
	})
	// Files dropped on the window by other applications arrive here; the
	// drag and drop core takes them from the backend to whatever the
	// application registered as its target.
	app.OnDragDrop(host.handleFileDrop)
	SetDragBackend(host)
	logGogpuDragEnvironment()
	// A drag out has to begin on this loop: on Windows and X11 gogpu's
	// drag source is a modal loop of its own, and everywhere the window
	// belongs to this thread.
	app.OnUpdate(func(float64) {
		defer LogAndRepanic("gogpu OnUpdate")
		host.pumpDragOut()
	})

	app.EventSource().OnKeyPress(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		vkBase := gogpuKeyToVK(key, 0)

		host.mu.Lock()
		currMods := host.syncMods(vkBase, mods, true)

		vk := gogpuKeyToVK(key, currMods)
		if vk != 0 {
			DebugLog("GOGPU_HOST_EVENT: OnKeyPress key=%v, vk=%d", key, vk)
		} else {
			DebugLog("GOGPU_HOST_EVENT: OnKeyPress UNMAPPED key=%v", key)
		}

		// A keypad key that resolved to navigation must not also type its
		// digit. Whether the platform still emits text for it depends on how
		// faithfully its keymap tracks NumLock, so rather than trust that,
		// drop the text this keystroke would carry. Every other key clears
		// the flag again, and text only ever follows its own key press.
		host.suppressTextInput = isGogpuKeypadKey(key) && gogpuNumpadRune(vk) == 0

		if host.pendingKeyEvent != nil {
			if host.pendingKeyTimer != nil {
				host.pendingKeyTimer.Stop()
			}
			if host.pendingKeyEvent.Char == 0 && host.lastRuneForVK != nil {
				host.pendingKeyEvent.Char = host.lastRuneForVK[host.pendingKeyEvent.VirtualKeyCode]
			}
			host.sendEvent(host.pendingKeyEvent)
			host.pendingKeyEvent = nil
		}

		if vk != 0 {
			host.lastVK = vk
			ev := &vtinput.InputEvent{
				Type:            vtinput.KeyEventType,
				KeyDown:         true,
				VirtualKeyCode:  vk,
				ControlKeyState: currMods,
			}

			if isSpecialOrModifiedKey(vk, currMods) {
				// Buggy
				/*
					// A modified key carries its character too. The X11 and
					// Wayland backends put the virtual key and the character in
					// one event unconditionally, and accelerators such as
					// Alt+letter need the character to know what to search for.
					// This branch used to send Char zero while the three other
					// send sites in this function all filled it in, so the same
					// keystroke meant different things depending on the backend.
					if ev.Char == 0 && host.lastRuneForVK != nil {
						ev.Char = host.lastRuneForVK[vk]
					}
					if ev.Char == 0 {
						ev.Char = defaultRuneForVK(vk)
					}
				*/
				host.sendEvent(ev)
			} else {
				if host.lastRuneForVK != nil {
					ev.Char = host.lastRuneForVK[vk]
				}
				// The keypad names its own character, and unlike the letter
				// rows it does not depend on the layout. Seeding it here keeps
				// the digit working where the platform sends no text for the
				// keypad at all; a character that does arrive still wins,
				// which matters for layouts whose decimal key yields a comma.
				if ev.Char == 0 {
					ev.Char = gogpuNumpadRune(vk)
				}
				host.pendingKeyEvent = ev
				host.pendingKeyTimer = time.AfterFunc(10*time.Millisecond, func() {
					host.mu.Lock()
					defer host.mu.Unlock()
					if host.pendingKeyEvent != nil {
						if host.pendingKeyEvent.Char == 0 && host.lastRuneForVK != nil {
							host.pendingKeyEvent.Char = host.lastRuneForVK[host.pendingKeyEvent.VirtualKeyCode]
						}
						host.sendEvent(host.pendingKeyEvent)
						host.pendingKeyEvent = nil
					}
				})
			}
		}
		host.mu.Unlock()
	})

	app.EventSource().OnTextInput(func(text string) {
		DebugLog("GOGPU_HOST_EVENT: OnTextInput text=%q", text)
		host.mu.Lock()
		defer host.mu.Unlock()

		runes := []rune(text)
		if len(runes) == 0 {
			return
		}

		// This text belongs to a keypad key already delivered as navigation.
		// The key press flushed any pending event before setting the flag, so
		// there is nothing here to pair the text with either.
		if host.suppressTextInput {
			host.suppressTextInput = false
			DebugLog("GOGPU_HOST_EVENT: dropped keypad text %q, key was navigation", text)
			return
		}

		if host.lastRuneForVK == nil {
			host.lastRuneForVK = make(map[uint16]rune)
		}

		if host.pendingKeyEvent != nil {
			if host.pendingKeyTimer != nil {
				host.pendingKeyTimer.Stop()
			}
			host.pendingKeyEvent.Char = runes[0]
			host.lastRuneForVK[host.pendingKeyEvent.VirtualKeyCode] = runes[0]
			host.sendEvent(host.pendingKeyEvent)
			host.pendingKeyEvent = nil

			for i := 1; i < len(runes); i++ {
				host.sendEvent(&vtinput.InputEvent{
					Type:            vtinput.KeyEventType,
					KeyDown:         true,
					Char:            runes[i],
					ControlKeyState: host.currentMods,
				})
			}
		} else {
			if host.lastVK != 0 {
				host.lastRuneForVK[host.lastVK] = runes[0]
			}
			for _, r := range runes {
				host.sendEvent(&vtinput.InputEvent{
					Type:            vtinput.KeyEventType,
					KeyDown:         true,
					Char:            r,
					ControlKeyState: host.currentMods,
				})
			}
		}
	})

	app.EventSource().OnKeyRelease(func(key gpucontext.Key, mods gpucontext.Modifiers) {
		vkBase := gogpuKeyToVK(key, 0)

		host.mu.Lock()
		currMods := host.syncMods(vkBase, mods, false)

		vk := gogpuKeyToVK(key, currMods)
		if vk == 0 {
			DebugLog("GOGPU_HOST_EVENT: OnKeyRelease UNMAPPED key=%v", key)
		}

		if host.pendingKeyEvent != nil {
			if host.pendingKeyTimer != nil {
				host.pendingKeyTimer.Stop()
			}
			if host.pendingKeyEvent.Char == 0 && host.lastRuneForVK != nil {
				host.pendingKeyEvent.Char = host.lastRuneForVK[host.pendingKeyEvent.VirtualKeyCode]
			}
			host.sendEvent(host.pendingKeyEvent)
			host.pendingKeyEvent = nil
		}

		host.mu.Unlock()

		if vk == 0 {
			return
		}
		host.sendEvent(&vtinput.InputEvent{
			Type:            vtinput.KeyEventType,
			KeyDown:         false,
			VirtualKeyCode:  vk,
			ControlKeyState: currMods,
		})
	})

	app.EventSource().OnMousePress(func(button gpucontext.MouseButton, x, y float64) {
		var btn uint32
		switch button {
		case gpucontext.MouseButtonLeft:
			btn = uint32(vtinput.FromLeft1stButtonPressed)
		case gpucontext.MouseButtonRight:
			btn = uint32(vtinput.RightmostButtonPressed)
		case gpucontext.MouseButtonMiddle:
			btn = uint32(vtinput.FromLeft2ndButtonPressed)
		default:
			btn = uint32(vtinput.FromLeft1stButtonPressed)
		}

		host.mu.Lock()
		host.mouseBtn = btn
		cW := host.cellW
		cH := host.cellH
		host.mu.Unlock()

		host.sendEvent(&vtinput.InputEvent{
			Type:        vtinput.MouseEventType,
			MouseX:      int16(x / float64(cW)),
			MouseY:      int16(y / float64(cH)),
			KeyDown:     true,
			ButtonState: btn,
		})
	})

	app.EventSource().OnMouseRelease(func(button gpucontext.MouseButton, x, y float64) {
		host.mu.Lock()
		host.mouseBtn = 0
		cW := host.cellW
		cH := host.cellH
		host.mu.Unlock()

		host.sendEvent(&vtinput.InputEvent{
			Type:        vtinput.MouseEventType,
			MouseX:      int16(x / float64(cW)),
			MouseY:      int16(y / float64(cH)),
			KeyDown:     false,
			ButtonState: 0,
		})
	})

	app.EventSource().OnMouseMove(func(x, y float64) {
		host.mu.Lock()
		btn := host.mouseBtn
		cW := host.cellW
		cH := host.cellH
		host.mu.Unlock()

		if btn != 0 {
			host.sendEvent(&vtinput.InputEvent{
				Type:            vtinput.MouseEventType,
				MouseX:          int16(x / float64(cW)),
				MouseY:          int16(y / float64(cH)),
				MouseEventFlags: vtinput.MouseMoved,
				ButtonState:     btn,
				ControlKeyState: host.currentMods,
			})
		}
	})

	app.EventSource().OnScroll(func(dx float64, dy float64) {
		host.mu.Lock()
		cW := host.cellW
		cH := host.cellH
		host.mu.Unlock()

		mx, my := app.Input().Mouse().Position()

		// Multiply scroll lines to match preferred user experience
		steps := int(math.Abs(dy) * float64(getSystemScrollLines()))
		if steps == 0 {
			return
		}
		dir := -1
		if dy < 0 {
			dir = 1
		}
		for i := 0; i < steps; i++ {
			host.sendEvent(&vtinput.InputEvent{
				Type:           vtinput.MouseEventType,
				MouseX:         int16(float64(mx) / float64(cW)),
				MouseY:         int16(float64(my) / float64(cH)),
				WheelDirection: dir,
			})
		}
		// Request a redraw to ensure the UI updates instantly in event-driven mode
		app.RequestRedraw()
	})

	var infoLogged sync.Once
	app.OnDraw(func(dc *gogpu.Context) {
		defer LogAndRepanic("gogpu OnDraw")

		w, h := dc.Width(), dc.Height()

		host.mu.Lock()
		sizeChanged := (host.lastAppW != w || host.lastAppH != h)
		host.lastAppW, host.lastAppH = w, h
		if sizeChanged {
			host.resizePending = true
		}
		host.mu.Unlock()

		if sizeChanged && host.reader != nil && host.reader.EventChan != nil {
			host.sendEvent(&vtinput.InputEvent{Type: vtinput.ResizeEventType})
		}

		infoLogged.Do(func() {
			if provider := app.GPUContextProvider(); provider != nil {
				info := provider.AdapterInfo()
				fmt.Fprintf(os.Stdout, "GOGPU_HOST_ON_DRAW: Adapter confirmed: %q, Type: %v\n", info.Name, info.Type)
				DebugLog("GOGPU_HOST_ON_DRAW: Adapter confirmed: %q, Type: %v", info.Name, info.Type)
			}
		})

		if gogpuRenderer, ok := host.scr.Renderer.(*GogpuRenderer); ok {
			gogpuRenderer.DrawToScreen(dc)
		}
	})

	GetTerminalSize = func() (int, int, error) {
		host.mu.Lock()
		defer host.mu.Unlock()

		w, h := host.lastAppW, host.lastAppH

		if host.cellW > 0 && host.cellH > 0 && w > 0 && h > 0 {
			c := w / host.cellW
			r := h / host.cellH
			if c != host.cols || r != host.rows {
				host.cols = c
				host.rows = r
			}
		}
		return host.cols, host.rows, nil
	}

	setupApp()
	// After setupApp: the application installs the debug log sink during
	// setup, so a backend announced before it is logged nowhere.
	SetActiveBackend("gogpu")

	go func() {
		w, h := app.Size()
		fw, fh := app.PhysicalSize()
		fmt.Fprintf(os.Stdout, "GOGPU_HOST: Before Run(). App Size (Log): %dx%d. App PhysicalSize: %dx%d. ScaleFactor: %f\n", w, h, fw, fh, app.ScaleFactor())
		DebugLog("GOGPU_HOST: Before Run(). App Size (Log): %dx%d. App PhysicalSize: %dx%d. ScaleFactor: %f", w, h, fw, fh, app.ScaleFactor())

		provider := app.GPUContextProvider()
		if provider != nil {
			info := provider.AdapterInfo()
			fmt.Fprintf(os.Stdout, "GOGPU_HOST: Adapter: Name=%q, Type=%v\n", info.Name, info.Type)
			DebugLog("GOGPU_HOST: Adapter: Name=%q, Type=%v", info.Name, info.Type)
		}

		fmt.Fprintln(os.Stdout, "GOGPU_HOST: FrameManager starting...")
		DebugLog("GOGPU_HOST: FrameManager starting...")
		FrameManager.Run(reader)
		fmt.Fprintln(os.Stdout, "GOGPU_HOST: FrameManager exited. Forcing app shutdown to prevent blue screen hang.")
		DebugLog("GOGPU_HOST: FrameManager exited. Forcing app shutdown to prevent blue screen hang.")
		os.Exit(0)
	}()

	fmt.Fprintln(os.Stdout, "GOGPU_HOST: Calling app.Run()...")
	err := app.Run()
	fmt.Fprintf(os.Stdout, "GOGPU_HOST: app.Run() exited with: %v\n", err)
	return err
}

// isGogpuFaceSafe does a minimal feature probe on a gg/text.Face.
// The GPU GlyphMask path later calls into FontSource.Parsed/copyCheck.
// A face that survives Metrics() can still have a nil internal
// FontSource and panic on the first DrawString. We catch that class
// of failure early and fall back to the primary face.
func isGogpuFaceSafe(f text.Face) (ok bool) {
	if f == nil {
		return false
	}
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	_ = f.Metrics()
	// Touch a couple of common code points that the UI actually draws.
	_ = f.Advance("A")
	_ = f.Advance("字")
	ok = true
	return
}
func loadGogpuFont(fontName string, size float64) (text.Face, int, int) {
	if size <= 0 {
		size = 18.0
	}
	var primaryFace text.Face
	var cellW, cellH int

	for _, p := range getFontCandidates(fontName) {
		if _, err := os.Stat(p); err == nil {
			src, err := text.NewFontSourceFromFile(p)
			if err == nil {
				face := src.Face(size)
				metrics := face.Metrics()
				adv := face.Advance("A")
				cellH = int(metrics.Ascent + metrics.Descent + 0.5)
				cellW = int(adv + 0.5)
				if cellW == 0 {
					cellW = 8
				}
				if cellH == 0 {
					cellH = 16
				}
				fmt.Fprintf(os.Stdout, "GOGPU_DIAG_FONT: Loaded File=%s RequestSize=%.1f, Cell: %dx%d\n", p, size, cellW, cellH)
				DebugLog("GOGPU_DIAG_FONT: File=%s RequestSize=%.1f", p, size)
				DebugLog("GOGPU_DIAG_FONT: Metrics: Ascent=%.2f Descent=%.2f LineGap=%.2f AdvanceA=%.2f",
					float64(metrics.Ascent), float64(metrics.Descent), float64(metrics.LineGap), adv)
				DebugLog("GOGPU_DIAG_FONT: Calculated Cell: %dx%d", cellW, cellH)
				primaryFace = face
				break
			}
		}
	}

	if primaryFace == nil {
		return nil, 8, 16
	}

	// The fallbacks cannot be attached to the face yet — see the note below on
	// MultiFace — but which of them exist on this machine is worth recording.
	// "No CJK font installed" and "CJK font installed and never consulted" are
	// different bugs with identical symptoms, and the log is the only place
	// they can be told apart. Parsing is part of the probe on purpose: a .ttc
	// collection that gg refuses to open explains a missing glyph just as well
	// as a missing file does.
	for _, p := range fallbackFontPaths {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		src, err := text.NewFontSourceFromFile(p)
		if err != nil {
			DebugLog("GOGPU_DIAG_FONT: fallback present but gg cannot open it: %s: %v", p, err)
			continue
		}
		probe := src.Face(size)
		DebugLog("GOGPU_DIAG_FONT: fallback ok: %s (has 字=%v, has emoji=%v)",
			p, probe.HasGlyph('字'), probe.HasGlyph('😀'))
	}

	// MultiFace gives CJK/emoji coverage, but in gg@v0.50.11 it can
	// hand us a Face whose internal FontSource is nil. The GPU path
	// then panics in copyCheck/Parsed on the first DrawString.
	// Feature-probe the result; on any problem silently keep the
	// already-working primary face (X11/Wayland fallback is separate
	// and already correct).
	// MultiFace is intentionally disabled for the gogpu backend.
	// In gg@v0.50.11 (and the GPU GlyphMask path) a MultiFace can
	// expose a nil *FontSource. The first real DrawString then
	// panics inside copyCheck/Parsed on the render thread.
	// Feature probes that only call Metrics/Advance are not enough
	// to catch it. X11 and Wayland use a completely different font
	// stack and their fallback continues to work.
	// TODO: re-enable when gg fixes MultiFace for the GPU text engine
	//       or when we can construct a verified-safe MultiFace.
	return primaryFace, cellW, cellH
}

func isNumLockEffectiveGogpu(mods vtinput.ControlKeyState) bool {
	numLock := (mods & vtinput.NumLockOn) != 0
	shift := (mods & vtinput.ShiftPressed) != 0
	return numLock != shift
}

// isGogpuKeypadKey reports whether a key belongs to the numeric keypad.
//
// Only the keys whose meaning NumLock changes are listed. The operators and
// Enter are the same key either way, so text arriving for them is genuine and
// must not be dropped.
func isGogpuKeypadKey(k gpucontext.Key) bool {
	switch k {
	case gpucontext.KeyNumpad0, gpucontext.KeyNumpad1, gpucontext.KeyNumpad2,
		gpucontext.KeyNumpad3, gpucontext.KeyNumpad4, gpucontext.KeyNumpad5,
		gpucontext.KeyNumpad6, gpucontext.KeyNumpad7, gpucontext.KeyNumpad8,
		gpucontext.KeyNumpad9, gpucontext.KeyNumpadDecimal:
		return true
	}
	return false
}

// gogpuNumpadRune returns the character a keypad virtual key stands for, or
// zero for the navigation codes the same keys produce with NumLock off.
//
// The caller uses zero as the test for "this keystroke types nothing", so the
// two jobs are one function: it both fills in the character and names the keys
// whose text has to be thrown away.
func gogpuNumpadRune(vk uint16) rune {
	if vk >= vtinput.VK_NUMPAD0 && vk <= vtinput.VK_NUMPAD9 {
		return rune('0' + (vk - vtinput.VK_NUMPAD0))
	}
	switch vk {
	case vtinput.VK_DECIMAL:
		return '.'
	case vtinput.VK_ADD:
		return '+'
	case vtinput.VK_SUBTRACT:
		return '-'
	case vtinput.VK_MULTIPLY:
		return '*'
	case vtinput.VK_DIVIDE:
		return '/'
	case vtinput.VK_SEPARATOR:
		return ','
	}
	return 0
}

func gogpuKeyToVK(k gpucontext.Key, mods vtinput.ControlKeyState) uint16 {
	switch k {
	case gpucontext.KeyEscape:
		return vtinput.VK_ESCAPE
	case gpucontext.KeyF1:
		return vtinput.VK_F1
	case gpucontext.KeyF2:
		return vtinput.VK_F2
	case gpucontext.KeyF3:
		return vtinput.VK_F3
	case gpucontext.KeyF4:
		return vtinput.VK_F4
	case gpucontext.KeyF5:
		return vtinput.VK_F5
	case gpucontext.KeyF6:
		return vtinput.VK_F6
	case gpucontext.KeyF7:
		return vtinput.VK_F7
	case gpucontext.KeyF8:
		return vtinput.VK_F8
	case gpucontext.KeyF9:
		return vtinput.VK_F9
	case gpucontext.KeyF10:
		return vtinput.VK_F10
	case gpucontext.KeyF11:
		return vtinput.VK_F11
	case gpucontext.KeyF12:
		return vtinput.VK_F12
	case gpucontext.KeyInsert:
		return vtinput.VK_INSERT
	case gpucontext.KeyDelete:
		return vtinput.VK_DELETE
	case gpucontext.KeyHome:
		return vtinput.VK_HOME
	case gpucontext.KeyEnd:
		return vtinput.VK_END
	case gpucontext.KeyPageUp:
		return vtinput.VK_PRIOR
	case gpucontext.KeyPageDown:
		return vtinput.VK_NEXT
	case gpucontext.KeyUp:
		return vtinput.VK_UP
	case gpucontext.KeyDown:
		return vtinput.VK_DOWN
	case gpucontext.KeyLeft:
		return vtinput.VK_LEFT
	case gpucontext.KeyRight:
		return vtinput.VK_RIGHT
	case gpucontext.KeyBackspace:
		return vtinput.VK_BACK
	case gpucontext.KeyEnter:
		return vtinput.VK_RETURN
	case gpucontext.KeyTab:
		return vtinput.VK_TAB
	case gpucontext.KeySpace:
		return vtinput.VK_SPACE

	// Numeric keypad.
	//
	// gogpu reports the physical key, so the same Key arrives whatever NumLock
	// says; the split into digits and navigation has to happen here. Shift
	// inverts the lock, which is what isNumLockEffectiveGogpu computes. On
	// Windows the platform layer has already resolved the lock and simply never
	// sends KeyNumpadN in navigation mode, so the same rule holds there too.
	case gpucontext.KeyNumpad0:
		if isNumLockEffectiveGogpu(mods) {
			return vtinput.VK_NUMPAD0
		}
		return vtinput.VK_INSERT
	case gpucontext.KeyNumpad1:
		if isNumLockEffectiveGogpu(mods) {
			return vtinput.VK_NUMPAD1
		}
		return vtinput.VK_END
	case gpucontext.KeyNumpad2:
		if isNumLockEffectiveGogpu(mods) {
			return vtinput.VK_NUMPAD2
		}
		return vtinput.VK_DOWN
	case gpucontext.KeyNumpad3:
		if isNumLockEffectiveGogpu(mods) {
			return vtinput.VK_NUMPAD3
		}
		return vtinput.VK_NEXT
	case gpucontext.KeyNumpad4:
		if isNumLockEffectiveGogpu(mods) {
			return vtinput.VK_NUMPAD4
		}
		return vtinput.VK_LEFT
	case gpucontext.KeyNumpad5:
		if isNumLockEffectiveGogpu(mods) {
			return vtinput.VK_NUMPAD5
		}
		return vtinput.VK_CLEAR
	case gpucontext.KeyNumpad6:
		if isNumLockEffectiveGogpu(mods) {
			return vtinput.VK_NUMPAD6
		}
		return vtinput.VK_RIGHT
	case gpucontext.KeyNumpad7:
		if isNumLockEffectiveGogpu(mods) {
			return vtinput.VK_NUMPAD7
		}
		return vtinput.VK_HOME
	case gpucontext.KeyNumpad8:
		if isNumLockEffectiveGogpu(mods) {
			return vtinput.VK_NUMPAD8
		}
		return vtinput.VK_UP
	case gpucontext.KeyNumpad9:
		if isNumLockEffectiveGogpu(mods) {
			return vtinput.VK_NUMPAD9
		}
		return vtinput.VK_PRIOR
	case gpucontext.KeyNumpadDecimal:
		if isNumLockEffectiveGogpu(mods) {
			return vtinput.VK_DECIMAL
		}
		return vtinput.VK_DELETE
	case gpucontext.KeyNumpadAdd:
		return vtinput.VK_ADD
	case gpucontext.KeyNumpadSubtract:
		return vtinput.VK_SUBTRACT
	case gpucontext.KeyNumpadMultiply:
		return vtinput.VK_MULTIPLY
	case gpucontext.KeyNumpadDivide:
		return vtinput.VK_DIVIDE
	case gpucontext.KeyNumpadComma:
		return vtinput.VK_SEPARATOR
	case gpucontext.KeyNumpadEqual:
		return vtinput.VK_OEM_PLUS
	case gpucontext.KeyNumpadEnter:
		return vtinput.VK_RETURN

	// Lock keys. The keypad is unusable without NumLock reaching the
	// application, and the other two travel with it on every other backend.
	case gpucontext.KeyNumLock:
		return vtinput.VK_NUMLOCK
	case gpucontext.KeyCapsLock:
		return vtinput.VK_CAPITAL
	case gpucontext.KeyScrollLock:
		return vtinput.VK_SCROLL

	case gpucontext.KeyLeftControl:
		return vtinput.VK_LCONTROL
	case gpucontext.KeyRightControl:
		return vtinput.VK_RCONTROL
	case gpucontext.KeyLeftShift:
		return vtinput.VK_LSHIFT
	case gpucontext.KeyRightShift:
		return vtinput.VK_RSHIFT
	case gpucontext.KeyLeftAlt:
		return vtinput.VK_LMENU
	case gpucontext.KeyRightAlt:
		return vtinput.VK_RMENU
	case gpucontext.KeyA:
		return vtinput.VK_A
	case gpucontext.KeyB:
		return vtinput.VK_B
	case gpucontext.KeyC:
		return vtinput.VK_C
	case gpucontext.KeyD:
		return vtinput.VK_D
	case gpucontext.KeyE:
		return vtinput.VK_E
	case gpucontext.KeyF:
		return vtinput.VK_F
	case gpucontext.KeyG:
		return vtinput.VK_G
	case gpucontext.KeyH:
		return vtinput.VK_H
	case gpucontext.KeyI:
		return vtinput.VK_I
	case gpucontext.KeyJ:
		return vtinput.VK_J
	case gpucontext.KeyK:
		return vtinput.VK_K
	case gpucontext.KeyL:
		return vtinput.VK_L
	case gpucontext.KeyM:
		return vtinput.VK_M
	case gpucontext.KeyN:
		return vtinput.VK_N
	case gpucontext.KeyO:
		return vtinput.VK_O
	case gpucontext.KeyP:
		return vtinput.VK_P
	case gpucontext.KeyQ:
		return vtinput.VK_Q
	case gpucontext.KeyR:
		return vtinput.VK_R
	case gpucontext.KeyS:
		return vtinput.VK_S
	case gpucontext.KeyT:
		return vtinput.VK_T
	case gpucontext.KeyU:
		return vtinput.VK_U
	case gpucontext.KeyV:
		return vtinput.VK_V
	case gpucontext.KeyW:
		return vtinput.VK_W
	case gpucontext.KeyX:
		return vtinput.VK_X
	case gpucontext.KeyY:
		return vtinput.VK_Y
	case gpucontext.KeyZ:
		return vtinput.VK_Z
	case gpucontext.Key0:
		return vtinput.VK_0
	case gpucontext.Key1:
		return vtinput.VK_1
	case gpucontext.Key2:
		return vtinput.VK_2
	case gpucontext.Key3:
		return vtinput.VK_3
	case gpucontext.Key4:
		return vtinput.VK_4
	case gpucontext.Key5:
		return vtinput.VK_5
	case gpucontext.Key6:
		return vtinput.VK_6
	case gpucontext.Key7:
		return vtinput.VK_7
	case gpucontext.Key8:
		return vtinput.VK_8
	case gpucontext.Key9:
		return vtinput.VK_9
	case gpucontext.KeyMinus:
		return vtinput.VK_OEM_MINUS
	case gpucontext.KeyEqual:
		return vtinput.VK_OEM_PLUS
	case gpucontext.KeyLeftBracket:
		return vtinput.VK_OEM_4
	case gpucontext.KeyRightBracket:
		return vtinput.VK_OEM_6
	case gpucontext.KeyBackslash:
		return vtinput.VK_OEM_5
	case gpucontext.KeySemicolon:
		return vtinput.VK_OEM_1
	case gpucontext.KeyApostrophe:
		return vtinput.VK_OEM_7
	case gpucontext.KeyComma:
		return vtinput.VK_OEM_COMMA
	case gpucontext.KeyPeriod:
		return vtinput.VK_OEM_PERIOD
	case gpucontext.KeySlash:
		return vtinput.VK_OEM_2
	}
	return 0
}
