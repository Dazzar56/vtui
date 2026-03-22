package vtui

import (
	"github.com/unxed/vtinput"
)

// UIElement is the interface that all dialog elements must implement.
type UIElement interface {
	GetPosition() (int, int, int, int)
	SetPosition(int, int, int, int)
	GetGrowMode() GrowMode
	Show(scr *ScreenBuf)
	Hide(scr *ScreenBuf)
	SetFocus(bool)
	IsFocused() bool
	CanFocus() bool
	GetHotkey() rune
	GetHelp() string
	ProcessKey(e *vtinput.InputEvent) bool
	ProcessMouse(e *vtinput.InputEvent) bool
	HandleCommand(cmd int, args any) bool
}

// Dialog is a modal container for UI elements.
type Dialog struct {
	BaseWindow
}

func NewDialog(x1, y1, x2, y2 int, title string) *Dialog {
	d := &Dialog{
		BaseWindow: *NewBaseWindow(x1, y1, x2, y2, title),
	}
	return d
}

func (d *Dialog) IsModal() bool { return true }
func (d *Dialog) GetType() FrameType { return TypeDialog }
