//go:build linux || freebsd || openbsd || netbsd || dragonfly || darwin

package vtui

import (
	"fmt"
	"image"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/shm"
	"github.com/BurntSushi/xgb/xproto"
	"github.com/unxed/vtinput"
)

var (
	shmId   int
	shmAddr uintptr
	shmData []byte
)

func init() {
	// Constants for Linux IPC
	const (
		ipcPrivate = 0
		ipcCreat   = 01000
		ipcRmid    = 0
	)

	// Allocate a 32MB segment (enough for 4K display)
	size := 3840 * 2160 * 4
	
	r1, _, err := syscall.Syscall(syscall.SYS_SHMGET, uintptr(ipcPrivate), uintptr(size), uintptr(ipcCreat|0600))
	if err != 0 {
		DebugLog("X11: shmget failed: %v", err)
		return
	}
	id := int(r1)

	r1, _, err = syscall.Syscall(syscall.SYS_SHMAT, uintptr(id), 0, 0)
	if err != 0 {
		syscall.Syscall(syscall.SYS_SHMCTL, uintptr(id), uintptr(ipcRmid), 0)
		DebugLog("X11: shmat failed: %v", err)
		return
	}
	addr := r1

	shmId = id
	shmAddr = addr
	shmData = unsafe.Slice((*byte)(unsafe.Pointer(shmAddr)), size)
	DebugLog("X11: Allocated shared memory segment (ID: %d)", shmId)
}

// X11Host encapsulates the connection to the X server and window management.
type X11Host struct {
	mu           sync.Mutex
	conn         *xgb.Conn
	wid          xproto.Window
	screen       *xproto.ScreenInfo
	gc           xproto.Gcontext
	pixmap       xproto.Pixmap
	shmSeg       shm.Seg // Shared memory segment
	width        uint16
	height       uint16
	cellW        int
	cellH        int
	scale        int // Scaling factor (1 for standard, 2 for HiDPI, etc.)
	imgBuf       *image.RGBA
	bgraBuf      []byte
	reader       *vtinput.Reader
	cols, rows   int
	closeChan    chan struct{}
	keyMap       []xproto.Keysym
	keysPerCode  byte
	minKeyCode   xproto.Keycode
	atomDelete   xproto.Atom
	dirtyLines   []bool
	lCtrl, rCtrl bool
	lAlt, rAlt   bool
	lShift, rShift bool
	isAltGrPressed bool
}

func NewX11Host(cols, rows, cellW, cellH int) (*X11Host, error) {
	conn, err := xgb.NewConn()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to X11: %v", err)
	}

	setup := xproto.Setup(conn)
	screen := setup.DefaultScreen(conn)

	dpi := 96.0
	if screen.WidthInMillimeters > 0 {
		dpi = (float64(screen.WidthInPixels) * 25.4) / float64(screen.WidthInMillimeters)
	}

	scale := 1
	if dpi > 120 {
		scale = 2
	}

	host := &X11Host{
		conn:       conn,
		screen:     screen,
		cellW:      cellW,
		cellH:      cellH,
		scale:      scale,
		cols:       cols,
		rows:       rows,
		width:      uint16(cols * cellW),
		height:     uint16(rows * cellH),
		closeChan:  make(chan struct{}),
		dirtyLines: make([]bool, rows*cellH),
	}

	host.wid, err = xproto.NewWindowId(conn)
	if err != nil {
		return nil, err
	}

	xproto.CreateWindow(conn, screen.RootDepth, host.wid, screen.Root,
		0, 0, host.width, host.height, 0,
		xproto.WindowClassInputOutput, screen.RootVisual,
		xproto.CwBackPixel|xproto.CwEventMask,
		[]uint32{
			screen.BlackPixel,
			xproto.EventMaskExposure | xproto.EventMaskKeyPress | xproto.EventMaskKeyRelease |
				xproto.EventMaskButtonPress | xproto.EventMaskButtonRelease | xproto.EventMaskPointerMotion |
				xproto.EventMaskStructureNotify,
		})

	title := AppName + " (X11)"
	xproto.ChangeProperty(conn, xproto.PropModeReplace, host.wid, xproto.AtomWmName,
		xproto.AtomString, 8, uint32(len(title)), []byte(title))

	host.gc, err = xproto.NewGcontextId(conn)
	if err == nil {
		xproto.CreateGC(conn, host.gc, xproto.Drawable(host.wid),
			xproto.GcForeground|xproto.GcBackground,
			[]uint32{screen.BlackPixel, screen.WhitePixel})
	}

	host.imgBuf = image.NewRGBA(image.Rect(0, 0, int(host.width), int(host.height)))
	if shmId > 0 {
		host.bgraBuf = shmData
	} else {
		host.bgraBuf = host.imgBuf.Pix
	}

	if shmId > 0 {
		if err := shm.Init(conn); err == nil {
			host.shmSeg, _ = shm.NewSegId(conn)
			if host.shmSeg != 0 {
				shm.Attach(conn, host.shmSeg, uint32(shmId), false)
			}
		}
	}

	host.minKeyCode = setup.MinKeycode
	kmReply, err := xproto.GetKeyboardMapping(conn, host.minKeyCode, byte(setup.MaxKeycode-host.minKeyCode+1)).Reply()
	if err == nil {
		host.keyMap = kmReply.Keysyms
		host.keysPerCode = kmReply.KeysymsPerKeycode
	}

	protocolsAtom, _ := xproto.InternAtom(conn, false, 12, "WM_PROTOCOLS").Reply()
	deleteAtom, _ := xproto.InternAtom(conn, false, 16, "WM_DELETE_WINDOW").Reply()
	if protocolsAtom != nil && deleteAtom != nil {
		host.atomDelete = deleteAtom.Atom
		data := make([]byte, 4)
		xgb.Put32(data, uint32(deleteAtom.Atom))
		xproto.ChangeProperty(conn, xproto.PropModeReplace, host.wid, protocolsAtom.Atom,
xproto.AtomAtom, 32, 1, data)
	}

	// Request maximization via EWMH
	stateAtom, _ := xproto.InternAtom(conn, false, 13, "_NET_WM_STATE").Reply()
	maxVertAtom, _ := xproto.InternAtom(conn, false, 28, "_NET_WM_STATE_MAXIMIZED_VERT").Reply()
	maxHorzAtom, _ := xproto.InternAtom(conn, false, 28, "_NET_WM_STATE_MAXIMIZED_HORZ").Reply()
	if stateAtom != nil && maxVertAtom != nil && maxHorzAtom != nil {
		data := make([]byte, 8)
		xgb.Put32(data, uint32(maxVertAtom.Atom))
		xgb.Put32(data[4:], uint32(maxHorzAtom.Atom))
		xproto.ChangeProperty(conn, xproto.PropModeReplace, host.wid, stateAtom.Atom, xproto.AtomAtom, 32, 2, data)
	}

	xproto.MapWindow(conn, host.wid)
	return host, nil
}

