package vtui

import (
	"github.com/mattn/go-runewidth"
)

// Text represents a simple static text label.
type Text struct {
	ScreenObject
	FocusLink UIElement // Если установлен хоткей, фокус будет передан этому элементу
	content   string
	color     uint64
}

func NewText(x, y int, content string, color uint64) *Text {
	clean, hk, _ := ParseAmpersandString(content)
	t := &Text{content: content, color: color}
	t.hotkey = hk
	vLen := runewidth.StringWidth(clean)
	t.SetPosition(x, y, x+vLen-1, y)
	return t
}

func (t *Text) Show(scr *ScreenBuf) {
	t.ScreenObject.Show(scr)
	t.DisplayObject(scr)
}

func (t *Text) DisplayObject(scr *ScreenBuf) {
	if !t.IsVisible() { return }
	cells, _ := StringToCharInfoHighlighted(t.content, t.color, Palette[ColDialogHighlightText])
	scr.Write(t.X1, t.Y1, cells)
}