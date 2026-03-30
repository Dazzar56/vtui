package vtui

import (
	"testing"
	"github.com/unxed/f4/vfs"
	"github.com/unxed/vtinput"
)

func TestHelpView_Navigation(t *testing.T) {
	memVfs := vfs.NewOSVFS(t.TempDir())
	helpPath := memVfs.Join(memVfs.GetPath(), "test.hlf")
	content := `
@Contents
~GoToNext~NextTopic@

@NextTopic
Success
`
	wc, _ := memVfs.Create(helpPath)
	wc.Write([]byte(content))
	wc.Close()

	engine := NewHelpEngine(memVfs)
	engine.LoadFile(helpPath)

	hv := NewHelpView(engine, "Contents")

	// 1. Initial state
	if hv.current.Name != "Contents" { t.Errorf("Expected Contents, got %s", hv.current.Name) }
	if hv.selectedIdx != 0 { t.Error("Link should be selected by default") }

	// 2. Press Enter to jump
	hv.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_RETURN})

	if hv.current.Name != "NextTopic" {
		t.Errorf("Jump failed, current is %s", hv.current.Name)
	}
	if len(hv.history) != 1 || hv.history[0] != "Contents" {
		t.Error("History not updated after jump")
	}

	// 3. Press Backspace to return
	hv.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_BACK})
	if hv.current.Name != "Contents" {
		t.Error("History Back failed")
	}
}