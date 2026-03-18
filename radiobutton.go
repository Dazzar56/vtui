package vtui

import (
	"github.com/unxed/vtinput"
	"github.com/mattn/go-runewidth"
)

// RadioButton представляет собой переключатель в группе.
type RadioButton struct {
	ScreenObject
	Text     string
	Selected bool
}

func NewRadioButton(x, y int, text string) *RadioButton {
	rb := &RadioButton{
		Text: text,
	}
	clean, hk, _ := ParseAmpersandString(text)
	rb.hotkey = hk
	rb.canFocus = true
	// Формат: "( ) Текст" или "(•) Текст"
	vLen := 4 + runewidth.StringWidth(clean)
	rb.SetPosition(x, y, x+vLen-1, y)
	return rb
}

func (rb *RadioButton) Show(scr *ScreenBuf) {
	rb.ScreenObject.Show(scr)
	rb.DisplayObject(scr)
}

func (rb *RadioButton) DisplayObject(scr *ScreenBuf) {
	if !rb.IsVisible() { return }

	attr := Palette[ColDialogText]
	highAttr := Palette[ColDialogHighlightText]
	if rb.IsFocused() {
		attr = Palette[ColDialogSelectedButton]
		highAttr = Palette[ColDialogHighlightSelectedButton]
	}

	state := "( ) "
	if rb.Selected {
		state = "(•) "
	}

	cells, _ := StringToCharInfoHighlighted(state+rb.Text, attr, highAttr)
	scr.Write(rb.X1, rb.Y1, cells)
}

func (rb *RadioButton) ProcessKey(e *vtinput.InputEvent) bool {
	if !e.KeyDown { return false }
	if e.VirtualKeyCode == vtinput.VK_SPACE || e.VirtualKeyCode == vtinput.VK_RETURN {
		// Сама кнопка не меняет состояние напрямую, это сделает Dialog
		return false // Возвращаем false, чтобы Dialog поймал событие и обновил группу
	}
	return false
}

func (rb *RadioButton) ProcessMouse(e *vtinput.InputEvent) bool {
	if e.ButtonState == vtinput.FromLeft1stButtonPressed && e.KeyDown {
		return false // Даем диалогу обработать клик и обновить группу
	}
	return false
}