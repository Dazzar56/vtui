package vtui

// SaveScreen хранит копию прямоугольной области из ScreenBuf.
// Аналог SaveScreen из savescr.cpp.
type SaveScreen struct {
	x1, y1, x2, y2 int
	width          int
	data           []CharInfo
}

// NewSaveScreen создает новый SaveScreen, копируя указанную область из ScreenBuf.
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

// Restore восстанавливает сохраненную область обратно в ScreenBuf.
func (ss *SaveScreen) Restore(scr *ScreenBuf) {
	if ss == nil || ss.data == nil {
		return
	}

	scr.mu.Lock()
	defer scr.mu.Unlock()

	height := ss.y2 - ss.y1 + 1

	for y := 0; y < height; y++ {
		// Проверяем, что координаты все еще в пределах буфера, на случай ресайза.
		if ss.y1+y >= scr.height || ss.x1 >= scr.width {
			continue
		}

		dstOffset := (ss.y1 + y) * scr.width + ss.x1
		srcOffset := y * ss.width

		// Определяем, сколько реально можно скопировать
		copyLen := ss.width
		if ss.x1+copyLen > scr.width {
			copyLen = scr.width - ss.x1
		}

		copy(scr.buf[dstOffset:dstOffset+copyLen], ss.data[srcOffset:srcOffset+copyLen])
	}
}