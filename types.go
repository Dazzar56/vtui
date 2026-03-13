package vtui

// Coord определяет координаты в консоли.
type Coord struct {
	X int16
	Y int16
}

// SmallRect определяет прямоугольную область в консоли.
type SmallRect struct {
	Left   int16
	Top    int16
	Right  int16
	Bottom int16
}

// CharInfo содержит символ и его визуальные атрибуты (включая цвета).
// В far2l Char (UnicodeChar) является uint64 (COMP_CHAR) для поддержки композитных символов.
type CharInfo struct {
	Char       uint64 // Эквивалент union с COMP_CHAR UnicodeChar
	Attributes uint64 // Эквивалент DWORD64 Attributes (младшие 16 бит - флаги, 16-39 - Fore RGB, 40-63 - Back RGB)
}