package vtui

import (
	"testing"
)

type mockOwner struct {
	ScreenObject
	commandHandled bool
	lastCmd        int
	lastArgs       any
}

func (m *mockOwner) HandleCommand(cmd int, args any) bool {
	m.commandHandled = true
	m.lastCmd = cmd
	m.lastArgs = args
	return true
}

func TestScreenObject_FireAction(t *testing.T) {
	owner := &mockOwner{}
	so := &ScreenObject{}
	so.SetOwner(owner)

	// 1. Test OnClick priority
	clicked := false
	owner.commandHandled = false
	onClick := func() { clicked = true }

	handled := so.FireAction(onClick, 123, nil)

	if !handled {
		t.Error("FireAction should return true when OnClick is handled")
	}
	if !clicked {
		t.Error("OnClick was not called")
	}
	if owner.commandHandled {
		t.Error("Command should not be handled when OnClick is present")
	}

	// 2. Test Command bubbling
	clicked = false
	owner.commandHandled = false

	handled = so.FireAction(nil, 456, "test_args")

	if !handled {
		t.Error("FireAction should return true when command is handled by owner")
	}
	if clicked {
		t.Error("OnClick should not be called when it's nil")
	}
	if !owner.commandHandled {
		t.Error("Command was not bubbled up to owner")
	}
	if owner.lastCmd != 456 || owner.lastArgs != "test_args" {
		t.Errorf("Command data mismatch: cmd=%d, args=%v", owner.lastCmd, owner.lastArgs)
	}

	// 3. Test no action
	owner.commandHandled = false
	handled = so.FireAction(nil, 0, nil)
	if handled {
		t.Error("FireAction should return false when there is nothing to do")
	}
}