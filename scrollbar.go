package vtui

// Symbols for the scrollbar, similar to Oem2Unicode from far2l
const (
	ScrollUpArrow    = '▲' // 0x25B2
	ScrollDownArrow  = '▼' // 0x25BC
	ScrollBlockLight = '░' // 0x2591 (BS_X_B0)
	ScrollBlockDark  = '▓' // 0x2593 (BS_X_B2)
)

// MathRound performs mathematical rounding of x / y
func MathRound(x, y uint64) uint64 {
	if y == 0 {
		return 0
	}
	return (x + (y / 2)) / y
}

// Max returns the maximum of two numbers
func Max(a, b uint64) uint64 {
	if a > b {
		return a
	}
	return b
}

// Min returns the minimum of two numbers
func Min(a, b uint64) uint64 {
	if a < b {
		return a
	}
	return b
}

// CalcScrollBar calculates the position and size of the scrollbar thumb.
// Returns caretPos (offset from the top arrow, from 0) and caretLength (thumb size).
func CalcScrollBar(length, topItem, itemsCount int) (caretPos, caretLength int) {
	if length <= 2 || itemsCount <= 0 || length >= itemsCount {
		return 0, 0
	}

	trackLen := uint64(length - 2)
	total := uint64(itemsCount)
	viewHeight := uint64(length)
	top := uint64(topItem)

	// Calculate thumb size (proportional to the visible area)
	cLen := MathRound(trackLen*viewHeight, total)
	if cLen < 1 {
		cLen = 1
	}
	if cLen >= trackLen {
		cLen = trackLen - 1
	}

	// Calculate maximum values for content scroll and the thumb itself
	maxTop := total - viewHeight
	if top > maxTop {
		top = maxTop
	}

	maxCaret := trackLen - cLen
	cPos := uint64(0)
	if maxTop > 0 {
		// Exact proportion guarantees touching the bottom edge at the very end of the list
		cPos = MathRound(top*maxCaret, maxTop)
	}

	return int(cPos), int(cLen)
}

// DrawScrollBar draws a vertical scrollbar.
// x, y - coordinates of the top character (up arrow).
// length - total scrollbar length (including 2 arrows).
// topItem - index of the first visible element.
// itemsCount - total number of elements in the list.
// attr - color attribute for drawing.
func DrawScrollBar(scr *ScreenBuf, x, y, length int, topItem, itemsCount int, attr uint64) bool {
	caretPos, caretLength := CalcScrollBar(length, topItem, itemsCount)
	if caretLength == 0 {
		return false // Scrollbar is not needed
	}

	trackLen := length - 2

	// 1. Top arrow
	scr.Write(x, y, []CharInfo{{Char: uint64(ScrollUpArrow), Attributes: attr}})

	// 2. Track
	for i := 0; i < trackLen; i++ {
		char := ScrollBlockLight
		if i >= caretPos && i < caretPos+caretLength {
			char = ScrollBlockDark
		}
		scr.Write(x, y+1+i, []CharInfo{{Char: uint64(char), Attributes: attr}})
	}

	// 3. Bottom arrow
	scr.Write(x, y+length-1, []CharInfo{{Char: uint64(ScrollDownArrow), Attributes: attr}})

	return true
}