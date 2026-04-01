package vtui

import (
	"github.com/unxed/vtinput"
	"github.com/mattn/go-runewidth"
)

// ListBox represents a list of strings for selection within a dialog.
type ListBox struct {
	ScrollView
	Items    []string

	ColorTextIdx             int
	ColorSelectedTextIdx     int
	ColorItemSelectTextIdx   int
	ColorItemSelectCursorIdx int
	ColorTitleIdx            int
	ColorBoxIdx              int
	MultiSelect              bool
	SelectedMap              map[int]bool
}

func NewListBox(x, y, w, h int, items []string) *ListBox {
	lb := &ListBox{
		Items:                    items,
		SelectedMap:              make(map[int]bool),
		ColorTextIdx:             ColTableText,
		ColorSelectedTextIdx:     ColTableSelectedText,
		ColorItemSelectTextIdx:   ColTableText,
		ColorItemSelectCursorIdx: ColTableSelectedText,
		ColorTitleIdx:            ColTableColumnTitle,
		ColorBoxIdx:              ColTableBox,
	}
	lb.ItemCount = len(items)
	lb.ViewHeight = h
	if lb.ItemCount == 0 {
		lb.SelectPos = 0
	}
	lb.canFocus = true
	lb.ShowScrollBar = true
	lb.InitScrollBar(lb)
	lb.SetPosition(x, y, x+w-1, y+h-1)
	return lb
}

func (lb *ListBox) GetSelectedIndices() []int {
	var res []int
	for i := range lb.Items {
		if lb.SelectedMap[i] { res = append(res, i) }
	}
	return res
}

func (lb *ListBox) Show(scr *ScreenBuf) {
	lb.ScreenObject.Show(scr)
	lb.DisplayObject(scr)
}

func (lb *ListBox) DisplayObject(scr *ScreenBuf) {
	if !lb.IsVisible() { return }

	width := lb.GetContentWidth()
	height := lb.Y2 - lb.Y1 + 1

	// 1. Elements rendering
	for i := 0; i < height; i++ {
		idx := lb.TopPos + i
		currY := lb.Y1 + i

		attr := lb.GetStateAttr(lb.ColorTextIdx, lb.ColorSelectedTextIdx)
		isSelected := lb.SelectedMap[idx]

		if isSelected {
			attr = lb.GetStateAttr(ColDialogHighlightText, ColDialogHighlightSelectedButton)
		} else if idx == lb.SelectPos && !lb.IsFocused() {
			attr = lb.GetStateAttr(lb.ColorTextIdx, lb.ColorTextIdx)
		}

		if idx < len(lb.Items) {
			text := runewidth.Truncate(lb.Items[idx], width, "")
			vLen := runewidth.StringWidth(text)

			scr.Write(lb.X1, currY, StringToCharInfo(text, attr))
			if vLen < width {
				scr.FillRect(lb.X1+vLen, currY, lb.X1+width-1, currY, ' ', attr)
			}
		} else {
			scr.FillRect(lb.X1, currY, lb.X1+width-1, currY, ' ', lb.GetStateAttr(lb.ColorTextIdx, lb.ColorTextIdx))
		}
	}

	// 2. Scrollbar
	lb.DrawScrollBar(scr)
}

func (lb *ListBox) ProcessKey(e *vtinput.InputEvent) bool {
	if !e.KeyDown || lb.IsDisabled() { return false }

	switch e.VirtualKeyCode {
	case vtinput.VK_SPACE, vtinput.VK_INSERT:
		if lb.MultiSelect {
			lb.SelectedMap[lb.SelectPos] = !lb.SelectedMap[lb.SelectPos]
			if e.VirtualKeyCode == vtinput.VK_INSERT { lb.MoveRelative(1) }
			return true
		}
	}

	return lb.HandleKey(e)
}

func (lb *ListBox) ProcessMouse(e *vtinput.InputEvent) bool {
	if lb.IsDisabled() { return false }
	return lb.HandleMouse(e)
}