package vtui

import "testing"

func TestNewLabel_Linkage(t *testing.T) {
	SetDefaultPalette()
	edit := NewEdit(10, 10, 20, "")
	label := NewLabel(0, 0, "&Name:", edit)

	// Проверяем, что ссылка установилась
	if label.FocusLink != edit {
		t.Error("Label FocusLink was not set correctly")
	}

	// Проверяем хоткей
	if label.GetHotkey() != 'n' {
		t.Errorf("Expected hotkey 'n', got %c", label.GetHotkey())
	}

	// Проверяем цвет (должен быть ColDialogText по умолчанию)
	if label.color != Palette[ColDialogText] {
		t.Error("Label did not use default ColDialogText color")
	}
}
