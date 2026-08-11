//go:build (linux || windows || darwin) && !arm

package vtui

import (
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/unxed/vtinput"
)

// mkGrid builds a buf/shadow pair of w*h cells filled with ch and attributes.
func mkGrid(w, h int, ch uint64, attr uint64) (buf, shadow []CharInfo) {
	buf = make([]CharInfo, w*h)
	shadow = make([]CharInfo, w*h)
	for i := range buf {
		buf[i] = CharInfo{Char: ch, Attributes: attr}
	}
	return buf, shadow
}

func TestEbitenRenderer_AllocatesFramebufferForGrid(t *testing.T) {
	r := NewEbitenRenderer(nil, nil, 8, 16)
	buf, shadow := mkGrid(10, 4, ' ', 0)

	r.Render(buf, shadow, 10, 4, true)

	if r.img == nil {
		t.Fatal("expected a framebuffer after the first Render")
	}
	if got, want := r.img.Rect.Dx(), 10*8; got != want {
		t.Errorf("framebuffer width = %d, want %d", got, want)
	}
	if got, want := r.img.Rect.Dy(), 4*16; got != want {
		t.Errorf("framebuffer height = %d, want %d", got, want)
	}
}

// A resized grid must reallocate rather than paint into the old bounds, which
// is how a shrink turns into an out-of-range write.
func TestEbitenRenderer_ReallocatesOnResize(t *testing.T) {
	r := NewEbitenRenderer(nil, nil, 8, 16)

	buf, shadow := mkGrid(10, 4, ' ', 0)
	r.Render(buf, shadow, 10, 4, true)

	buf2, shadow2 := mkGrid(20, 8, ' ', 0)
	r.Render(buf2, shadow2, 20, 8, false)

	if got, want := r.img.Rect.Dx(), 20*8; got != want {
		t.Errorf("width after grow = %d, want %d", got, want)
	}

	buf3, shadow3 := mkGrid(5, 2, ' ', 0)
	r.Render(buf3, shadow3, 5, 2, false)

	if got, want := r.img.Rect.Dx(), 5*8; got != want {
		t.Errorf("width after shrink = %d, want %d", got, want)
	}
	if got, want := r.cols, 5; got != want {
		t.Errorf("cols = %d, want %d", got, want)
	}
}

// Render must survive a short buffer instead of panicking: the grid can be
// resized between AllocBuf and the next flush.
func TestEbitenRenderer_IgnoresInconsistentInput(t *testing.T) {
	r := NewEbitenRenderer(nil, nil, 8, 16)
	buf, shadow := mkGrid(4, 4, ' ', 0)

	r.Render(buf, shadow, 100, 100, true) // claims far more cells than exist
	r.Render(buf, shadow, 0, 0, true)
	r.Render(nil, nil, 4, 4, true)

	if r.img != nil {
		t.Error("expected no framebuffer to be allocated from inconsistent input")
	}
}

func TestEbitenRenderer_TakeFrameClearsDirty(t *testing.T) {
	r := NewEbitenRenderer(nil, nil, 8, 16)
	buf, shadow := mkGrid(4, 2, ' ', 0)
	r.Render(buf, shadow, 4, 2, true)

	pix, w, h, changed := r.takeFrame()
	if !changed {
		t.Error("first takeFrame after a forced render should report a change")
	}
	if pix == nil || w != 32 || h != 32 {
		t.Errorf("takeFrame returned %dx%d, want 32x32", w, h)
	}

	// Nothing rendered since, so the game loop must not re-upload.
	if _, _, _, changed := r.takeFrame(); changed {
		t.Error("takeFrame should report no change when nothing was rendered")
	}
}

// An unchanged grid must not dirty the framebuffer, otherwise a static screen
// uploads a texture every frame.
func TestEbitenRenderer_UnchangedGridDoesNotDirty(t *testing.T) {
	r := NewEbitenRenderer(nil, nil, 8, 16)
	buf, shadow := mkGrid(8, 4, ' ', 0)

	r.Render(buf, shadow, 8, 4, true)
	r.takeFrame()

	copy(shadow, buf)
	r.blinkState = true
	r.lastBlinkTime = time.Now()

	r.Render(buf, shadow, 8, 4, false)
	if _, _, _, changed := r.takeFrame(); changed {
		t.Error("an unchanged grid should not mark the framebuffer dirty")
	}
}

