package main

import (
	"testing"

	"github.com/unxed/vtui"
)

// The Table Demo dialog must satisfy the same layout rules as framework
// dialogs: no overlaps, "air" between interactive elements, border padding.
func TestTableDemoDialog_Layout(t *testing.T) {
	vtui.SetDefaultPalette()

	scr := vtui.NewSilentScreenBuf()
	scr.AllocBuf(80, 25)
	vtui.FrameManager.Init(scr)

	dlg := buildTableDialog()
	vtui.AssertLayout(t, dlg)
}
