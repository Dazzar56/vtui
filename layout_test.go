package vtui

import "testing"

func TestVBoxLayout(t *testing.T) {
	vbox := NewVBoxLayout(10, 10, 50, 100)

	btn1 := NewButton(0, 0, "OK") // Base width calculated from text
	edit := NewEdit(0, 0, 10, "") // Dummy width 10

	// Add button with left margin 5, top margin 2
	vbox.Add(btn1, Margins{Top: 2, Left: 5}, AlignLeft)
	// Add edit spanning full width with top margin 1, right margin 5
	vbox.Add(edit, Margins{Top: 1, Right: 5}, AlignFill)

	vbox.Apply()

	// 1. Check Button position
	bx1, by1, _, by2 := btn1.GetPosition()
	if bx1 != 15 || by1 != 12 { // X: 10 + 5, Y: 10 + 2
		t.Errorf("Button position wrong: got (%d, %d)", bx1, by1)
	}
	if by2 != 12 { // Button height is 1
		t.Errorf("Button height changed unexpectedly: Y2 = %d", by2)
	}

	// 2. Check Edit position and stretching
	ex1, ey1, ex2, _ := edit.GetPosition()
	if ey1 != 14 { // Prev Y2 (12) + BottomMargin (0) + Edit TopMargin (1) + 1 = 14
		t.Errorf("Edit Y wrong: expected 14, got %d", ey1)
	}
	if ex1 != 10 || ex2 != 54 { // Width is 50. X1 = 10. X2 = 10+50-1 - 5 (Right margin) = 54
		t.Errorf("Edit bounds wrong: expected (10, 54), got (%d, %d)", ex1, ex2)
	}
}

func TestHBoxLayout_Center(t *testing.T) {
	hbox := NewHBoxLayout(0, 0, 100, 1)
	hbox.HorizontalAlign = AlignCenter
	hbox.Spacing = 2

	btn1 := NewButton(0, 0, "Yes")
	btn2 := NewButton(0, 0, "No")

	hbox.Add(btn1, Margins{}, AlignTop)
	hbox.Add(btn2, Margins{}, AlignTop)

	hbox.Apply()

	b1x1, _, b1x2, _ := btn1.GetPosition()
	b2x1, _, _, _ := btn2.GetPosition()

	// Both buttons are approx 7 chars wide ("[ Yes ]", "[ No  ]").
	// Total width = 7 + 2(spacing) + 7 = 16.
	// Center of 100 is 50. Start X = (100 - 16) / 2 = 42.

	if b1x1 != 42 {
		t.Errorf("Button 1 X wrong: expected 42, got %d", b1x1)
	}
	if b2x1 != b1x2 + 1 + 2 { // Spacing is 2
		t.Errorf("Button spacing wrong: B1X2=%d, B2X1=%d", b1x2, b2x1)
	}
}