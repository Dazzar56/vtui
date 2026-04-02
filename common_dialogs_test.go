package vtui

import (
	"os"
	"path/filepath"
	"testing"
	"context"
	"time"
	"github.com/unxed/vtinput"
)

// testVFS implements VFSMinimal for testing without coupling to f4's VFS.
type testVFS struct { currentPath string }
func (v *testVFS) GetPath() string { return v.currentPath }
func (v *testVFS) SetPath(p string) error { v.currentPath = p; return nil }
func (v *testVFS) ReadDir(ctx context.Context, p string, onChunk func([]VFSItem)) error {
	entries, _ := os.ReadDir(p)
	items := make([]VFSItem, 0)
	for _, e := range entries { items = append(items, VFSItem{Name: e.Name(), IsDir: e.IsDir()}) }
	if len(items) > 0 && onChunk != nil { onChunk(items) }
	return nil
}
func (v *testVFS) Join(elem ...string) string { return filepath.Join(elem...) }
func (v *testVFS) Dir(p string) string { return filepath.Dir(p) }
func (v *testVFS) Base(p string) string { return filepath.Base(p) }

func TestSelectDirDialog_ArrowVsEnter(t *testing.T) {
	SetDefaultPalette()
	tmpDir := t.TempDir()
	vfs := &testVFS{currentPath: tmpDir}

	dlg := SelectDirDialog("Test", tmpDir, vfs)

	var lb *ListBox
	for _, item := range dlg.rootGroup.items {
		if l, ok := item.(*ListBox); ok { lb = l; break }
	}

	initialPath := vfs.GetPath()

	// 1. Simulate Down Arrow (Select index 0, which is "..")
	// This should trigger OnChange but NOT change the VFS path.
	lb.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_DOWN})

	if vfs.GetPath() != initialPath {
		t.Errorf("Path changed on Arrow Key! Expected %s, got %s", initialPath, vfs.GetPath())
	}

	// 2. Simulate Enter (Action)
	// This should change the VFS path.
	lb.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_RETURN})

	if vfs.GetPath() == initialPath {
		t.Error("Path DID NOT change on Enter Key")
	}
}
func TestInputBox_OkCallback(t *testing.T) {
	SetDefaultPalette()
	FrameManager.Init(NewScreenBuf())

	received := ""
	onOk := func(s string) { received = s }

	dlg := InputBox("Title", "Prompt", "DefaultValue", onOk)

	// Find Edit and Button
	var edit *Edit
	var okBtn *Button
	for _, item := range dlg.rootGroup.items {
		if e, ok := item.(*Edit); ok { edit = e }
		if b, ok := item.(*Button); ok && b.hotkey == 'o' { okBtn = b }
	}

	if edit == nil || okBtn == nil { t.Fatal("Dialog structure missing components") }

	edit.SetText("NewValue")
	if okBtn.OnClick != nil {
		okBtn.OnClick()
	}

	if received != "NewValue" {
		t.Errorf("Expected 'NewValue', got '%s'", received)
	}
	if !dlg.IsDone() {
		t.Error("Dialog should be finished after Ok")
	}
}

func TestSelectFileDialog_Selection(t *testing.T) {
	SetDefaultPalette()
	tmpDir := t.TempDir()
	vfs := &testVFS{currentPath: tmpDir}

	// Create a dummy file
	os.WriteFile(vfs.Join(tmpDir, "dummy.txt"), []byte("data"), 0644)

	dlg := SelectFileDialog("Title", tmpDir, vfs)

	var lb *ListBox
	var fileEdit *Edit
	editCount := 0
	walk(dlg.rootGroup, func(el UIElement) bool {
		if l, ok := el.(*ListBox); ok { lb = l }
		if e, ok := el.(*Edit); ok {
			editCount++
			if editCount == 2 { fileEdit = e }
		}
		return true
	})

	if lb == nil || fileEdit == nil {
		t.Fatal("SelectFileDialog structure error")
	}

	// Wait for async VFS to load items into the listbox
	timeout := time.After(1 * time.Second)
Loop:
	for {
		for _, name := range lb.Items {
			if name == "dummy.txt" { break Loop }
		}
		select {
		case task := <-FrameManager.TaskChan:
			task()
		case <-timeout:
			t.Fatal("Timeout waiting for dummy.txt to appear in list")
		}
	}

	// Find dummy.txt in list
	fileIdx := -1
	for i, name := range lb.Items {
		if name == "dummy.txt" {
			fileIdx = i
			break
		}
	}

	if fileIdx == -1 { t.Fatal("File not found in list") }

	// Change selection to file
	if lb.OnSelect != nil {
		lb.OnSelect(fileIdx)
	}

	if fileEdit.GetText() != "dummy.txt" {
		t.Errorf("File Edit not updated on selection. Got %q", fileEdit.GetText())
	}
}

func TestSelectFileDialog_LayoutBestPractice(t *testing.T) {
	SetDefaultPalette()
	FrameManager.Init(NewScreenBuf())
	v := &testVFS{currentPath: "/tmp"}

	// Create dialog (55x20)
	dlg := SelectFileDialog("LayoutTest", "/tmp", v)

	var fileEdit *Edit
	var btnOk *Button
	var lb *ListBox

	walk(dlg.rootGroup, func(el UIElement) bool {
		if t, ok := el.(*Text); ok && el.GetHotkey() == 'f' {
			if e, ok := t.FocusLink.(*Edit); ok { fileEdit = e }
		}
		if b, ok := el.(*Button); ok && b.GetHotkey() == 'o' { btnOk = b }
		if l, ok := el.(*ListBox); ok { lb = l }
		return true
	})

	if fileEdit == nil || btnOk == nil || lb == nil {
		t.Fatal("Required components not found in dialog")
	}

	// 1. Check ListBox stretch
	lx1, _, lx2, _ := lb.GetPosition()
	if lx1 < dlg.X1 || lx2 > dlg.X2 {
		t.Errorf("ListBox bounds invalid: %d..%d", lx1, lx2)
	}

	// 2. Check File Edit stretch
	ex1, _, ex2, _ := fileEdit.GetPosition()
	if ex1 <= dlg.X1+2 {
		t.Errorf("File Edit overlap with label: X1=%d", ex1)
	}
	if ex2 < ex1 {
		t.Errorf("File Edit has negative width: X1=%d, X2=%d", ex1, ex2)
	}

	// 3. Check Button centering
	bx1, _, _, _ := btnOk.GetPosition()
	if bx1 < dlg.X1 {
		t.Errorf("Button out of bounds: X1=%d", bx1)
	}
}
