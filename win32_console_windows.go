//go:build windows

package vtui

import (
	"sync"
	"syscall"
	"unsafe"
)

var (
	ntdllDLL           = syscall.NewLazyDLL("ntdll.dll")
	procWineGetVersion = ntdllDLL.NewProc("wine_get_version")

	procWriteConsoleOutputW          = kernel32.NewProc("WriteConsoleOutputW")
	procSetConsoleCursorPosition     = kernel32.NewProc("SetConsoleCursorPosition")
	procSetConsoleTitleW             = kernel32.NewProc("SetConsoleTitleW")
	procGetConsoleScreenBufferInfo   = kernel32.NewProc("GetConsoleScreenBufferInfo")
	procCreateConsoleScreenBuffer    = kernel32.NewProc("CreateConsoleScreenBuffer")
	procSetConsoleActiveScreenBuffer = kernel32.NewProc("SetConsoleActiveScreenBuffer")
	procSetConsoleScreenBufferSize   = kernel32.NewProc("SetConsoleScreenBufferSize")
)

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

	// Update cursor position and shape
	if r.cursorVis && r.cursorX >= 0 && r.cursorX < int(w) && r.cursorY >= 0 && r.cursorY < int(h) {
		cursorCoord := uintptr(uint32(uint16(r.cursorX)) | (uint32(uint16(r.cursorY)) << 16))
		procSetConsoleCursorPosition.Call(uintptr(targetHandle), cursorCoord)
		SetCursorStyleOS(true, r.cursorShape)
	} else {
		SetCursorStyleOS(false, r.cursorShape)
	}
}
