//go:build (linux || windows || darwin) && !arm

package vtui

import (
	"io"
	"os"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/unxed/vtinput"
)

// EbitenHost drives a vtui application inside an Ebitengine window.
//
// The point of this backend is that it needs no cgo on Linux, Windows and
// macOS alike, so vtui's GUI mode does not depend on a single graphics stack.
// Ebitengine reaches the platform through purego, which means the whole thing
// still cross-compiles with CGO_ENABLED=0.
type EbitenHost struct {
	mu sync.Mutex

	renderer *EbitenRenderer
	reader   *vtinput.Reader
	scr      *ScreenBuf

	cols, rows   int
	cellW, cellH int

	// Last window size seen by Layout, in logical pixels.
	winW, winH int

	// Mouse state, kept so that a drag reports the button that started it and
	// so that a move with no button down is not sent at all.
	mouseBtn   uint32
	lastMouseX int
	lastMouseY int

	// Buffers reused every frame to keep the input path allocation-free.
	keyBuf  []ebiten.Key
	charBuf []rune

	pendingSize struct {
		w, h  int
		valid bool
	}
}

func (h *EbitenHost) sendEvent(ev *vtinput.InputEvent) {
	if h.reader == nil || h.reader.EventChan == nil {
		return
	}
	select {
	case h.reader.EventChan <- ev:
	default:
		// A full queue means the app is behind. Mouse motion is the only
		// event stream dense enough to cause that and the only one where
		// dropping an intermediate sample costs nothing, so it goes first.
		if ev.Type == vtinput.MouseEventType && (ev.MouseEventFlags&vtinput.MouseMoved) != 0 {
			return
		}
		select {
		case h.reader.EventChan <- ev:
		default:
			DebugLog("EBITEN_HOST: dropped event, queue full: %s", ev.String())
		}
	}
}

// requestSize records a resize asked for by the UI. The actual call happens on
// the game loop, because Ebitengine's window functions want the main thread.
func (h *EbitenHost) requestSize(w, h2 int) {
	h.mu.Lock()
	h.pendingSize.w, h.pendingSize.h, h.pendingSize.valid = w, h2, true
	h.mu.Unlock()
}

// ebitenGame is the ebiten.Game implementation. Update translates input,
// Draw uploads whatever the renderer has rasterised.
type ebitenGame struct {
	host   *EbitenHost
	screen *ebiten.Image
}

func (g *ebitenGame) Update() error {
	h := g.host

	h.mu.Lock()
	if h.pendingSize.valid {
		w, ph := h.pendingSize.w, h.pendingSize.h
		h.pendingSize.valid = false
		h.mu.Unlock()
		ebiten.SetWindowSize(w, ph)
	} else {
		h.mu.Unlock()
	}

	if title, ok := h.renderer.takeTitle(); ok {
		ebiten.SetWindowTitle(title)
	}

	mods := ebitenModifiers()

	// Keys first, then text. A modified or special key is delivered by virtual
	// key code; an unmodified printable key is left to the text stream below,
	// because only the platform knows what character the current layout
	// produces for that physical key. Sending both would double every
	// keystroke.
	h.keyBuf = inpututil.AppendJustPressedKeys(h.keyBuf[:0])
	for _, k := range h.keyBuf {
		vk := ebitenKeyToVK(k)
		if vk == 0 {
			continue
		}
		if !isSpecialOrModifiedKey(vk, mods) {
			continue
		}
		h.sendEvent(&vtinput.InputEvent{
			Type:            vtinput.KeyEventType,
			KeyDown:         true,
			VirtualKeyCode:  vk,
			ControlKeyState: mods,
		})
	}

	h.keyBuf = inpututil.AppendJustReleasedKeys(h.keyBuf[:0])
	for _, k := range h.keyBuf {
		vk := ebitenKeyToVK(k)
		if vk == 0 {
			continue
		}
		h.sendEvent(&vtinput.InputEvent{
			Type:            vtinput.KeyEventType,
			KeyDown:         false,
			VirtualKeyCode:  vk,
			ControlKeyState: mods,
		})
	}

	// Text input. Ctrl and Alt combinations are filtered out: some platforms
	// still emit a control character for them, and the virtual key event above
	// has already covered that keystroke.
	h.charBuf = ebiten.AppendInputChars(h.charBuf[:0])
	if mods&(vtinput.LeftCtrlPressed|vtinput.RightCtrlPressed|vtinput.LeftAltPressed|vtinput.RightAltPressed) == 0 {
		for _, r := range h.charBuf {
			if r < 0x20 || r == 0x7f {
				continue
			}
			h.sendEvent(&vtinput.InputEvent{
				Type:            vtinput.KeyEventType,
				KeyDown:         true,
				Char:            r,
				ControlKeyState: mods,
			})
		}
	}

	g.updateMouse(mods)
	return nil
}

