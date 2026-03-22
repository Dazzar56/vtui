package vtui

import (
	"testing"
)

func TestBaseWindow_ShadowFlag(t *testing.T) {
	bw := NewBaseWindow(0, 0, 10, 10, "Title")
	if !bw.HasShadow() {
		t.Error("BaseWindow (Dialogs/Windows) should have shadows enabled by default")
	}
}

func TestBaseWindow_HandleCommand(t *testing.T) {
	bw := NewBaseWindow(0, 0, 10, 10, "Command Test")

	// Add an element to test bubbling down
	btn := NewButton(1, 1, "Btn")
	//clicked := false
	//btn.OnClick = func() { clicked = true }
	bw.AddItem(btn)
	bw.focusIdx = 0

	// 1. Test custom command (should bubble to UI Element, but button ignores raw commands by default)
	handled := bw.HandleCommand(999, nil)
	if handled {
		t.Error("Unrecognized command should not be handled")
	}

	// 2. Test built-in Window command (CmClose)
	if bw.IsDone() {
		t.Fatal("Window should not be done initially")
	}

	bw.HandleCommand(CmClose, nil)

	if !bw.IsDone() {
		t.Error("CmClose command should close the BaseWindow")
	}
}
