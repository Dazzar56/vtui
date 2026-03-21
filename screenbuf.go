package vtui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
)

// ScreenBuf is a complete analog of far2l/scrbuf.cpp.
// It implements double buffering to minimize terminal write operations.
type ScreenBuf struct {
	mu            sync.Mutex
	buf           []CharInfo // 'buf' is the target screen state formed by UI logic.
	shadow        []CharInfo // 'shadow' is the state last rendered in the terminal.
	width, height int

	cursorX, cursorY int
	cursorVisible    bool
	cursorSize       uint32
	cursorDirty      bool

	lockCount int
	dirty     bool // Flag indicating that a full rewrite is required during the next Flush.
	clipStack []Rect
}

// NewScreenBuf creates a new ScreenBuf instance.
func NewScreenBuf() *ScreenBuf {
	return &ScreenBuf{
		dirty: true, // Initially the buffer is "dirty"
	}
}

// AllocBuf allocates or reallocates memory for the screen buffers.
func (s *ScreenBuf) AllocBuf(width, height int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if width == s.width && height == s.height {
		return
	}

	if width <= 0 || height <= 0 {
		s.buf = nil
		s.shadow = nil
		s.width = 0
		s.height = 0
		return
	}

	size := width * height
	newBuf := make([]CharInfo, size)
	newShadow := make([]CharInfo, size)

	if newBuf == nil || newShadow == nil {
		// In Go it is customary to return an error, but for a critical error such as
		// running out of memory for the screen, a panic is justified and matches
		// the behavior of far2l (abort).
		panic(fmt.Sprintf("FATAL: Failed to allocate screen buffer (%d x %d)", width, height))
	}

	s.buf = newBuf
	s.shadow = newShadow
	s.width = width
	s.height = height
	s.dirty = true // After resizing, a full redraw is needed
	s.clipStack = []Rect{{0, 0, width - 1, height - 1}}
}

// PushClipRect adds a new clipping rectangle by intersecting it with the current one.
func (s *ScreenBuf) PushClipRect(x1, y1, x2, y2 int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.clipStack) == 0 {
		if s.width <= 0 || s.height <= 0 {
			return
		}
		s.clipStack = []Rect{{0, 0, s.width - 1, s.height - 1}}
	}
	curr := s.clipStack[len(s.clipStack)-1]
	nx1, ny1 := max(curr.X1, x1), max(curr.Y1, y1)
	nx2, ny2 := min(curr.X2, x2), min(curr.Y2, y2)
	s.clipStack = append(s.clipStack, Rect{nx1, ny1, nx2, ny2})
}

// PopClipRect removes the top clipping rectangle.
func (s *ScreenBuf) PopClipRect() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.clipStack) > 1 {
		s.clipStack = s.clipStack[:len(s.clipStack)-1]
	}
}

// Write writes a slice of CharInfo into the virtual buffer at specified coordinates.
func (s *ScreenBuf) Write(x, y int, text []CharInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.buf == nil || len(s.clipStack) == 0 {
		return
	}

	clip := s.clipStack[len(s.clipStack)-1]
	if y < clip.Y1 || y > clip.Y2 || x > clip.X2 {
		return
	}

	if x < clip.X1 {
		skip := clip.X1 - x
		if skip >= len(text) {
			return
		}
		text = text[skip:]
		x = clip.X1
	}

	if x+len(text)-1 > clip.X2 {
		text = text[:clip.X2-x+1]
	}

	if len(text) == 0 {
		return
	}

	offset := y*s.width + x
	copy(s.buf[offset:], text)
	// Note: not comparing with shadow yet, just copying.
	// Comparison optimization will happen in Flush().
}

