//go:build windows

package vtui

import (
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	ntdllDLL           = syscall.NewLazyDLL("ntdll.dll")
	procWineGetVersion = ntdllDLL.NewProc("wine_get_version")

	procReadConsoleOutputW           = kernel32.NewProc("ReadConsoleOutputW")
	procWriteConsoleOutputW          = kernel32.NewProc("WriteConsoleOutputW")
	procSetConsoleCursorPosition     = kernel32.NewProc("SetConsoleCursorPosition")
	procSetConsoleTitleW             = kernel32.NewProc("SetConsoleTitleW")
	procGetConsoleScreenBufferInfo   = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procCreateConsoleScreenBuffer    = kernel32.NewProc("CreateConsoleScreenBuffer")
	procSetConsoleActiveScreenBuffer = kernel32.NewProc("SetConsoleActiveScreenBuffer")
	procSetConsoleScreenBufferSize   = kernel32.NewProc("SetConsoleScreenBufferSize")
	procSetConsoleWindowInfo         = kernel32.NewProc("SetConsoleWindowInfo")
)

func resetConsoleWindowPos(hConsole syscall.Handle, w, h int16) {
	if hConsole == 0 || hConsole == syscall.InvalidHandle {
		return
	}
	rect := SmallRect{
		Left:   0,
		Top:    0,
		Right:  w - 1,
		Bottom: h - 1,
	}
	procSetConsoleWindowInfo.Call(
		uintptr(hConsole),
		uintptr(1), // TRUE = absolute
		uintptr(unsafe.Pointer(&rect)),
	)
}

var (
	activeWin32ConsoleRenderer *Win32ConsoleRenderer
	activeWin32ConsoleMu       sync.Mutex
)

func setAltScreenWin32(enable bool) {
	activeWin32ConsoleMu.Lock()
	r := activeWin32ConsoleRenderer
	activeWin32ConsoleMu.Unlock()
	if r != nil && r.hFarOut != 0 && r.hFarOut != r.hStdOut {
		if enable {
			procSetConsoleActiveScreenBuffer.Call(uintptr(r.hFarOut))
		} else {
			procSetConsoleActiveScreenBuffer.Call(uintptr(r.hStdOut))
		}
	}
}

func getActiveConsoleHandle() uintptr {
	activeWin32ConsoleMu.Lock()
	r := activeWin32ConsoleRenderer
	activeWin32ConsoleMu.Unlock()
	if r != nil && r.hFarOut != 0 {
		return uintptr(r.hFarOut)
	}
	return 0
}

// win32ConsoleActive reports whether the classic Windows Console API renderer
// currently owns the visible screen via its own dedicated screen buffer
// (hFarOut distinct from hStdOut). When it does, the console buffer the user
// sees after leaving f4's screen (e.g. Ctrl+O under WINE.md's no-PTY console
// view) is not a VT stream: it is hStdOut, painted directly with
// WriteConsoleOutputW, and ANSI escape sequences written into it would show
// up as literal text or move the buffer's own cursor instead of doing
// anything useful.
func win32ConsoleActive() bool {
	activeWin32ConsoleMu.Lock()
	defer activeWin32ConsoleMu.Unlock()
	r := activeWin32ConsoleRenderer
	return r != nil && r.hFarOut != 0 && r.hFarOut != r.hStdOut
}

type consoleScreenBufferInfo struct {
	dwSize              Coord
	dwCursorPosition    Coord
	wAttributes         uint16
	srWindow            SmallRect
	dwMaximumWindowSize Coord
}

func isWineOS() bool {
	return procWineGetVersion.Find() == nil
}

func hasConsoleBufferOS() bool {
	hOut, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	if err != nil || hOut == syscall.InvalidHandle || hOut == 0 {
		return false
	}
	var csbi consoleScreenBufferInfo
	r1, _, _ := procGetConsoleScreenBufferInfo.Call(uintptr(hOut), uintptr(unsafe.Pointer(&csbi)))
	return r1 != 0 && csbi.dwSize.X > 0 && csbi.dwSize.Y > 0
}