func (h *X11Host) translateModifiers(state uint16) vtinput.ControlKeyState {
	var mods vtinput.ControlKeyState
	if h.lShift || h.rShift {
		mods |= vtinput.ShiftPressed
	}
	if h.lCtrl {
		mods |= vtinput.LeftCtrlPressed
	}
	if h.rCtrl {
		mods |= vtinput.RightCtrlPressed
	}
	if h.lAlt {
		mods |= vtinput.LeftAltPressed
	}
	if h.rAlt {
		mods |= vtinput.RightAltPressed
	}
	if state&xproto.ModMaskLock != 0 {
		mods |= vtinput.CapsLockOn
	}
	if state&xproto.ModMask2 != 0 {
		mods |= vtinput.NumLockOn
	}
	return mods
}

func (h *X11Host) getKeysym(detail xproto.Keycode, state uint16) xproto.Keysym {
	if h.keyMap == nil { return 0 }
	baseIdx := int(detail-h.minKeyCode) * int(h.keysPerCode)
	
	// Bitwise breakdown of X11 state for diagnostic
	shift := (state & xproto.ModMaskShift) != 0
	lock := (state & xproto.ModMaskLock) != 0
	ctrl := (state & xproto.ModMaskControl) != 0
	mod1 := (state & xproto.ModMask1) != 0
	mod2 := (state & xproto.ModMask2) != 0
	mod3 := (state & xproto.ModMask3) != 0
	mod4 := (state & xproto.ModMask4) != 0
	mod5 := (state & xproto.ModMask5) != 0
	group := (state >> 13) & 0x03

	var allSyms []string
	for i := 0; i < int(h.keysPerCode); i++ {
		allSyms = append(allSyms, fmt.Sprintf("%d:0x%X", i, h.keyMap[baseIdx+i]))
	}

	DebugLog("X11_BITS: detail=%d state=0x%04X [S:%v L:%v C:%v M1:%v M2:%v M3:%v M4:%v M5:%v] G:%d",
		detail, state, shift, lock, ctrl, mod1, mod2, mod3, mod4, mod5, group)
	DebugLog("X11_ROW: %s", strings.Join(allSyms, " "))

	col := int(group)*2
	
	// FIX: Disambiguate Mod5. 
	// If physical AltGr is held, we want Level 3 symbols (cols 4-5).
	// Otherwise, if Mod5 is on and Group is 0, we are in the alternate layout (cols 2-3).
	if mod5 && int(h.keysPerCode) > 4 && h.isAltGrPressed {
		col = 4 
	} else if group == 0 && mod5 && int(h.keysPerCode) > 2 {
		col += 2
	}

	if shift { col += 1 }

	if col >= int(h.keysPerCode) {
		oldCol := col
		if h.keysPerCode > 2 {
			col = col % int(h.keysPerCode)
		} else {
			col = col % 2
		}
		DebugLog("X11_GETKEYSYM: Column overflow. detail=%d, keysPerCode=%d, requested=%d, adjusted=%d", detail, h.keysPerCode, oldCol, col)
	}
	
	DebugLog("X11_GETKEYSYM: final_col=%d keysym=0x%X syms=[%s]", 
		col, h.keyMap[baseIdx+col], strings.Join(allSyms, ", "))

	sym := h.keyMap[baseIdx+col]
	if sym == 0 && col > 0 { sym = h.keyMap[baseIdx] }
	return sym
}

