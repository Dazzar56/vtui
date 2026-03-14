package vtui

import (
	"testing"
)

func TestStringToCharInfo_WideChars(t *testing.T) {
	// "A世B" -> A (1), 世 (2), B (1) -> Total visual width: 4
	str := "A世B"
	attr := uint64(123)

	ci := StringToCharInfo(str, attr)

	if len(ci) != 4 {
		t.Fatalf("Expected 4 cells, got %d", len(ci))
	}

	if ci[0].Char != 'A' {
		t.Errorf("Cell 0: expected 'A', got %c", ci[0].Char)
	}
	if ci[1].Char != '世' {
		t.Errorf("Cell 1: expected '世', got %c", ci[1].Char)
	}
	if ci[2].Char != WideCharFiller {
		t.Errorf("Cell 2: expected WideCharFiller, got %X", ci[2].Char)
	}
	if ci[3].Char != 'B' {
		t.Errorf("Cell 3: expected 'B', got %c", ci[3].Char)
	}
}