// ApplyColor applies specified attributes to a rectangular area.
func (s *ScreenBuf) ApplyColor(x1, y1, x2, y2 int, attributes uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.buf == nil {
		return
	}

	if len(s.clipStack) == 0 { return }
	clip := s.clipStack[len(s.clipStack)-1]
	if x1 < clip.X1 { x1 = clip.X1 }
	if y1 < clip.Y1 { y1 = clip.Y1 }
	if x2 > clip.X2 { x2 = clip.X2 }
	if y2 > clip.Y2 { y2 = clip.Y2 }
	if x1 > x2 || y1 > y2 { return }

	for y := y1; y <= y2; y++ {
		offset := y*s.width + x1
		for x := 0; x <= x2-x1; x++ {
			s.buf[offset+x].Attributes = attributes
		}
	}
}

// FillRect fills a rectangular area with specified character and attributes.
func (s *ScreenBuf) FillRect(x1, y1, x2, y2 int, char rune, attributes uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.buf == nil || len(s.clipStack) == 0 { return }
	clip := s.clipStack[len(s.clipStack)-1]
	if x1 < clip.X1 { x1 = clip.X1 }
	if y1 < clip.Y1 { y1 = clip.Y1 }
	if x2 > clip.X2 { x2 = clip.X2 }
	if y2 > clip.Y2 { y2 = clip.Y2 }
	if x1 > x2 || y1 > y2 { return }
	cell := CharInfo{Char: uint64(char), Attributes: attributes}
	for y := y1; y <= y2; y++ {
		offset := y*s.width + x1
		for x := 0; x <= x2-x1; x++ {
			s.buf[offset+x] = cell
		}
	}
}

func (s *ScreenBuf) SetCursorPos(x, y int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cursorX != x || s.cursorY != y {
		s.cursorX, s.cursorY = x, y
		s.cursorDirty = true
	}
}

func (s *ScreenBuf) SetCursorVisible(visible bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cursorVisible != visible {
		s.cursorVisible = visible
		s.cursorDirty = true
	}
}

func (s *ScreenBuf) Width() int {
	return s.width
}

func (s *ScreenBuf) Height() int {
	return s.height
}

// Tables for quickly converting palettes to ANSI codes.
var (
	ansiFg = []string{"30", "34", "32", "36", "31", "35", "33", "37"}
	ansiBg = []string{"40", "44", "42", "46", "41", "45", "43", "47"}
)

// rgb extracts R, G, B components from 24-bit color (format 0xRRGGBB).
func rgb(c uint32) (r, g, b byte) {
	return byte((c >> 16) & 0xFF), byte((c >> 8) & 0xFF), byte(c & 0xFF)
}

var winToAnsi = []int{0, 4, 2, 6, 1, 5, 3, 7}

func colorToANSI(isBg bool, attr uint64) string {
	flag := ForegroundTrueColor
	rgbVal := GetRGBFore(attr)
	cmd := 38
	if isBg {
		flag = BackgroundTrueColor
		rgbVal = GetRGBBack(attr)
		cmd = 48
	}

	if attr&flag != 0 {
		// Optimization: Check if RGB matches 256-color palette
		if idx, ok := rgbToXTerm[rgbVal]; ok {
			return fmt.Sprintf("%d;5;%d", cmd, idx)
		}
		r, g, b := rgb(rgbVal)
		return fmt.Sprintf("%d;2;%d;%d;%d", cmd, r, g, b)
	}

	// Standard 16 colors (Win32 to ANSI mapping)
	colorPart := uint8(attr & 0x7)
	if isBg {
		colorPart = uint8((attr >> 4) & 0x7)
	}
	ansiColor := winToAnsi[colorPart]

	offset := 30
	if isBg {
		offset = 40
	}
	if attr&ForegroundIntensity != 0 && !isBg {
		offset += 60 // 90-97 for bright FG
	}
	if attr&BackgroundIntensity != 0 && isBg {
		offset += 60 // 100-107 for bright BG
	}

	return strconv.Itoa(offset + ansiColor)
}

