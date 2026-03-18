package vtui

import "testing"

func TestShowMessage_Structure(t *testing.T) {
	SetDefaultPalette()
	FrameManager.Init(NewScreenBuf())

	title := "Warning"
	text := "This is a test message\nwith two lines."
	buttons := []string{"&Yes", "&No", "&Cancel"}

	dlg := ShowMessage(title, text, buttons)

	// Проверка количества элементов:
	// 2 строки текста + 3 кнопки = 5 элементов
	if len(dlg.items) != 5 {
		t.Errorf("Wrong item count. Got %d, want 5", len(dlg.items))
	}

	// Проверка заголовка фрейма
	if dlg.frame.title != title {
		t.Errorf("Wrong title. Got %q, want %q", dlg.frame.title, title)
	}

	// Проверка, что кнопки возвращают правильный ExitCode
	for i := 0; i < 3; i++ {
		btn := dlg.items[2+i].(*Button)
		btn.OnClick()
		if dlg.exitCode != i {
			t.Errorf("Button %d failed to set exit code. Got %d", i, dlg.exitCode)
		}
	}
}