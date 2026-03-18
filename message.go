package vtui

import "github.com/mattn/go-runewidth"

// ShowMessage creates and displays a modal window with text and buttons.
// Returns the ID of the pressed button (0, 1...) or -1 on cancellation.
// Since FrameManager works asynchronously, this function returns a Dialog
// that can be tracked.
func ShowMessage(title string, text string, buttons []string) *Dialog {
	const maxDialogWidth = 60
	const padding = 4

	// 1. Text preparation
	lines := WrapText(text, maxDialogWidth-padding)

	textWidth := 0
	for _, l := range lines {
		w := runewidth.StringWidth(l)
		if w > textWidth { textWidth = w }
	}

	// 2. Size calculation
	if title != "" {
		tw := runewidth.StringWidth(title) + 4
		if tw > textWidth { textWidth = tw }
	}

	// Button width calculation
	btnsWidth := 0
	for _, b := range buttons {
		btnsWidth += runewidth.StringWidth(b) + 5 // + brackets and spaces
	}

	dlgWidth := textWidth + padding
	if btnsWidth+padding > dlgWidth {
		dlgWidth = btnsWidth + padding
	}
	if dlgWidth > maxDialogWidth {
		dlgWidth = maxDialogWidth
	}

	dlgHeight := len(lines) + 4 // text + padding + buttons
	if len(buttons) > 0 {
		dlgHeight += 2
	}

	scrWidth := FrameManager.GetScreenSize()
	x1 := (scrWidth - dlgWidth) / 2
	y1 := 6 // fixed top padding

	dlg := NewDialog(x1, y1, x1+dlgWidth-1, y1+dlgHeight-1, title)

	// 3. Content addition
	for i, l := range lines {
		// Center each line of text
		lineW := runewidth.StringWidth(l)
		offX := (dlgWidth - lineW) / 2
		dlg.AddItem(NewText(x1+offX, y1+2+i, l, Palette[ColDialogText]))
	}

	// 4. Buttons addition
	if len(buttons) > 0 {
		// Calculate total width of buttons with spaces between them
		spacing := 2
		totalBtnW := btnsWidth + (len(buttons)-1)*spacing

		currX := x1 + (dlgWidth-totalBtnW)/2
		btnY := y1 + dlgHeight - 2

		for i, b := range buttons {
			btnText := b
			btnID := i
			btn := NewButton(currX, btnY, btnText)
			btn.OnClick = func() {
				dlg.SetExitCode(btnID)
			}
			dlg.AddItem(btn)
			currX += runewidth.StringWidth(btn.text) + spacing
		}
	}

	FrameManager.Push(dlg)
	return dlg
}