func TestEbitenRenderer_CursorChangeMarksDirty(t *testing.T) {
	r := NewEbitenRenderer(nil, nil, 8, 16)
	buf, shadow := mkGrid(8, 4, ' ', 0)
	r.Render(buf, shadow, 8, 4, true)
	r.takeFrame()

	r.SetCursor(3, 1, true, CursorShapeUnderline)
	if _, _, _, changed := r.takeFrame(); !changed {
		t.Error("moving the cursor should mark the framebuffer dirty")
	}

	r.takeFrame()
	r.SetCursor(3, 1, true, CursorShapeBlock)
	if _, _, _, changed := r.takeFrame(); !changed {
		t.Error("changing cursor shape should mark the framebuffer dirty")
	}
}

func TestEbitenRenderer_TitleIsReportedOnce(t *testing.T) {
	r := NewEbitenRenderer(nil, nil, 8, 16)

	if _, ok := r.takeTitle(); ok {
		t.Error("no title was set, takeTitle should report nothing")
	}

	r.SetWindowTitle("vtui")
	title, ok := r.takeTitle()
	if !ok || title != "vtui" {
		t.Errorf("takeTitle = %q, %v; want \"vtui\", true", title, ok)
	}
	if _, ok := r.takeTitle(); ok {
		t.Error("a title must be reported once, not on every frame")
	}

	// Setting the same title again is not a change.
	r.SetWindowTitle("vtui")
	if _, ok := r.takeTitle(); ok {
		t.Error("setting an identical title should not report a change")
	}
}

// EbitenRenderer must satisfy the interface the ScreenBuf drives it through.
func TestEbitenRenderer_ImplementsSurfaceRenderer(t *testing.T) {
	var _ SurfaceRenderer = (*EbitenRenderer)(nil)
}

func TestEbitenKeyToVK(t *testing.T) {
	cases := []struct {
		key  ebiten.Key
		want uint16
		name string
	}{
		{ebiten.KeyEscape, vtinput.VK_ESCAPE, "Escape"},
		{ebiten.KeyEnter, vtinput.VK_RETURN, "Enter"},
		{ebiten.KeyNumpadEnter, vtinput.VK_RETURN, "NumpadEnter"},
		{ebiten.KeyTab, vtinput.VK_TAB, "Tab"},
		{ebiten.KeyBackspace, vtinput.VK_BACK, "Backspace"},
		{ebiten.KeyPageUp, vtinput.VK_PRIOR, "PageUp"},
		{ebiten.KeyPageDown, vtinput.VK_NEXT, "PageDown"},
		{ebiten.KeyArrowUp, vtinput.VK_UP, "Up"},
		{ebiten.KeyF1, vtinput.VK_F1, "F1"},
		{ebiten.KeyF12, vtinput.VK_F12, "F12"},
		{ebiten.KeyControlLeft, vtinput.VK_LCONTROL, "LeftCtrl"},
		{ebiten.KeyControlRight, vtinput.VK_RCONTROL, "RightCtrl"},
		{ebiten.KeyShiftLeft, vtinput.VK_LSHIFT, "LeftShift"},
		{ebiten.KeyAltRight, vtinput.VK_RMENU, "RightAlt"},
		{ebiten.KeyA, vtinput.VK_A, "A"},
		{ebiten.KeyZ, vtinput.VK_Z, "Z"},
		{ebiten.KeyDigit0, vtinput.VK_0, "0"},
		{ebiten.KeyDigit9, vtinput.VK_9, "9"},
	}
	for _, c := range cases {
		if got := ebitenKeyToVK(c.key); got != c.want {
			t.Errorf("ebitenKeyToVK(%s) = %d, want %d", c.name, got, c.want)
		}
	}
}