func (h *X11Host) Close() {
	if h.shmSeg != 0 { shm.Detach(h.conn, h.shmSeg) }
	h.conn.Close()
	close(h.closeChan)
}

func (h *X11Host) RunEventLoop() {
	for {
		ev, err := h.conn.WaitForEvent()
		if ev == nil && err == nil { return }
		if ev != nil {
			// Log raw event type to see if KeyPresses are even arriving
			DebugLog("X11_HOST_TRACE: Received raw X11 event type: %T", ev)
		}
		switch e := ev.(type) {
		case xproto.ExposeEvent:
			h.mu.Lock()
			for i := range h.dirtyLines { h.dirtyLines[i] = true }
			h.mu.Unlock()
			if e.Count == 0 { h.flushImage() }
		case xproto.ConfigureNotifyEvent:
			if e.Width != h.width || e.Height != h.height {
				h.mu.Lock()
				h.width, h.height = e.Width, e.Height
				h.cols, h.rows = int(e.Width)/h.cellW, int(e.Height)/h.cellH
				h.imgBuf = image.NewRGBA(image.Rect(0, 0, int(h.width), int(h.height)))
				h.dirtyLines = make([]bool, int(e.Height))
				for i := range h.dirtyLines { h.dirtyLines[i] = true }
				h.mu.Unlock()
				if h.reader != nil {
					h.reader.NativeEventChan <- &vtinput.InputEvent{Type: vtinput.ResizeEventType}
				}
			}
		case xproto.KeyPressEvent, xproto.KeyReleaseEvent:
			var detail xproto.Keycode
			var state uint16
			isDown := false
			if kp, ok := e.(xproto.KeyPressEvent); ok {
				detail, state, isDown = kp.Detail, kp.State, true
			} else if kr, ok := e.(xproto.KeyReleaseEvent); ok {
				detail, state, isDown = kr.Detail, kr.State, false
			}

			// Heuristic to fix stuck modifiers after layout switch (e.g., Alt+Shift).
			// If we get a KeyPress for a non-modifier key, and the X11 state mask
			// contradicts our physically tracked state, we likely missed a KeyRelease.
			isModifierKey := false
			switch detail {
			case 37, 105, 64, 108, 50, 62: // l/r ctrl, alt, shift
				isModifierKey = true
			}
			if isDown && !isModifierKey {
				// If our tracker thinks a modifier is down, but X11 state says it's not, reset tracker.
				// This fixes "stuck" modifiers after layout switches (Alt+Shift) or window focus loss.
				if h.lAlt && (state&xproto.ModMask1) == 0 {
					h.lAlt = false
				}
				if h.rAlt && (state&xproto.ModMask5) == 0 {
					h.rAlt = false
					h.isAltGrPressed = false
				}
				if (h.lShift || h.rShift) && (state&xproto.ModMaskShift) == 0 {
					h.lShift, h.rShift = false, false
				}
				if (h.lCtrl || h.rCtrl) && (state&xproto.ModMaskControl) == 0 {
					h.lCtrl, h.rCtrl = false, false
				}
			}

			// Track physical state of modifiers to disambiguate bits and solve state-lag
			switch detail {
			case 50:
				h.lShift = isDown
			case 62:
				h.rShift = isDown
			case 37:
				h.lCtrl = isDown
			case 105:
				h.rCtrl = isDown
			case 64:
				h.lAlt = isDown
			case 108:
				h.rAlt = isDown
				h.isAltGrPressed = isDown
			}

			keysym := h.getKeysym(detail, state)
			vk := keysymToVK(uint32(keysym))
			char := keysymToRune(uint32(keysym))

			charLog := fmt.Sprintf("'%c'", char)
			if char < 32 || char == 127 {
				charLog = fmt.Sprintf("0x%02X", char)
			}
			DebugLog("X11_TRACE: KeyPress detail=%d state=0x%X (bits: %016b) keysym=0x%X vk=0x%X char=%s",
				detail, state, state, uint32(keysym), vk, charLog)

			if h.reader != nil {
				mods := h.translateModifiers(state)
				// If AltGr was used to produce a character (e.g. typographic quotes),
				// we strip the Alt modifier from the event. This prevents f4 from
				// triggering "Fast Find" (Alt+Letter) and lets it process the char as text.
				if char != 0 && h.isAltGrPressed {
					mods &= ^vtinput.RightAltPressed
				}

				h.reader.NativeEventChan <- &vtinput.InputEvent{
					Type: vtinput.KeyEventType, KeyDown: isDown, VirtualKeyCode: vk,
					Char: char, ControlKeyState: mods,
				}
			}
		case xproto.ButtonPressEvent, xproto.ButtonReleaseEvent:
			var bx, by int16
			var detail xproto.Button
			var state uint16
			isDown := false
			if bp, ok := e.(xproto.ButtonPressEvent); ok {
				bx, by, detail, state, isDown = bp.EventX, bp.EventY, bp.Detail, bp.State, true
			} else {
				br := e.(xproto.ButtonReleaseEvent)
				bx, by, detail, state, isDown = br.EventX, br.EventY, br.Detail, br.State, false
			}
			event := &vtinput.InputEvent{
				Type:            vtinput.MouseEventType, MouseX: uint16(int(bx) / h.cellW),
				MouseY:          uint16(int(by) / h.cellH), KeyDown: isDown,
				ControlKeyState: h.translateModifiers(state),
			}
			switch detail {
			case 1: event.ButtonState = vtinput.FromLeft1stButtonPressed
			case 2: event.ButtonState = vtinput.FromLeft2ndButtonPressed
			case 3: event.ButtonState = vtinput.RightmostButtonPressed
			case 4: if isDown { event.WheelDirection = 1 } else { continue }
			case 5: if isDown { event.WheelDirection = -1 } else { continue }
			}
			if h.reader != nil { h.reader.NativeEventChan <- event }
		case xproto.MotionNotifyEvent:
			if h.reader != nil {
				h.reader.NativeEventChan <- &vtinput.InputEvent{
					Type:            vtinput.MouseEventType, MouseX: uint16(int(e.EventX) / h.cellW),
					MouseY:          uint16(int(e.EventY) / h.cellH), MouseEventFlags: vtinput.MouseMoved,
				}
			}
		case xproto.ClientMessageEvent:
			if e.Type == h.atomDelete { FrameManager.EmitCommand(CmQuit, nil) }
		}
	}
}

