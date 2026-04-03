package vtui

import (
	"testing"

	"github.com/unxed/vtinput"
)
import "io"

func TestDesktop_ExitKeys(t *testing.T) {
	// Desktop uses global FrameManager.
	oldScreens := FrameManager.Screens
	defer func() { FrameManager.Screens = oldScreens }()

	// 1. Test F10
	scr := NewScreenBuf()
	scr.Writer = io.Discard
	FrameManager.Init(scr)
	d1 := NewDesktop()
	FrameManager.Push(d1)

	d1.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_F10})
	if !FrameManager.IsShutdown() {
		t.Error("Desktop should trigger Shutdown on F10 via CmQuit")
	}

	// 2. Test ESC (Re-init manager for fresh state)
	scr2 := NewScreenBuf()
	scr2.Writer = io.Discard
	FrameManager.Init(scr2)
	d2 := NewDesktop()
	FrameManager.Push(d2)
	
	d2.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_ESCAPE})
	if !FrameManager.IsShutdown() {
		t.Error("Desktop should trigger Shutdown on ESC via CmQuit")
	}
}
