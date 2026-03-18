package vtui

import "github.com/mattn/go-runewidth"

// ShowMessage создает и отображает модальное окно с текстом и кнопками.
// Возвращает ID нажатой кнопки (0, 1...) или -1 при отмене.
// Поскольку FrameManager работает асинхронно, эта функция возвращает Dialog,
// который можно отслеживать.
func ShowMessage(title string, text string, buttons []string) *Dialog {
	const maxDialogWidth = 60
	const padding = 4

	// 1. Подготовка текста
	lines := WrapText(text, maxDialogWidth-padding)

	textWidth := 0
	for _, l := range lines {
		w := runewidth.StringWidth(l)
		if w > textWidth { textWidth = w }
	}

	// 2. Расчет размеров
	if title != "" {
		tw := runewidth.StringWidth(title) + 4
		if tw > textWidth { textWidth = tw }
	}

	// Расчет ширины кнопок
	btnsWidth := 0
	for _, b := range buttons {
		btnsWidth += runewidth.StringWidth(b) + 5 // + скобки и пробелы
	}

	dlgWidth := textWidth + padding
	if btnsWidth+padding > dlgWidth {
		dlgWidth = btnsWidth + padding
	}
	if dlgWidth > maxDialogWidth {
		dlgWidth = maxDialogWidth
	}

	dlgHeight := len(lines) + 4 // текст + отступы + кнопки
	if len(buttons) > 0 {
		dlgHeight += 2
	}

	scrWidth := FrameManager.GetScreenSize()
	x1 := (scrWidth - dlgWidth) / 2
	y1 := 6 // фиксированный отступ сверху

	dlg := NewDialog(x1, y1, x1+dlgWidth-1, y1+dlgHeight-1, title)

	// 3. Добавление контента
	for i, l := range lines {
		// Центрируем каждую строку текста
		lineW := runewidth.StringWidth(l)
		offX := (dlgWidth - lineW) / 2
		dlg.AddItem(NewText(x1+offX, y1+2+i, l, Palette[ColDialogText]))
	}

	// 4. Добавление кнопок
	if len(buttons) > 0 {
		// Вычисляем общую ширину кнопок с пробелами между ними
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