// attributesToANSI generates the minimum ANSI sequence to transition from lastAttr to attr.
func attributesToANSI(attr, lastAttr uint64) string {
	if attr == lastAttr {
		return ""
	}

	var params []string

	// Check if we need a full reset (if any flag was removed)
	// IMPORTANT: Mask with 0xFFFF to avoid looking at TrueColor bits (16-63)
	const flagsMask = (ForegroundIntensity | ForegroundDim | CommonLvbUnderscore | CommonLvbReverse | CommonLvbStrikeout) & 0xFFFF
	if (lastAttr&flagsMask) & ^(attr&flagsMask) != 0 {
		params = append(params, "0")
		lastAttr = 0 // After reset, compare with zero state
	}

	// 1. Style Flags
	if attr&ForegroundIntensity != 0 && lastAttr&ForegroundIntensity == 0 { params = append(params, "1") }
	if attr&ForegroundDim != 0 && lastAttr&ForegroundDim == 0 { params = append(params, "2") }
	if attr&CommonLvbUnderscore != 0 && lastAttr&CommonLvbUnderscore == 0 { params = append(params, "4") }
	if attr&CommonLvbReverse != 0 && lastAttr&CommonLvbReverse == 0 { params = append(params, "7") }
	if attr&CommonLvbStrikeout != 0 && lastAttr&CommonLvbStrikeout == 0 { params = append(params, "9") }

	// 2. Foreground Color
	fgMask := ForegroundTrueColor | ForegroundIntensity | 0x7
	if attr&fgMask != lastAttr&fgMask || (attr&ForegroundTrueColor != 0 && GetRGBFore(attr) != GetRGBFore(lastAttr)) {
		params = append(params, colorToANSI(false, attr))
	}

	// 3. Background Color
	bgMask := BackgroundTrueColor | BackgroundIntensity | (0x7 << 4)
	if attr&bgMask != lastAttr&bgMask || (attr&BackgroundTrueColor != 0 && GetRGBBack(attr) != GetRGBBack(lastAttr)) {
		params = append(params, colorToANSI(true, attr))
	}

	if len(params) == 0 {
		return ""
	}

	return "\x1b[" + strings.Join(params, ";") + "m"
}

// Flush compares `buf` and `shadow` and outputs the difference to the terminal.
func (s *ScreenBuf) Flush() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.lockCount > 0 || s.buf == nil {
		return
	}

	var builder strings.Builder

	// 1. Hide the cursor to avoid flickering during rendering.
	builder.WriteString("\x1b[?25l")

	lastAttr := ^uint64(0)
	lastX, lastY := -1, -1

	// Optimization: if nothing is dirty and cursor is in place, do nothing.
	// (Simplified check for now, can be improved).

	// 2. Main comparison and sequence generation loop.
	changesCount := 0
	for y := 0; y < s.height; y++ {
		for x := 0; x < s.width; x++ {
			idx := y*s.width + x

			if !s.dirty && s.buf[idx] == s.shadow[idx] {
				continue
			}

			if changesCount == 0 {
				// First change found, prepare the builder
				builder.WriteString("\x1b[?25l") // Hide cursor
			}
			changesCount++

			if x != lastX+1 || y != lastY {
				builder.WriteString(fmt.Sprintf("\x1b[%d;%dH", y+1, x+1))
			}

			attr := s.buf[idx].Attributes
			builder.WriteString(attributesToANSI(attr, lastAttr))
			lastAttr = attr

			charRaw := s.buf[idx].Char

			if charRaw == WideCharFiller {
				// The terminal already advanced the cursor when drawing the left half.
				// We just update our internal tracker.
				lastX, lastY = x, y
				continue
			}

			if charRaw == 0 {
				builder.WriteByte(' ')
			} else {
				builder.WriteRune(rune(charRaw))
			}

			lastX, lastY = x, y
		}
	}

	if changesCount > 0 || s.dirty || s.cursorDirty {
		s.dirty = false
		s.cursorDirty = false
		copy(s.shadow, s.buf)

		// 3. Move cursor to final position and make visible if needed.
		builder.WriteString(fmt.Sprintf("\x1b[%d;%dH", s.cursorY+1, s.cursorX+1))
		if s.cursorVisible {
			builder.WriteString("\x1b[?25h")
		}

		// 4. Single write to stdout.
		if builder.Len() > 0 {
			os.Stdout.WriteString(builder.String())
		}
	}
}