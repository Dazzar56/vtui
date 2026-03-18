package vtui

import (
	"testing"
	"github.com/unxed/vtinput"
)

func TestEdit_PasswordMode(t *testing.T) {
	SetDefaultPalette()
	scr := NewScreenBuf()
	scr.AllocBuf(10, 1)

	e := NewEdit(0, 0, 10, "abc")
	e.PasswordMode = true
	e.Show(scr)

	// Check that buffer contains '*' instead of 'a'
	// Attributes must match ColDialogEdit
	checkCell(t, scr, 0, 0, '*', Palette[ColDialogEdit])
	checkCell(t, scr, 1, 0, '*', Palette[ColDialogEdit])
	checkCell(t, scr, 2, 0, '*', Palette[ColDialogEdit])
}

func TestEdit_IgnoreLockKeys(t *testing.T) {
	e := NewEdit(0, 0, 10, "")

	// Simulate entering 'x' with NumLock and CapsLock enabled
	e.ProcessKey(&vtinput.InputEvent{
		Type:            vtinput.KeyEventType,
		KeyDown:         true,
		Char:            'x',
		ControlKeyState: vtinput.NumLockOn | vtinput.CapsLockOn,
	})

	if e.GetText() != "x" {
		t.Errorf("Expected 'x', got %q. Lock keys probably blocked the input.", e.GetText())
	}
}

func TestVMenu_ScrollbarMouseClick(t *testing.T) {
	SetDefaultPalette()
	m := NewVMenu("Title")
	// Add 20 items so menu scrolls
	for i := 0; i < 20; i++ {
		m.AddItem("Item")
	}
	m.SetPosition(0, 0, 10, 6) // Height 7, data 5 (Y1+1..Y2-1)

	// Initial state: SelectPos 0

	// 1. Click down arrow (X = X2 = 10, Y = Y2-1 = 5)
	m.ProcessMouse(&vtinput.InputEvent{
		Type: vtinput.MouseEventType, KeyDown: true, MouseX: 10, MouseY: 5, ButtonState: vtinput.FromLeft1stButtonPressed,
	})
	if m.selectPos != 1 {
		t.Errorf("VMenu down arrow click failed, pos %d", m.selectPos)
	}

	// 2. Click up arrow (Y = Y1+1 = 1)
	m.ProcessMouse(&vtinput.InputEvent{
		Type: vtinput.MouseEventType, KeyDown: true, MouseX: 10, MouseY: 1, ButtonState: vtinput.FromLeft1stButtonPressed,
	})
	if m.selectPos != 0 {
		t.Errorf("VMenu up arrow click failed, pos %d", m.selectPos)
	}

	// 3. Page Down click (Y = 4)
	m.ProcessMouse(&vtinput.InputEvent{
		Type: vtinput.MouseEventType, KeyDown: true, MouseX: 10, MouseY: 4, ButtonState: vtinput.FromLeft1stButtonPressed,
	})
	if m.selectPos != 5 { // 0 + height (5) = 5
		t.Errorf("VMenu PageDown click failed, pos %d", m.selectPos)
	}

	// 4. Page Up click (Y = 2)
	m.ProcessMouse(&vtinput.InputEvent{
		Type: vtinput.MouseEventType, KeyDown: true, MouseX: 10, MouseY: 2, ButtonState: vtinput.FromLeft1stButtonPressed,
	})
	if m.selectPos != 0 { // 5 - height (5) = 0
		t.Errorf("VMenu PageUp click failed, pos %d", m.selectPos)
	}
}
func TestVMenu_Hotkeys(t *testing.T) {
	m := NewVMenu("Menu")
	m.AddItem("Open &File")
	m.AddItem("&Save")
	m.AddItem("E&xit")

	// 1. Press 's' (second item hotkey)
	m.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, Char: 's'})

	if m.selectPos != 1 {
		t.Errorf("Expected selectPos 1 for 'Save', got %d", m.selectPos)
	}
	if !m.IsDone() || m.exitCode != 1 {
		t.Error("Menu should be finished with exitCode 1")
	}
}

func TestEdit_History(t *testing.T) {
	e := NewEdit(0, 0, 10, "")
	e.History = []string{"First", "Second"}

	// Simulate Alt+Down
	handled := e.ProcessKey(&vtinput.InputEvent{
		Type:            vtinput.KeyEventType,
		KeyDown:         true,
		VirtualKeyCode:  vtinput.VK_DOWN,
		ControlKeyState: vtinput.LeftAltPressed,
	})

	if !handled {
		t.Error("Alt+Down should be handled when History is present")
	}
}
