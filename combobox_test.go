package vtui

import (
	"testing"
	"github.com/unxed/vtinput"
)

func TestComboBox_Selection(t *testing.T) {
	items := []string{"One", "Two", "Three"}
	cb := NewComboBox(0, 0, 20, items)

	// Initially text is empty
	if cb.Edit.GetText() != "" {
		t.Errorf("Expected empty text, got %q", cb.Edit.GetText())
	}

	// Simulate selecting the second item ("Two") in menu
	if cb.Menu.OnAction != nil {
		cb.Menu.OnAction(1)
	}

	if cb.Edit.GetText() != "Two" {
		t.Errorf("Expected 'Two', got %q", cb.Edit.GetText())
	}
}

func TestComboBox_DropdownOnly(t *testing.T) {
	cb := NewComboBox(0, 0, 20, []string{"A", "B"})
	cb.DropdownOnly = true

	// Attempting to enter text 'X'
	cb.ProcessKey(&vtinput.InputEvent{
		Type:    vtinput.KeyEventType,
		KeyDown: true,
		Char:    'X',
	})

	if cb.Edit.GetText() == "X" {
		t.Error("DropdownOnly ComboBox should not allow manual text entry")
	}
}
func TestComboBox_OpenFlip(t *testing.T) {
	SetDefaultPalette()
	scr := NewSilentScreenBuf()
	scr.AllocBuf(80, 10) // Small height
	FrameManager.Init(scr)

	cb := NewComboBox(0, 8, 20, []string{"Item 1", "Item 2"})

	// ComboBox is at Y=8. Default open is downwards (Y=9).
	// But screen height is 10, so Y=9 is the last line.
	// With 2 items + border, menu height is 4.
	// It MUST flip upwards to fit.
	cb.Open()

	top := FrameManager.GetTopFrame()
	if top == nil || top.GetType() != TypeMenu {
		t.Fatal("Menu not opened")
	}

	_, y1, _, _ := top.GetPosition()
	// ComboBox is at Y=8. Upward flip with height 4 should start at 8-4 = 4.
	if y1 >= 8 {
		t.Errorf("ComboBox menu did not flip upwards. Y1=%d, ComboBoxY=%d", y1, cb.Y1)
	}
}