func (g *ebitenGame) updateMouse(mods vtinput.ControlKeyState) {
	h := g.host

	h.mu.Lock()
	cw, ch := h.cellW, h.cellH
	h.mu.Unlock()
	if cw <= 0 || ch <= 0 {
		return
	}

	px, py := ebiten.CursorPosition()
	cx, cy := px/cw, py/ch

	for _, b := range [...]struct {
		btn  ebiten.MouseButton
		mask uint32
	}{
		{ebiten.MouseButtonLeft, uint32(vtinput.FromLeft1stButtonPressed)},
		{ebiten.MouseButtonMiddle, uint32(vtinput.FromLeft2ndButtonPressed)},
		{ebiten.MouseButtonRight, uint32(vtinput.RightmostButtonPressed)},
	} {
		if inpututil.IsMouseButtonJustPressed(b.btn) {
			h.mu.Lock()
			h.mouseBtn |= b.mask
			btn := h.mouseBtn
			h.mu.Unlock()
			h.sendEvent(&vtinput.InputEvent{
				Type:            vtinput.MouseEventType,
				MouseX:          int16(cx),
				MouseY:          int16(cy),
				KeyDown:         true,
				ButtonState:     btn,
				ControlKeyState: mods,
			})
		}
		if inpututil.IsMouseButtonJustReleased(b.btn) {
			h.mu.Lock()
			h.mouseBtn &^= b.mask
			btn := h.mouseBtn
			h.mu.Unlock()
			h.sendEvent(&vtinput.InputEvent{
				Type:            vtinput.MouseEventType,
				MouseX:          int16(cx),
				MouseY:          int16(cy),
				KeyDown:         false,
				ButtonState:     btn,
				ControlKeyState: mods,
			})
		}
	}

	// Motion is reported per cell, not per pixel: the UI works in cells, and
	// a pixel-granular stream would flood the queue during a drag.
	h.mu.Lock()
	moved := cx != h.lastMouseX || cy != h.lastMouseY
	h.lastMouseX, h.lastMouseY = cx, cy
	btn := h.mouseBtn
	h.mu.Unlock()

	if moved && btn != 0 {
		h.sendEvent(&vtinput.InputEvent{
			Type:            vtinput.MouseEventType,
			MouseX:          int16(cx),
			MouseY:          int16(cy),
			MouseEventFlags: vtinput.MouseMoved,
			ButtonState:     btn,
			ControlKeyState: mods,
		})
	}

	if _, dy := ebiten.Wheel(); dy != 0 {
		steps := int(dy)
		if steps == 0 {
			if dy > 0 {
				steps = 1
			} else {
				steps = -1
			}
		}
		if steps < 0 {
			steps = -steps
		}
		steps *= getSystemScrollLines()
		dir := -1
		if dy > 0 {
			dir = 1
		}
		for i := 0; i < steps; i++ {
			h.sendEvent(&vtinput.InputEvent{
				Type:            vtinput.MouseEventType,
				MouseX:          int16(cx),
				MouseY:          int16(cy),
				WheelDirection:  dir,
				ControlKeyState: mods,
			})
		}
	}
}

