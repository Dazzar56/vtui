package vtui

import (
	"reflect"
	"testing"
)

func TestWrapText_Simple(t *testing.T) {
	text := "The quick brown fox jumps"
	// Ширина 10: "The quick ", "brown fox ", "jumps"
	got := WrapText(text, 10)
	want := []string{"The quick", "brown fox", "jumps"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WrapText failed. Got %v, want %v", got, want)
	}
}

func TestWrapText_ForcedBreak(t *testing.T) {
	text := "supercalifragilistic"
	// Ширина 5: должна разбить слово насильно
	got := WrapText(text, 5)
	want := []string{"super", "calif", "ragil", "istic"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Forced break failed. Got %v, want %v", got, want)
	}
}

func TestWrapText_NewLines(t *testing.T) {
	text := "Line 1\nLine 2 is longer\n\nLine 4"
	got := WrapText(text, 10)
	// Ожидаем сохранение пустых строк и переносы внутри длинных
	want := []string{
		"Line 1",
		"Line 2 is",
		"longer",
		"",
		"Line 4",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Explicit newlines failed. Got %v, want %v", got, want)
	}
}

func TestWrapText_Unicode(t *testing.T) {
	// "世" занимает 2 колонки
	text := "A世B世C"
	// Ширина 3: "A世", "B世", "C"
	got := WrapText(text, 3)
	want := []string{"A世", "B世", "C"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Unicode wrap failed. Got %v, want %v", got, want)
	}
}