// Win32ConsoleRenderer implements SurfaceRenderer using the classic Windows Console API (WriteConsoleOutputW).
type Win32ConsoleRenderer struct {
	mu          sync.Mutex
	parent      *ScreenBuf
	hStdOut     syscall.Handle
	hFarOut     syscall.Handle
	consoleBuf  []win32CharInfo
	lastCols    int
	lastRows    int
	lastTitle   string
	cursorX     int
	cursorY     int
	cursorVis   bool
	cursorShape CursorShape
	activePal   *[256]uint32
	forceRedraw bool

	// cursorStateSet/lastCursor* let Flush skip SetConsoleCursorInfo/
	// SetConsoleCursorPosition when nothing about the cursor actually
	// changed. Without this, the idle heartbeat that keeps the software
	// (gogpu) cursor blinking also fires here every ~250ms, and each
	// resulting SetConsoleCursorInfo call -- even with identical
	// visible/size -- restarts Wine's native cursor blink timer, making an
	// otherwise-idle cursor blink fast and jittery instead of at its normal
	// rate. Real Windows appears to tolerate the redundant calls; Wine does
	// not. See f4 issue #518 (jittery cursor with panels visible under
	// `wine f4.exe`, fine with panels hidden where the busy gate mostly
	// suppresses this heartbeat).
	cursorStateSet  bool
	lastCursorVis   bool
	lastCursorShape CursorShape
	lastCursorX     int
	lastCursorY     int

	// dirty tracks whether Render() found any actual content change since
	// the last Flush. Flush used to call WriteConsoleOutputW over the whole
	// buffer unconditionally on every call, including the idle heartbeat's
	// ~250ms ticks where nothing changed at all -- rewriting every cell,
	// including whatever glyph sits under the cursor, disturbs Wine's
	// native cursor blink rendering independently of the
	// SetConsoleCursorInfo calls addressed above. See f4 issue #518.
	dirty bool

	// blinkState/lastBlinkTime/blinkStateSet drive the cursor's own blink
	// cycle in software, on a fixed wall-clock period, instead of trusting
	// the console frontend's native blink timer. Testing against real Wine
	// showed the two console frontends disagree about what keeps a native
	// blink alive: wineconsole's blink stops entirely once Flush stops
	// making periodic calls (the dirty-skip above silenced it), while
	// `wine f4.exe` from a unix terminal restarts its blink cycle on every
	// redundant SetConsoleCursorInfo call, however unchanged (jittery,
	// fast). Neither wants the same thing from us. Driving visibility
	// ourselves on a fixed period sidesteps both: the toggle happens at a
	// steady rate regardless of how often Flush is called, and Flush only
	// issues an actual Win32 call on the toggle boundary, not on every tick.
	blinkState    bool
	lastBlinkTime time.Time
	blinkStateSet bool

	// realCursorSet/lastRealCursor* track SetCursor()'s raw request
	// (independent of the blink phase) so a genuine move/visibility change
	// can restart the blink cycle from visible -- otherwise the cursor
	// could stay invisible for up to 530ms right after the user starts
	// typing again, if that happened to land mid blink-off.
	realCursorSet       bool
	lastRealCursorVis   bool
	lastRealCursorShape CursorShape
	lastRealCursorX     int
	lastRealCursorY     int
}

// NewWin32ConsoleRenderer creates a renderer using classic Win32 Console API with a dedicated screen buffer.
func NewWin32ConsoleRenderer(parent *ScreenBuf) *Win32ConsoleRenderer {
	hStdOut, _ := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE)
	hFarOut := hStdOut

	r1, _, _ := procCreateConsoleScreenBuffer.Call(
		uintptr(0xC0000000), // GENERIC_READ | GENERIC_WRITE
		uintptr(3),          // FILE_SHARE_READ | FILE_SHARE_WRITE
		0,
		uintptr(1), // CONSOLE_TEXTMODE_BUFFER
		0,
	)
	if r1 != 0 && syscall.Handle(r1) != syscall.InvalidHandle {
		hFarOut = syscall.Handle(r1)
		procSetConsoleActiveScreenBuffer.Call(uintptr(hFarOut))
	}

	r := &Win32ConsoleRenderer{
		parent:  parent,
		hStdOut: hStdOut,
		hFarOut: hFarOut,
	}

	activeWin32ConsoleMu.Lock()
	activeWin32ConsoleRenderer = r
	activeWin32ConsoleMu.Unlock()

	return r
}

func (r *Win32ConsoleRenderer) Close() error {
	activeWin32ConsoleMu.Lock()
	defer activeWin32ConsoleMu.Unlock()
	if r.hFarOut != 0 && r.hFarOut != r.hStdOut {
		procSetConsoleActiveScreenBuffer.Call(uintptr(r.hStdOut))
		syscall.CloseHandle(r.hFarOut)
		r.hFarOut = r.hStdOut
	}
	if activeWin32ConsoleRenderer == r {
		activeWin32ConsoleRenderer = nil
	}
	return nil
}

func (r *Win32ConsoleRenderer) SetPalette(pal *[256]uint32) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.activePal = pal
}

func (r *Win32ConsoleRenderer) SetCursor(x, y int, visible bool, shape CursorShape) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cursorX = x
	r.cursorY = y
	r.cursorVis = visible
	r.cursorShape = shape
}

func (r *Win32ConsoleRenderer) SetWindowTitle(title string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if title == r.lastTitle {
		return
	}
	r.lastTitle = title
	u16, err := syscall.UTF16PtrFromString(title)
	if err == nil {
		procSetConsoleTitleW.Call(uintptr(unsafe.Pointer(u16)))
	}
}