func (h *X11Host) flushImage() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	b := h.imgBuf.Bounds()
	w, h2 := b.Dx(), b.Dy()
	if w <= 0 || h2 <= 0 { return 0 }
	minY, maxY := -1, -1
	for y := 0; y < h2; y++ {
		if h.dirtyLines[y] {
			if minY == -1 { minY = y }
			maxY = y
		}
	}
	if minY == -1 { return 0 }
	for y := minY; y <= maxY; y++ { h.dirtyLines[y] = false }

	if h.shmSeg != 0 {
		stride := w * 4
		for y := minY; y <= maxY; y++ {
			srcOff, dstOff := y*stride, y*stride
			if dstOff+stride > len(h.bgraBuf) || srcOff+stride > len(h.imgBuf.Pix) { continue }
			srcRow, dstRow := h.imgBuf.Pix[srcOff:srcOff+stride], h.bgraBuf[dstOff:dstOff+stride]
			for i := 0; i < stride; i += 4 {
				dstRow[i], dstRow[i+1], dstRow[i+2], dstRow[i+3] = srcRow[i+2], srcRow[i+1], srcRow[i], 255
			}
		}
		// shm.PutImage: 16 arguments
		shm.PutImage(h.conn, xproto.Drawable(h.wid), h.gc,
			uint16(w), uint16(h2), // total_width, total_height
			0, uint16(minY), // src_x, src_y
			uint16(w), uint16(maxY-minY+1), // src_width, src_height
			0, int16(minY), // dst_x, dst_y
			24, 2, 0, // depth, format (ZPixmap), send_event
			h.shmSeg, 0) // offset должен быть 0, сдвиг задается через src_y
		return 1
	}

	pix, lineStride := h.imgBuf.Pix, w*4
	maxReq := int(xproto.Setup(h.conn).MaximumRequestLength) * 4
	rowsPerReqLimit := (maxReq - 24) / lineStride
	putCalls := 0
	for y := minY; y <= maxY; {
		chunkEnd := y + rowsPerReqLimit
		if chunkEnd > maxY+1 { chunkEnd = maxY + 1 }
		xproto.PutImage(h.conn, xproto.ImageFormatZPixmap, xproto.Drawable(h.wid), h.gc,
			uint16(w), uint16(chunkEnd-y), 0, int16(y), 0, 24, pix[y*lineStride:chunkEnd*lineStride])
		putCalls++
		y = chunkEnd
	}
	return putCalls
}