// The letter and digit ranges are mapped arithmetically, so every code point
// in them has to land on a real VK rather than merely the endpoints.
func TestEbitenKeyToVK_LetterAndDigitRanges(t *testing.T) {
	for k := ebiten.KeyA; k <= ebiten.KeyZ; k++ {
		want := vtinput.VK_A + uint16(k-ebiten.KeyA)
		if got := ebitenKeyToVK(k); got != want {
			t.Fatalf("letter key %v mapped to %d, want %d", k, got, want)
		}
	}
	for k := ebiten.KeyDigit0; k <= ebiten.KeyDigit9; k++ {
		want := vtinput.VK_0 + uint16(k-ebiten.KeyDigit0)
		if got := ebitenKeyToVK(k); got != want {
			t.Fatalf("digit key %v mapped to %d, want %d", k, got, want)
		}
	}
}

// Unmapped keys must return 0 so the host drops them; VK 0 sent onward would
// read as a real keystroke.
func TestEbitenKeyToVK_UnmappedIsZero(t *testing.T) {
	if got := ebitenKeyToVK(ebiten.KeyMax + 1); got != 0 {
		t.Errorf("out-of-range key mapped to %d, want 0", got)
	}
}

// No two distinct keys may collide on one virtual key code, except where the
// collision is deliberate.
func TestEbitenKeyToVK_NoAccidentalCollisions(t *testing.T) {
	intended := map[uint16]bool{
		vtinput.VK_RETURN: true, // Enter and NumpadEnter share it on purpose
	}
	seen := make(map[uint16]ebiten.Key)
	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		vk := ebitenKeyToVK(k)
		if vk == 0 || intended[vk] {
			continue
		}
		if prev, dup := seen[vk]; dup {
			t.Errorf("keys %v and %v both map to VK %d", prev, k, vk)
		}
		seen[vk] = k
	}
}

// A caret that is not visible must not keep its row repainting: the row was
// dirtied every frame before the painted-cursor state was tracked separately
// from the logical cursor position.
func TestEbitenRenderer_HiddenCursorDoesNotDirtyForever(t *testing.T) {
	r := NewEbitenRenderer(nil, nil, 8, 16)
	buf, shadow := mkGrid(8, 4, ' ', 0)

	r.SetCursor(2, 1, true, CursorShapeBlock)
	r.Render(buf, shadow, 8, 4, true)
	copy(shadow, buf)
	r.takeFrame()

	// Hide the caret. One more repaint is expected, to erase it.
	r.SetCursor(2, 1, false, CursorShapeBlock)
	r.Render(buf, shadow, 8, 4, false)
	if _, _, _, changed := r.takeFrame(); !changed {
		t.Fatal("hiding the caret should repaint its row once, to erase it")
	}

	// From then on the screen is static and must stay clean.
	for i := 0; i < 3; i++ {
		r.Render(buf, shadow, 8, 4, false)
		if _, _, _, changed := r.takeFrame(); changed {
			t.Fatalf("frame %d: hidden caret must not keep dirtying its row", i)
		}
	}
}

// A caret that moves must erase the row it left as well as paint the new one.
func TestEbitenRenderer_MovedCursorErasesOldRow(t *testing.T) {
	r := NewEbitenRenderer(nil, nil, 8, 16)
	buf, shadow := mkGrid(8, 6, ' ', 0)

	r.SetCursor(1, 1, true, CursorShapeBlock)
	r.Render(buf, shadow, 8, 6, true)
	copy(shadow, buf)
	r.takeFrame()

	if !r.paintedCursor || r.paintedCursorY != 1 {
		t.Fatalf("painted caret state = (%v, row %d), want (true, row 1)", r.paintedCursor, r.paintedCursorY)
	}

	r.SetCursor(1, 4, true, CursorShapeBlock)
	r.Render(buf, shadow, 8, 6, false)
	if r.paintedCursorY != 4 {
		t.Errorf("painted caret row = %d after move, want 4", r.paintedCursorY)
	}
	if _, _, _, changed := r.takeFrame(); !changed {
		t.Error("moving the caret should dirty the framebuffer")
	}
}