func (r *Win32ConsoleRenderer) Render(buf, shadow []CharInfo, w, h int, forceRedraw bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if w <= 0 || h <= 0 {
		return
	}

	size := w * h
	if len(r.consoleBuf) != size || r.lastCols != w || r.lastRows != h {
		r.consoleBuf = make([]win32CharInfo, size)
		r.lastCols = w
		r.lastRows = h
		forceRedraw = true
	}

	r.forceRedraw = forceRedraw
	pal := r.activePal
	if pal == nil && r.parent != nil {
		if r.parent.ActivePalette != nil {
			pal = r.parent.ActivePalette
		} else {
			pal = r.parent.ThemePalette
		}
	}

	for i := 0; i < size; i++ {
		if forceRedraw || buf[i] != shadow[i] {
			r.consoleBuf[i] = charInfoToWin32(buf[i], pal)
			r.dirty = true
		}
	}
}

func (r *Win32ConsoleRenderer) Flush() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.consoleBuf) == 0 || r.lastCols <= 0 || r.lastRows <= 0 {
		return
	}

	w := int16(r.lastCols)
	h := int16(r.lastRows)

	targetHandle := r.hFarOut
	if targetHandle == 0 {
		targetHandle = r.hStdOut
	}

	resetConsoleWindowPos(targetHandle, w, h)

	// Drive the blink cycle ourselves on a fixed period instead of trusting
	// the console frontend's native timer (see blinkState's doc comment).
	// A toggle forces both the cursor cell and, if the cursor's own cell
	// happens to be the only thing that would otherwise look unchanged,
	// the content write below, so the OS calls that actually flip
	// visibility keep happening on schedule even while the screen is
	// otherwise fully idle.
	now := time.Now()
	blinkToggled := false
	if !r.blinkStateSet {
		r.blinkState = true
		r.lastBlinkTime = now
		r.blinkStateSet = true
		blinkToggled = true
	} else if now.Sub(r.lastBlinkTime) >= 530*time.Millisecond {
		r.blinkState = !r.blinkState
		r.lastBlinkTime = now
		blinkToggled = true
	}

	// A real cursor move/visibility change (typing, navigation) should show
	// the cursor immediately rather than leaving it stranded in whatever
	// blink phase it happened to be in. Restart the cycle from visible.
	realChanged := !r.realCursorSet ||
		r.lastRealCursorVis != r.cursorVis ||
		r.lastRealCursorShape != r.cursorShape ||
		r.lastRealCursorX != r.cursorX ||
		r.lastRealCursorY != r.cursorY
	if realChanged {
		r.blinkState = true
		r.lastBlinkTime = now
		blinkToggled = true
		r.realCursorSet = true
		r.lastRealCursorVis = r.cursorVis
		r.lastRealCursorShape = r.cursorShape
		r.lastRealCursorX = r.cursorX
		r.lastRealCursorY = r.cursorY
	}

	if blinkToggled {
		r.dirty = true
	}

	if r.dirty {
		bufSize := uintptr(uint32(uint16(w)) | (uint32(uint16(h)) << 16))
		bufCoord := uintptr(0)
		writeRegion := SmallRect{
			Left:   0,
			Top:    0,
			Right:  w - 1,
			Bottom: h - 1,
		}

		procWriteConsoleOutputW.Call(
			uintptr(targetHandle),
			uintptr(unsafe.Pointer(&r.consoleBuf[0])),
			bufSize,
			bufCoord,
			uintptr(unsafe.Pointer(&writeRegion)),
		)
		r.dirty = false
	}

	// Update cursor position and shape, but only when something about the
	// cursor actually changed since the last call -- see cursorStateSet's
	// doc comment for why this matters under Wine. blinkToggled always
	// counts as a change, so the visibility flip itself is never skipped.
	visNow := r.cursorVis && r.blinkState && r.cursorX >= 0 && r.cursorX < int(w) && r.cursorY >= 0 && r.cursorY < int(h)
	unchanged := !blinkToggled && r.cursorStateSet &&
		r.lastCursorVis == visNow &&
		r.lastCursorShape == r.cursorShape &&
		(!visNow || (r.lastCursorX == r.cursorX && r.lastCursorY == r.cursorY))
	if !unchanged {
		if visNow {
			cursorCoord := uintptr(uint32(uint16(r.cursorX)) | (uint32(uint16(r.cursorY)) << 16))
			procSetConsoleCursorPosition.Call(uintptr(targetHandle), cursorCoord)
			SetCursorStyleOS(true, r.cursorShape)
		} else {
			SetCursorStyleOS(false, r.cursorShape)
		}
		r.cursorStateSet = true
		r.lastCursorVis = visNow
		r.lastCursorShape = r.cursorShape
		r.lastCursorX = r.cursorX
		r.lastCursorY = r.cursorY
	}
}
