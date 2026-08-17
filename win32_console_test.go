package vtui

import "testing"

func TestWin32ColorMapping_Ansi16(t *testing.T) {
	cases := []struct {
		ansiIdx uint8
		want    uint16
	}{
		{0, 0},
		{1, Win32FgRed},
		{2, Win32FgGreen},
		{3, Win32FgRed | Win32FgGreen},
		{4, Win32FgBlue},
		{5, Win32FgRed | Win32FgBlue},
		{6, Win32FgGreen | Win32FgBlue},
		{7, Win32FgRed | Win32FgGreen | Win32FgBlue},
		{8, Win32FgIntensity},
		{9, Win32FgIntensity | Win32FgRed},
		{10, Win32FgIntensity | Win32FgGreen},
		{11, Win32FgIntensity | Win32FgRed | Win32FgGreen},
		{12, Win32FgIntensity | Win32FgBlue},
		{13, Win32FgIntensity | Win32FgRed | Win32FgBlue},
		{14, Win32FgIntensity | Win32FgGreen | Win32FgBlue},
		{15, Win32FgIntensity | Win32FgRed | Win32FgGreen | Win32FgBlue},
	}

	for _, tc := range cases {
		got := AnsiIndexToWin32Color(tc.ansiIdx)
		if got != tc.want {
			t.Errorf("AnsiIndexToWin32Color(%d) = %#04x, want %#04x", tc.ansiIdx, got, tc.want)
		}
	}
}

func TestWin32AttrMapping_AttributesAndStyles(t *testing.T) {
	// Index color (1: Red FG, 4: Blue BG)
	attr := SetIndexBoth(0, 1, 4) | ForegroundIntensity | CommonLvbUnderscore
	win32Attr := attrToWin32Attr(attr, nil)

	wantFG := Win32FgRed | Win32FgIntensity
	wantBG := Win32BgBlue
	wantStyles := Win32CommonLvbUnderscore

	expected := wantFG | wantBG | wantStyles
	if win32Attr != expected {
		t.Errorf("attrToWin32Attr() = %#04x, want %#04x (FG:%#04x BG:%#04x Styles:%#04x)",
			win32Attr, expected, wantFG, wantBG, wantStyles)
	}
}

func TestWin32AttrMapping_RGBQuantization(t *testing.T) {
	// Pure Green RGB FG (0x00FF00), Pure Red RGB BG (0xFF0000)
	attr := SetRGBBoth(0, 0x00FF00, 0xFF0000)
	win32Attr := attrToWin32Attr(attr, &XTerm256Palette)

	// ANSI Green is 2 (or 10), Red is 1 (or 9)
	fg := win32Attr & 0x000F
	bg := (win32Attr & 0x00F0) >> 4

	if fg&Win32FgGreen == 0 {
		t.Errorf("expected Green FG bit in %#04x", fg)
	}
	if bg&Win32FgRed == 0 {
		t.Errorf("expected Red BG bit in %#04x", bg)
	}
}

func TestCharInfoToWin32(t *testing.T) {
	ci := CharInfo{
		Char:       'A',
		Attributes: SetIndexBoth(0, 7, 0),
	}
	wCi := charInfoToWin32(ci, nil)

	if wCi.UnicodeChar != 'A' {
		t.Errorf("UnicodeChar = %c, want 'A'", rune(wCi.UnicodeChar))
	}
	if wCi.Attributes != (Win32FgRed | Win32FgGreen | Win32FgBlue) {
		t.Errorf("Attributes = %#04x, want light gray (%#04x)", wCi.Attributes, Win32FgRed|Win32FgGreen|Win32FgBlue)
	}

	// WideCharFiller should become a space
	ciFiller := CharInfo{Char: WideCharFiller, Attributes: 0}
	wCiFiller := charInfoToWin32(ciFiller, nil)
	if wCiFiller.UnicodeChar != ' ' {
		t.Errorf("WideCharFiller = %c, want ' '", rune(wCiFiller.UnicodeChar))
	}

	// Zero char should become a space
	ciZero := CharInfo{Char: 0, Attributes: 0}
	wCiZero := charInfoToWin32(ciZero, nil)
	if wCiZero.UnicodeChar != ' ' {
		t.Errorf("Zero char = %c, want ' '", rune(wCiZero.UnicodeChar))
	}
}

func TestWin32ConsoleRenderer_ImplementsSurfaceRenderer(t *testing.T) {
	var _ SurfaceRenderer = (*Win32ConsoleRenderer)(nil)
}
