package vtui

import (
	"fmt"
	"github.com/mattn/go-runewidth"
)

// KeyBarLabels stores labels for F1-F12 for a specific modifier state.
type KeyBarLabels [12]string

// KeyBar implements the bottom row of function key hints.
type KeyBar struct {
	ScreenObject
	Normal KeyBarLabels
	Shift  KeyBarLabels
	Ctrl   KeyBarLabels
	Alt    KeyBarLabels

	shiftState bool
	ctrlState  bool
	altState   bool
}

func NewKeyBar() *KeyBar {
	kb := &KeyBar{}
	return kb
}

func (kb *KeyBar) SetModifiers(shift, ctrl, alt bool) {
	if kb.shiftState != shift || kb.ctrlState != ctrl || kb.altState != alt {
		kb.shiftState = shift
		kb.ctrlState = ctrl
		kb.altState = alt
	}
}

func (kb *KeyBar) Show(scr *ScreenBuf) {
	kb.ScreenObject.Show(scr)
	kb.DisplayObject(scr)
}

func (kb *KeyBar) DisplayObject(scr *ScreenBuf) {
	if !kb.IsVisible() { return }

	labels := kb.Normal
	if kb.shiftState {
		labels = kb.Shift
	} else if kb.ctrlState {
		labels = kb.Ctrl
	} else if kb.altState {
		labels = kb.Alt
	}

	width := kb.X2 - kb.X1 + 1
	slotWidth := width / 12
	if slotWidth < 3 { slotWidth = 3 }

	numAttr := Palette[ColKeyBarNum]
	textAttr := Palette[ColKeyBarText]

	for i := 0; i < 12; i++ {
		x := kb.X1 + (i * slotWidth)
		if x > kb.X2 { break }

		// 1. Draw number
		numStr := fmt.Sprintf("%d", i+1)
		scr.Write(x, kb.Y1, StringToCharInfo(numStr, numAttr))

		// 2. Draw label
		label := labels[i]
		vLabelWidth := slotWidth - runewidth.StringWidth(numStr)
		if vLabelWidth > 0 {
			label = runewidth.Truncate(label, vLabelWidth, "")
			// Padding
			for runewidth.StringWidth(label) < vLabelWidth {
				label += " "
			}
			scr.Write(x+runewidth.StringWidth(numStr), kb.Y1, StringToCharInfo(label, textAttr))
		}
	}
}
