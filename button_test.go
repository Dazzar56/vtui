package vtui

import (
	"testing"

	"github.com/unxed/vtinput"
)

func TestButton_OnClick(t *testing.T) {
	b := NewButton(0, 0, "OK")
	clicked := false
	b.OnClick = func() { clicked = true }

	// Test KeyDown Space
	b.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_SPACE})
	if !clicked {
		t.Error("Button should be clicked on Space")
	}

	clicked = false
	// Test KeyDown Return (Buttons SHOULD still handle Return)
	b.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_RETURN})
	if !clicked {
		t.Error("Button should be clicked on Return")
	}

	clicked = false
	// Test Mouse Click
	b.ProcessMouse(&vtinput.InputEvent{Type: vtinput.MouseEventType, KeyDown: true, ButtonState: vtinput.FromLeft1stButtonPressed})
	if !clicked {
		t.Error("Button should be clicked on Left Mouse Button")
	}
}

func TestButton_HotkeyParsing(t *testing.T) {
	b := NewButton(0, 0, "Sa&ve")
	// Check that the constructor correctly extracted 'v' (lowercase)
	if b.GetHotkey() != 'v' {
		t.Errorf("Expected hotkey 'v', got %c", b.GetHotkey())
	}
	// The brackets must not leak into the caption exposed to the outside.
	if b.GetCaption() != "Save" {
		t.Errorf("Expected caption %q, got %q", "Save", b.GetCaption())
	}
	if b.GetText() != "[ Sa&ve ]" {
		t.Errorf("Expected raw text %q, got %q", "[ Sa&ve ]", b.GetText())
	}
	node := b.SemanticNode(&SemanticContext{Width: 80, Height: 25})
	if node["text"] != "Save" {
		t.Errorf("Expected semantic text %q, got %v", "Save", node["text"])
	}
	if node["hotkey"] != "v" {
		t.Errorf("Expected semantic hotkey %q, got %v", "v", node["hotkey"])
	}
}
