package vtui

import (
	"bytes"
	"encoding/json"
	"io"
	"testing"
	"time"

	"github.com/unxed/vtinput"
)

func TestProtocol_Lifecycle(t *testing.T) {
	SetDefaultPalette()
	scr := NewSilentScreenBuf()
	scr.AllocBuf(80, 25)

	fm := &frameManager{}
	fm.Init(scr)
	defer fm.Shutdown()

	oldFM := FrameManager
	FrameManager = fm
	defer func() { FrameManager = oldFM }()

	clientReader, serverWriter := io.Pipe()
	serverReader, clientWriter := io.Pipe()

	session := NewProtocolSession(serverReader, serverWriter, fm)

	go func() {
		_ = session.Serve()
	}()

	// 1. Send "hello"
	helloMsg := `{"op":"hello","seq":1,"version":1}` + "\n"
	_, _ = clientWriter.Write([]byte(helloMsg))

	buf := make([]byte, 1024)
	n, err := clientReader.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read welcome response: %v", err)
	}

	var welcome UpMessage
	if err := json.Unmarshal(buf[:n], &welcome); err != nil {
		t.Fatalf("Failed to parse welcome message: %v (raw: %s)", err, string(buf[:n]))
	}

	if welcome.Op != "welcome" || welcome.ReplyTo != 1 || welcome.Version != 1 {
		t.Errorf("Unexpected welcome payload: %+v", welcome)
	}

	// 2. Send "mount" with dialog (JSON Lines formatted as single line)
	mountMsg := `{"op":"mount","frameId":"testDlg","tree":{"type":"Dialog","id":"testDlg","props":{"title":" Test Dialog "},"children":[{"type":"Edit","id":"userEdit","props":{"text":"Alice"}},{"type":"Button","id":"submitBtn","props":{"text":"&Ok","command":1000}}]}}` + "\n"

	_, _ = clientWriter.Write([]byte(mountMsg))
	time.Sleep(50 * time.Millisecond)

	edit, ok := fm.Lookup("testDlg", "userEdit")
	if !ok {
		t.Fatal("userEdit not mounted")
	}
	if edit.(*Edit).GetText() != "Alice" {
		t.Errorf("Expected edit text 'Alice', got %q", edit.(*Edit).GetText())
	}

	// 3. Send "patch" to update Edit text (JSON Lines single line)
	patchMsg := `{"op":"patch","frameId":"testDlg","ops":[{"kind":"set","id":"userEdit","props":{"text":"Bob"}}]}` + "\n"

	_, _ = clientWriter.Write([]byte(patchMsg))
	time.Sleep(50 * time.Millisecond)

	if edit.(*Edit).GetText() != "Bob" {
		t.Errorf("Expected patched text 'Bob', got %q", edit.(*Edit).GetText())
	}

	// 4. Trigger button command and verify event upstream
	btn, ok := fm.Lookup("testDlg", "submitBtn")
	if !ok {
		t.Fatal("submitBtn not found")
	}

	btn.(*Button).ProcessKey(&vtinput.InputEvent{
		Type:           vtinput.KeyEventType,
		KeyDown:        true,
		VirtualKeyCode: vtinput.VK_RETURN,
	})

	n, err = clientReader.Read(buf)
	if err != nil {
		t.Fatalf("Failed to read command event: %v", err)
	}

	var cmdEvent UpMessage
	if err := json.Unmarshal(buf[:n], &cmdEvent); err != nil {
		t.Fatalf("Failed to parse command event: %v (raw: %s)", err, string(buf[:n]))
	}

	if cmdEvent.Op != "command" || cmdEvent.Cmd != 1000 || cmdEvent.SrcID != "submitBtn" {
		t.Errorf("Unexpected command event: %+v", cmdEvent)
	}

	// 5. Send "quit"
	quitMsg := `{"op":"quit"}` + "\n"
	_, _ = clientWriter.Write([]byte(quitMsg))
	time.Sleep(50 * time.Millisecond)

	if !fm.IsShutdown() {
		t.Error("Expected FrameManager to be shutdown after quit")
	}

	_ = clientWriter.Close()
	_ = serverWriter.Close()
}

func TestProtocol_PipeClosureTeardown(t *testing.T) {
	SetDefaultPalette()
	scr := NewSilentScreenBuf()
	scr.AllocBuf(80, 25)

	fm := &frameManager{}
	fm.Init(scr)

	var inBuf, outBuf bytes.Buffer
	inReader, inWriter := io.Pipe()
	session := NewProtocolSession(inReader, &outBuf, fm)

	go func() {
		_ = session.Serve()
	}()

	// Closing the pipe simulates child/host crash
	_ = inWriter.Close()
	time.Sleep(50 * time.Millisecond)

	if !fm.IsShutdown() {
		t.Error("Session should shutdown FrameManager on pipe closure")
	}
	_ = inBuf
}
