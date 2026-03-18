package vtui

import "testing"
import "os"
import "path/filepath"

func TestOSVFS_PathLogic(t *testing.T) {
	vfs := NewOSVFS(".")

	// Testing path logic
	joined := vfs.Join("dir", "file.txt")
	if vfs.Base(joined) != "file.txt" {
		t.Errorf("VFS Base failed: %s", vfs.Base(joined))
	}

	dir := vfs.Dir(joined)
	if vfs.Base(dir) != "dir" {
		t.Errorf("VFS Dir failed: %s", dir)
	}
}

func TestOSVFS_RealFilesystem(t *testing.T) {
	tmpDir := t.TempDir()
	vfs := NewOSVFS(tmpDir)

	// 1. Test ReadDir on empty
	items, err := vfs.ReadDir(tmpDir)
	if err != nil || len(items) != 0 {
		t.Errorf("Expected empty dir, got %v, err: %v", items, err)
	}

	// 2. Create a dummy file
	fname := "test.txt"
	os.WriteFile(filepath.Join(tmpDir, fname), []byte("hello"), 0644)

	// 3. Test ReadDir again
	items, err = vfs.ReadDir(tmpDir)
	if err != nil || len(items) != 1 || items[0].Name != fname {
		t.Errorf("ReadDir failed to find file: %v", items)
	}

	// 4. Test Stat
	info, err := vfs.Stat(filepath.Join(tmpDir, fname))
	if err != nil || info.Name != fname || info.IsDir {
		t.Errorf("Stat failed: %v, err: %v", info, err)
	}
}