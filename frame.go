package vtui

import (
	"strings"
)

// Frame представляет собой рамочный контейнер, который может иметь заголовок.
// Он встраивает ScreenObject для управления позицией и видимостью.
type Frame struct {
	ScreenObject
	title      string
	boxType    int
	titleColor uint64
	borderColor uint64
}

// NewFrame создает новый экземпляр Frame.
func NewFrame(x1, y1, x2, y2 int, boxType int, title string) *Frame {
	f := &Frame{
		title:   title,
		boxType: boxType,
		// TODO: цвета пока захардкожены, позже будут браться из палитры
		titleColor:  SetRGBFore(0, 0xFFFF00),
		borderColor: SetRGBFore(0, 0x808080),
	}
	f.SetPosition(x1, y1, x2, y2)
	return f
}

// SetTitle устанавливает заголовок для рамки.
func (f *Frame) SetTitle(title string) {
	f.title = title
}

// Show сохраняет фон и вызывает отрисовку объекта.
func (f *Frame) Show(scr *ScreenBuf) {
	if f.IsLocked() {
		return
	}
	f.ScreenObject.Show(scr) // Вызов метода встроенной структуры
	f.DisplayObject(scr)
}

// DisplayObject отрисовывает рамку и заголовок в ScreenBuf.
func (f *Frame) DisplayObject(scr *ScreenBuf) {
	if f.boxType == NoBox || !f.IsVisible() {
		return
	}

	sym := getBoxSymbols(f.boxType)
	w := f.X2 - f.X1 + 1

	// Очистка фона внутри рамки (опционально, но полезно)
	// scr.FillRect(f.X1+1, f.Y1+1, f.X2-1, f.Y2-1, ' ', f.borderColor)

	// Верхняя и нижняя границы
	var topBorder, bottomBorder strings.Builder
	topBorder.WriteRune(sym[bsTL])
	bottomBorder.WriteRune(sym[bsBL])
	for i := 0; i < w-2; i++ {
		topBorder.WriteRune(sym[bsH])
		bottomBorder.WriteRune(sym[bsH])
	}
	topBorder.WriteRune(sym[bsTR])
	bottomBorder.WriteRune(sym[bsBR])

	// Вставка заголовка в верхнюю рамку
	topStr := topBorder.String()
	if f.title != "" {
		// Простое усечение, позже можно сделать более умным
		titleLen := len([]rune(f.title))
		if titleLen > w-4 {
			titleLen = w - 4
		}
		startPos := (w - titleLen) / 2
		// Формируем строку с заголовком
		tempTop := []rune(topStr)
		copy(tempTop[startPos-1:], []rune(" "+f.title+" "))
		topStr = string(tempTop)
	}

	// Отрисовка
	scr.Write(f.X1, f.Y1, strToCharInfo(topStr, f.borderColor, f.titleColor, f.title))
	scr.Write(f.X1, f.Y2, strToCharInfo(bottomBorder.String(), f.borderColor, 0, ""))

	// Вертикальные линии
	vertLine := []CharInfo{{Char: uint64(sym[bsV]), Attributes: f.borderColor}}
	for y := f.Y1 + 1; y < f.Y2; y++ {
		scr.Write(f.X1, y, vertLine)
		scr.Write(f.X2, y, vertLine)
	}
}

// Вспомогательная функция для преобразования строки в []CharInfo с учетом цвета заголовка.
func strToCharInfo(str string, borderColor, titleColor uint64, title string) []CharInfo {
	runes := []rune(str)
	info := make([]CharInfo, len(runes))

	titleStart := -1
	if title != "" {
		titleStart = strings.Index(str, title)
	}

	for i, r := range runes {
		isTitle := titleStart != -1 && i >= titleStart && i < titleStart+len(title)
		info[i].Char = uint64(r)
		if isTitle {
			info[i].Attributes = titleColor
		} else {
			info[i].Attributes = borderColor
		}
	}
	return info
}