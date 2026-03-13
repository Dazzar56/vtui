package vtui

// SaveScreen stores a copy of a rectangular area from ScreenBuf.
// Analog of SaveScreen from savescr.cpp.
type SaveScreen struct {
	x1, y1, x2, y2 int
	width          int
	data           []CharInfo
}

// NewSaveScreen creates a new SaveScreen, copying the specified area from ScreenBuf.
func NewSaveScreen(scr *ScreenBuf, x1, y1, x2, y2 int) *SaveScreen {
	scr.mu.Lock()
	defer scr.mu.Unlock()

	if x1 < 0 { x1 = 0 }
	if y1 < 0 { y1 = 0 }
	if x2 >= scr.width { x2 = scr.width - 1 }
	if y2 >= scr.height { y2 = scr.height - 1 }

	width := x2 - x1 + 1
	height := y2 - y1 + 1

	if width <= 0 || height <= 0 {
		return nil
	}

	ss := &SaveScreen{
		x1: x1, y1: y1, x2: x2, y2: y2,
		width: width,
		data:  make([]CharInfo, width*height),
	}

	for y := 0; y < height; y++ {
		srcOffset := (y1 + y) * scr.width + x1
		dstOffset := y * width
		copy(ss.data[dstOffset:dstOffset+width], scr.buf[srcOffset:srcOffset+width])
	}

	return ss
}

// Restore restores the saved area back to ScreenBuf.
func (ss *SaveScreen) Restore(scr *ScreenBuf) {
	if ss == nil || ss.data == nil {
		return
	}

	scr.mu.Lock()
	defer scr.mu.Unlock()

	height := ss.y2 - ss.y1 + 1

	for y := 0; y < height; y++ {
		// Verify that coordinates are still within buffer bounds, in case of a resize.
		if ss.y1+y >= scr.height || ss.x1 >= scr.width {
			continue
		}

		dstOffset := (ss.y1 + y) * scr.width + ss.x1
		srcOffset := y * ss.width

		// Determine how much can actually be copied
		copyLen := ss.width
		if ss.x1+copyLen > scr.width {
			copyLen = scr.width - ss.x1
		}

		copy(scr.buf[dstOffset:dstOffset+copyLen], ss.data[srcOffset:srcOffset+copyLen])
	}
}