func (g *ebitenGame) Draw(screen *ebiten.Image) {
	pix, w, h, changed := g.host.renderer.takeFrame()
	if pix == nil || w <= 0 || h <= 0 {
		return
	}

	// The texture is rebuilt only when the rasteriser touched something. On a
	// still screen Draw costs one DrawImage and no upload at all.
	if g.screen == nil || g.screen.Bounds().Dx() != w || g.screen.Bounds().Dy() != h {
		g.screen = ebiten.NewImage(w, h)
		changed = true
	}
	if changed {
		g.screen.WritePixels(pix)
	}
	screen.DrawImage(g.screen, nil)
}

// Layout maps the window size onto the character grid and tells the running
// application when that grid changed.
func (g *ebitenGame) Layout(outsideWidth, outsideHeight int) (int, int) {
	h := g.host

	h.mu.Lock()
	changed := outsideWidth != h.winW || outsideHeight != h.winH
	h.winW, h.winH = outsideWidth, outsideHeight
	if h.cellW > 0 && h.cellH > 0 {
		cols, rows := outsideWidth/h.cellW, outsideHeight/h.cellH
		if cols < 1 {
			cols = 1
		}
		if rows < 1 {
			rows = 1
		}
		if cols != h.cols || rows != h.rows {
			h.cols, h.rows = cols, rows
			changed = true
		} else {
			changed = false
		}
	}
	h.mu.Unlock()

	if changed {
		h.sendEvent(&vtinput.InputEvent{Type: vtinput.ResizeEventType})
	}
	return outsideWidth, outsideHeight
}

// RunEbitenHost opens an Ebitengine window and runs the application in it.
// It blocks until the window closes, and must be called from the main
// goroutine because that is where Ebitengine insists on running its loop.
func RunEbitenHost(cols, rows int, fontName string, fontSize float64, setupApp func()) error {
	face, cellW, cellH := loadBestFont(fontName, fontSize, 72)
	if cellW <= 0 || cellH <= 0 {
		cellW, cellH = 7, 13
	}
	DebugLog("EBITEN_HOST: starting %dx%d cells, cell size %dx%d", cols, rows, cellW, cellH)

	host := &EbitenHost{
		cols:       cols,
		rows:       rows,
		cellW:      cellW,
		cellH:      cellH,
		winW:       cols * cellW,
		winH:       rows * cellH,
		lastMouseX: -1,
		lastMouseY: -1,
	}

	renderer := NewEbitenRenderer(host, face, cellW, cellH)
	host.renderer = renderer

	scr := NewScreenBuf()
	scr.AllocBuf(cols, rows)
	scr.Renderer = renderer
	host.scr = scr
	FrameManager.Init(scr)

	// vtinput normally parses a terminal byte stream. Here the events are
	// synthesised directly, so the reader is handed a pipe that never
	// produces anything and is used purely for its event channel.
	pr, _ := io.Pipe()
	reader := vtinput.NewReader(pr, true)
	host.reader = reader

	GetTerminalSize = func() (int, int, error) {
		host.mu.Lock()
		defer host.mu.Unlock()
		return host.cols, host.rows, nil
	}

	setupApp()

	ebiten.SetWindowTitle(AppName)
	ebiten.SetWindowSize(cols*cellW, rows*cellH)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	// The screen is fully repainted from our own framebuffer every frame, so
	// letting Ebitengine clear it first would only waste a pass.
	ebiten.SetScreenClearedEveryFrame(false)

	game := &ebitenGame{host: host}

	go func() {
		defer LogAndRepanic("ebiten FrameManager")
		FrameManager.Run(reader)
		DebugLog("EBITEN_HOST: FrameManager exited, shutting down")
		os.Exit(0)
	}()

	err := ebiten.RunGame(game)
	DebugLog("EBITEN_HOST: RunGame returned: %v", err)
	return err
}
