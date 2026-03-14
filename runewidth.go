package vtui

import (
	"github.com/mattn/go-runewidth"
)

// WideCharFiller is a special marker indicating that this cell in ScreenBuf
// is occupied by the right half of a full-width character (like CJK or Emoji).
const WideCharFiller = ^uint64(0)

// StringToCharInfo converts a string into a slice of CharInfo cells,
// correctly handling double-width characters by inserting WideCharFillers.
// It currently ignores zero-width characters to keep cell alignment strict.
func StringToCharInfo(s string, attr uint64) []CharInfo {
	var res []CharInfo
	for _, r := range s {
		w := runewidth.RuneWidth(r)
		if w > 0 {
			res = append(res, CharInfo{Char: uint64(r), Attributes: attr})
			// Fill the extra cells required by the wide character
			for i := 1; i < w; i++ {
				res = append(res, CharInfo{Char: WideCharFiller, Attributes: attr})
			}
		}
	}
	return res
}

func RunesToCharInfo(runes []rune, attr uint64) []CharInfo {
	return StringToCharInfo(string(runes), attr)
}
