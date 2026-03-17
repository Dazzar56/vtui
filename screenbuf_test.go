package vtui

import (
	"testing"
)

func TestAttributesToANSI(t *testing.T) {
	// 1. Simple Bold + Red
	attr := ForegroundIntensity | ForegroundRed // Red is bit 2, so index 4
	got := attributesToANSI(attr, 0)
	// Expected: 1 (Bold), 91 (Bright Red)
	want := "\x1b[1;91m"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	// 2. TrueColor optimization (Index 208 - Orange)
	orange := uint32(0xFF8700)
	attrTC := SetRGBFore(0, orange)
	gotTC := attributesToANSI(attrTC, 0)
	// Index 208 is Orange. colorToANSI should use 38;5;208
	wantTC := "\x1b[38;5;208m"
	if gotTC != wantTC {
		t.Errorf("TrueColor optimization: got %q, want %q", gotTC, wantTC)
	}

	// 3. Flag removal (Reset)
	attr1 := CommonLvbUnderscore
	attr2 := ForegroundBlue
	gotReset := attributesToANSI(attr2, attr1)
	// attr1 has underscore, attr2 does NOT. Should trigger reset '0'.
	if gotReset[:4] != "\x1b[0;" {
		t.Errorf("Reset expected, got %q", gotReset)
	}
}
func TestScreenBuf_ColorTransitions(t *testing.T) {
	// Проверка перехода от TrueColor (бит 8-15) к обычной 16-цветовой палитре
	tcAttr := SetRGBFore(0, 0xFF0000)
	palAttr := uint64(ForegroundBlue) // Обычный синий

	got := attributesToANSI(palAttr, tcAttr)

	// Так как мы сменили тип цвета (TrueColor -> Palette), должен сработать сброс или
	// явная установка кода 34 (Blue)
	if !contains(got, "34") {
		t.Errorf("Transition to palette failed, ANSI: %q", got)
	}
}

func TestAttributesToANSI_Styles(t *testing.T) {
	// Bold + Strikeout
	attr := ForegroundIntensity | CommonLvbStrikeout
	got := attributesToANSI(attr, 0)
	// Note: result might vary depending on whether we treat 0 as having black/black or no color.
	// But let's verify flags at least.
	if !contains(got, "1") || !contains(got, "9") {
		t.Errorf("Styles missing in %q", got)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}