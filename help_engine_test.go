package vtui

import (
	"testing"
	"github.com/unxed/f4/vfs"
)

func TestHelpEngine_Parsing(t *testing.T) {
	// Create dummy VFS with help content
	memVfs := vfs.NewOSVFS(t.TempDir())
	helpPath := memVfs.Join(memVfs.GetPath(), "test.hlf")

	content := `
@Contents
$Manual Header
This is a #bold# word.
See ~Introduction~IntroTopic@ for details.
  ^Centered line

@IntroTopic
$Introduction
Welcome to the intro.
`
	wc, _ := memVfs.Create(helpPath)
	wc.Write([]byte(content))
	wc.Close()

	engine := NewHelpEngine(memVfs)
	err := engine.LoadFile(helpPath)
	if err != nil {
		t.Fatalf("Failed to load help file: %v", err)
	}

	// 1. Test topic extraction
	contents := engine.GetTopic("Contents")
	if contents == nil { t.Fatal("Topic 'Contents' not found") }

	// 2. Test sticky headers
	if contents.StickyRows != 1 {
		t.Errorf("Expected 1 sticky row, got %d", contents.StickyRows)
	}
	if contents.Lines[0] != "Manual Header" {
		t.Errorf("Header content mismatch: %q", contents.Lines[0])
	}

	// 3. Test link extraction
	if len(contents.Links) != 1 {
		t.Fatalf("Expected 1 link, got %d", len(contents.Links))
	}
	link := contents.Links[0]
	if link.Text != "Introduction" || link.Target != "IntroTopic" {
		t.Errorf("Link data mismatch: %+v", link)
	}
	if link.Line != 2 { // Line 0 is header, 1 is text, 2 is link
		t.Errorf("Link line mismatch: expected 2, got %d", link.Line)
	}

	// 4. Test multiple topics
	intro := engine.GetTopic("IntroTopic")
	if intro == nil || intro.Lines[1] != "Welcome to the intro." {
		t.Error("IntroTopic parsing failed")
	}
}