package vtui

import (
	"unicode"
	"github.com/unxed/vtinput"
)

// RadioGroup is a cluster of radio buttons where only one can be selected.
type RadioGroup struct {
	ScreenObject
	Items     []string
	Selected  int
	OnChange  func(int)
	Columns   int
	colWidths []int
}


func NewRadioGroup(x, y, cols int, items []string) *RadioGroup {
	rg := &RadioGroup{Items: items}
	rg.canFocus = true
	if cols < 1 { cols = 1 }
	rg.Columns = cols

	rows := (len(items) + cols - 1) / cols
	rg.colWidths = calcGridColWidths(cols, items)

	totalW := 0
	for _, w := range rg.colWidths { totalW += w }

	rg.SetPosition(x, y, x+totalW-1, y+rows-1)
	return rg
}

func (rg *RadioGroup) Show(scr *ScreenBuf) {
	rg.ScreenObject.Show(scr)
	rg.DisplayObject(scr)
}

func (rg *RadioGroup) DisplayObject(scr *ScreenBuf) {
	if !rg.IsVisible() { return }

	attr, highAttr := rg.GetStateAttrs(ColDialogText, ColDialogSelectedButton, ColDialogHighlightText, ColDialogHighlightSelectedButton)

	p := NewPainter(scr)
	for i, itm := range rg.Items {
		prefix := "( ) "
		if i == rg.Selected { prefix = "(•) " }

		row := i / rg.Columns
		col := i % rg.Columns
		cx := rg.X1
		for c := 0; c < col; c++ { cx += rg.colWidths[c] }

		p.DrawStringHighlighted(cx, rg.Y1+row, prefix+itm, attr, highAttr)
	}
}

func (rg *RadioGroup) GetData() any {
	return rg.Selected
}

func (rg *RadioGroup) SetData(val any) {
	if i, ok := val.(int); ok {
		rg.Selected = i
	}
}

func (rg *RadioGroup) ProcessKey(e *vtinput.InputEvent) bool {
	if !e.KeyDown {
		return false
	}

	if rg.IsDisabled() { return false }

	newIdx, moved := gridNav(rg.Selected, len(rg.Items), rg.Columns, e.VirtualKeyCode)
	if moved {
		rg.Selected = newIdx
		var onClick func()
		if rg.OnChange != nil {
			onClick = func() { rg.OnChange(rg.Selected) }
		}
		rg.FireAction(onClick, rg.Selected)
		return true
	}

	if handleGridBoundaryNav(e.VirtualKeyCode, rg.Selected, len(rg.Items)) {
		return true
	}

	switch e.VirtualKeyCode {
	case vtinput.VK_SPACE, vtinput.VK_RETURN:
		return false // Allow dialog to catch it if needed
	}

	if e.Char != 0 {
		hkChar := unicode.ToLower(e.Char)
		{
			for i, itm := range rg.Items {
				if ExtractHotkey(itm) == hkChar {
					rg.Selected = i
				var onClick func()
				if rg.OnChange != nil {
					onClick = func() { rg.OnChange(rg.Selected) }
				}
				rg.FireAction(onClick, rg.Selected)
					return true
				}
			}
		}
	}

	return false
}

func (rg *RadioGroup) ProcessMouse(e *vtinput.InputEvent) bool {
	if rg.IsDisabled() { return false }
	if e.Type == vtinput.MouseEventType && e.ButtonState == vtinput.FromLeft1stButtonPressed && e.KeyDown {
		mx, my := int(e.MouseX), int(e.MouseY)
		if rg.HitTest(mx, my) {
			idx := getGridIndexFromMouse(rg.X1, rg.Y1, mx, my, rg.Columns, rg.colWidths)
			if idx >= 0 && idx < len(rg.Items) {
				if rg.Selected != idx {
					rg.Selected = idx
					var onClick func()
					if rg.OnChange != nil {
						onClick = func() { rg.OnChange(rg.Selected) }
					}
					rg.FireAction(onClick, rg.Selected)
				}
				return true
			}
		}
	}
	return false
}

