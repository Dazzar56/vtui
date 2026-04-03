package vtui

import "github.com/mattn/go-runewidth"

func ShowMessage(title string, text string, buttons []string) *Window {
	dlg := createMessageDialog(title, text, buttons)
	FrameManager.Push(dlg)
	return dlg
}

// ShowMessageOn creates a message box targeted to a specific screen (via an anchor frame).
func ShowMessageOn(anchor Frame, title string, text string, buttons []string) *Window {
	// 1. Create the dialog but DON'T push it yet via the generic FrameManager.Push
	dlg := createMessageDialog(title, text, buttons)

	// 2. Target the specific screen
	FrameManager.PushToFrameScreen(anchor, dlg)
	return dlg
}

// Internal helper to avoid code duplication
func createMessageDialog(title string, text string, buttons []string) *Window {
	const maxDialogWidth = 60
	const sidePadding = 4 // Total side margins (2 left + 2 right)

	// 1. Calculate text dimensions
	lines := WrapText(text, maxDialogWidth-sidePadding)
	textWidth := 0
	for _, l := range lines {
		w := runewidth.StringWidth(l)
		if w > textWidth { textWidth = w }
	}

	// 2. Calculate button dimensions
	btnsWidth := 0
	for _, b := range buttons {
		clean, _, _ := ParseAmpersandString(b)
		btnsWidth += runewidth.StringWidth(clean) + 4 // brackets + spaces
	}
	spacing := 2
	totalBtnsWidth := 0
	if len(buttons) > 0 {
		totalBtnsWidth = btnsWidth + (len(buttons)-1)*spacing
	}

	// 3. Finalize Dialog size
	dlgWidth := textWidth + sidePadding
	if totalBtnsWidth+sidePadding > dlgWidth {
		dlgWidth = totalBtnsWidth + sidePadding
	}
	if title != "" {
		tw := runewidth.StringWidth(title) + 6 // Title padding
		if tw > dlgWidth { dlgWidth = tw }
	}
	if dlgWidth > maxDialogWidth { dlgWidth = maxDialogWidth }

	dlgHeight := len(lines) + 4 // top/bottom padding + borders
	if len(buttons) > 0 {
		dlgHeight += 2 // button row + gap
	}

	dlg := NewCenteredDialog(dlgWidth, dlgHeight, title)

	// 4. Use Layout Engine for positioning
	vbox := NewVBoxLayout(dlg.X1+2, dlg.Y1+2, dlgWidth-4, dlgHeight-4)

	// Add text lines
	for _, l := range lines {
		txt := NewText(0, 0, l, Palette[ColDialogText])
		vbox.Add(txt, Margins{}, AlignCenter)
		dlg.AddItem(txt)
	}

	// Add buttons row
	if len(buttons) > 0 {
		hbox := NewHBoxLayout(0, 0, dlgWidth-4, 1)
		hbox.HorizontalAlign = AlignCenter
		hbox.Spacing = spacing

		for i, b := range buttons {
			btnID := i
			btn := NewButton(0, 0, b)
			btn.OnClick = func() { dlg.SetExitCode(btnID) }
			hbox.Add(btn, Margins{}, AlignTop)
			dlg.AddItem(btn)
		}
		vbox.Add(hbox, Margins{Top: 1}, AlignFill)
	}

	// 5. Calculate and apply all coordinates
	vbox.Apply()

	return dlg
}
