package vtui

import (
	"testing"

	"github.com/unxed/vtinput"
)

func TestDesktop_ExitKeys(t *testing.T) {
	d := NewDesktop()

	// F10 should set exit code to -1
	d.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_F10})
	if !d.IsDone() || d.exitCode != -1 {
		t.Error("Desktop should exit with -1 on F10")
	}

	// Reset state
	d.done = false
	d.exitCode = 0

	// ESC should set exit code to -1
	d.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_ESCAPE})
	if !d.IsDone() || d.exitCode != -1 {
		t.Error("Desktop should exit with -1 on ESC")
	}
}