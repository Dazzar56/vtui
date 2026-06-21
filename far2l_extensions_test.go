package vtui

import (
	"testing"
	"time"

	"github.com/unxed/vtinput"
)

func TestFar2lClipboard_Disabled(t *testing.T) {
	Far2lEnabled = false
	ok := SetFar2lClipboard("test")
	if ok {
		t.Error("SetFar2lClipboard should return false when Far2lEnabled is false")
	}

	_, ok = GetFar2lClipboard()
	if ok {
		t.Error("GetFar2lClipboard should return false when Far2lEnabled is false")
	}
}

func TestFar2lInteract_Timeout(t *testing.T) {
	Far2lEnabled = true
	// Init minimal FrameManager
	FrameManager = &frameManager{}
	FrameManager.EventChan = make(chan *vtinput.InputEvent)
	// RedrawChan needs buffer to not block
	FrameManager.RedrawChan = make(chan struct{}, 1)

	stk := &vtinput.Far2lStack{}
	stk.PushU8('w') // request window size

	// Start interaction that waits for reply
	start := time.Now()
	// We wrap it in a goroutine because it blocks
	var reply *vtinput.Far2lStack
	done := make(chan bool)
	go func() {
		reply = Far2lInteract(stk, true)
		done <- true
	}()

	// Wait for timeout (we use short timeout in mock or just check time elapsed)
	select {
	case <-done:
		if reply != nil {
			t.Error("Expected nil reply on timeout")
		}
		if time.Since(start) < 100*time.Millisecond {
			t.Error("Timeout happened too fast")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Interaction did not timeout, it hung")
	}
}

func TestFar2lInteract_Success(t *testing.T) {
	Far2lEnabled = true
	idToWait := uint8(0)

	// Mocking the interaction loop
	localFm := &frameManager{}
	localFm.EventChan = make(chan *vtinput.InputEvent, 1)
	localFm.injectedEvents = make([]*vtinput.InputEvent, 0)
	FrameManager = localFm

	stk := &vtinput.Far2lStack{}
	stk.PushU8('w')

	go func() {
		// Wait for the ID to be assigned by Far2lInteract
		for far2lIDCounter == 0 {
			time.Sleep(10 * time.Millisecond)
		}
		idToWait = far2lIDCounter

		// Prepare reply
		resp := vtinput.Far2lStack{}
		resp.PushU16(24) // height
		resp.PushU16(80) // width
		resp.PushU8(idToWait)

		localFm.EventChan <- &vtinput.InputEvent{
			Type:         vtinput.Far2lEventType,
			Far2lCommand: "reply",
			Far2lData:    resp,
		}
	}()

	go func() {
		for {
			ev, ok := <-localFm.EventChan
			if !ok {
				return
			}
			localFm.dispatchEvent(ev, false)
		}
	}()

	reply := Far2lInteract(stk, true)
	close(localFm.EventChan)

	if reply == nil {
		t.Fatal("Interaction failed to receive reply")
	}

	if w := reply.PopU16(); w != 80 {
		t.Errorf("Unexpected data in reply: %d", w)
	}
}

func TestFar2lInteract_NoEventStealing(t *testing.T) {
	Far2lEnabled = true

	localFm := &frameManager{}
	localFm.EventChan = make(chan *vtinput.InputEvent, 10)
	localFm.injectedEvents = make([]*vtinput.InputEvent, 0)
	FrameManager = localFm

	// 1. Push a standard keypress event into the queue
	kp := &vtinput.InputEvent{
		Type:           vtinput.KeyEventType,
		KeyDown:        true,
		VirtualKeyCode: vtinput.VK_A,
		Char:           'a',
	}
	localFm.EventChan <- kp

	stk := &vtinput.Far2lStack{}
	stk.PushU8('w')

	// 2. Start Far2lInteract (which will block waiting for a reply)
	var reply *vtinput.Far2lStack
	done := make(chan bool)
	go func() {
		reply = Far2lInteract(stk, true)
		done <- true
	}()

	// Give it a brief moment to start waiting
	time.Sleep(20 * time.Millisecond)

	// 3. Verify that the keypress event is STILL in the EventChan and has NOT been stolen!
	select {
	case ev := <-localFm.EventChan:
		if ev != kp {
			t.Errorf("Expected to read keypress event, got %v", ev)
		}
	default:
		t.Error("KeyPress event was erroneously stolen or consumed by Far2lInteract wait mechanism!")
	}

	// 4. Now simulate the dispatcher receiving the reply
	idToWait := far2lIDCounter
	resp := vtinput.Far2lStack{}
	resp.PushU16(24) // height
	resp.PushU16(80) // width
	resp.PushU8(idToWait)

	replyEvent := &vtinput.InputEvent{
		Type:         vtinput.Far2lEventType,
		Far2lCommand: "reply",
		Far2lData:    resp,
	}

	// Manually dispatch the reply to satisfy the waiter
	localFm.dispatchEvent(replyEvent, false)

	// 5. Wait for the interaction to complete
	select {
	case <-done:
		if reply == nil {
			t.Fatal("Interaction failed on valid reply")
		}
		if w := reply.PopU16(); w != 80 {
			t.Errorf("Unexpected reply content: %d", w)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Interaction timed out / hung")
	}
}
