package vtui

import (
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/unxed/vtinput"
)

// TreeNode represents a single item in the TreeView.
type TreeNode struct {
	Text     string
	Children []*TreeNode
	Expanded bool
	Data     any

	parent *TreeNode
}

// AddChild adds a child node and sets its parent.
func (n *TreeNode) AddChild(child *TreeNode) {
	child.parent = n
	n.Children = append(n.Children, child)
}

// Parent returns the parent node, or nil if this is the root.
func (n *TreeNode) Parent() *TreeNode {
	return n.parent
}

type flatNode struct {
	node   *TreeNode
	level  int
	isLast []bool // Indicates if the ancestor at each level is the last child
}

// TreeView displays hierarchical data in an expandable tree structure.
type TreeView struct {
	ScreenObject
	Root                 *TreeNode
	ShowRoot             bool
	SelectPos            int
	TopPos               int
	ColorTextIdx         int
	ColorSelectedTextIdx int
	ColorTreeLineIdx     int
	ColorBoxIdx          int // For scrollbar

	OnSelect func(node *TreeNode)
	OnAction func(node *TreeNode)

	flatNodes []flatNode
}

func NewTreeView(x, y, w, h int, root *TreeNode) *TreeView {
	tv := &TreeView{
		Root:                 root,
		ShowRoot:             true,
		ColorTextIdx:         ColTableText,
		ColorSelectedTextIdx: ColTableSelectedText,
		ColorTreeLineIdx:     ColTableBox,
		ColorBoxIdx:          ColTableBox,
	}
	tv.canFocus = true
	tv.SetPosition(x, y, x+w-1, y+h-1)
	tv.Flatten()
	return tv
}

// Flatten rebuilds the internal flat list of visible nodes based on expansion state.
func (t *TreeView) Flatten() {
	t.flatNodes = nil
	if t.Root == nil {
		return
	}

	var traverse func(node *TreeNode, level int, isLast []bool)
	traverse = func(node *TreeNode, level int, isLast []bool) {
		t.flatNodes = append(t.flatNodes, flatNode{
			node:   node,
			level:  level,
			isLast: append([]bool(nil), isLast...),
		})
		if node.Expanded {
			for i, child := range node.Children {
				childIsLast := i == len(node.Children)-1
				traverse(child, level+1, append(isLast, childIsLast))
			}
		}
	}

	if t.ShowRoot {
		traverse(t.Root, 0, []bool{true})
	} else {
		for i, child := range t.Root.Children {
			childIsLast := i == len(t.Root.Children)-1
			traverse(child, 0, []bool{childIsLast})
		}
	}

	// Ensure selection stays within bounds after a collapse
	if t.SelectPos >= len(t.flatNodes) {
		t.SelectPos = len(t.flatNodes) - 1
	}
	if t.SelectPos < 0 {
		t.SelectPos = 0
	}
}

func (t *TreeView) Show(scr *ScreenBuf) {
	t.ScreenObject.Show(scr)
	t.DisplayObject(scr)
}

func (t *TreeView) DisplayObject(scr *ScreenBuf) {
	if !t.IsVisible() {
		return
	}

	width := t.X2 - t.X1 + 1
	height := t.Y2 - t.Y1 + 1

	colText := Palette[t.ColorTextIdx]
	colSel := Palette[t.ColorSelectedTextIdx]
	colLine := Palette[t.ColorTreeLineIdx]

	for i := 0; i < height; i++ {
		idx := t.TopPos + i
		currY := t.Y1 + i

		if idx < len(t.flatNodes) {
			fn := t.flatNodes[idx]

			attr := colText
			if idx == t.SelectPos {
				if t.IsFocused() {
					attr = Palette[ColDialogHighlightSelectedButton]
				} else {
					attr = colSel
				}
			}
			if t.IsDisabled() {
				attr = DimColor(attr)
			}

			// Build tree lines
			var sb strings.Builder
			for lvl := 0; lvl < fn.level; lvl++ {
				if fn.isLast[lvl] {
					sb.WriteString("  ")
				} else {
					sb.WriteRune(boxSymbols[bsV]) // '│'
					sb.WriteRune(' ')
				}
			}

			if fn.node != t.Root || !t.ShowRoot {
				if fn.isLast[fn.level] {
					sb.WriteRune(boxSymbols[4]) // '└'
					sb.WriteRune(boxSymbols[bsH]) // '─'
				} else {
					sb.WriteRune(boxSymbols[6]) // '├'
					sb.WriteRune(boxSymbols[bsH]) // '─'
				}
			}

			marker := "    "
			if len(fn.node.Children) > 0 {
				if fn.node.Expanded {
					marker = "[-] "
				} else {
					marker = "[+] "
				}
			}

			prefixStr := sb.String()
			textStr := marker + fn.node.Text

			// Clip string if too long
			prefixWidth := runewidth.StringWidth(prefixStr)
			textWidth := runewidth.StringWidth(textStr)

			if prefixWidth + textWidth > width {
				textStr = runewidth.Truncate(textStr, width - prefixWidth, "")
				textWidth = runewidth.StringWidth(textStr)
			}

			scr.Write(t.X1, currY, StringToCharInfo(prefixStr, colLine))
			scr.Write(t.X1+prefixWidth, currY, StringToCharInfo(textStr, attr))

			// Fill remaining
			fillWidth := width - prefixWidth - textWidth
			if fillWidth > 0 {
				scr.FillRect(t.X1+prefixWidth+textWidth, currY, t.X2, currY, ' ', attr)
			}
		} else {
			scr.FillRect(t.X1, currY, t.X2, currY, ' ', colText)
		}
	}

	// Scrollbar
	if len(t.flatNodes) > height {
		DrawScrollBar(scr, t.X2, t.Y1, height, t.TopPos, len(t.flatNodes), Palette[t.ColorBoxIdx])
	}
}

func (t *TreeView) ProcessKey(e *vtinput.InputEvent) bool {
	if !e.KeyDown || t.IsDisabled() || len(t.flatNodes) == 0 {
		return false
	}

	height := t.Y2 - t.Y1 + 1
	oldPos := t.SelectPos

	fn := t.flatNodes[t.SelectPos]

	switch e.VirtualKeyCode {
	case vtinput.VK_UP:
		if t.SelectPos > 0 { t.SelectPos-- }
	case vtinput.VK_DOWN:
		if t.SelectPos < len(t.flatNodes)-1 { t.SelectPos++ }
	case vtinput.VK_LEFT:
		if fn.node.Expanded && len(fn.node.Children) > 0 {
			fn.node.Expanded = false
			t.Flatten()
		} else if fn.node.parent != nil {
			// Jump to parent
			for i := t.SelectPos - 1; i >= 0; i-- {
				if t.flatNodes[i].node == fn.node.parent {
					t.SelectPos = i
					break
				}
			}
		}
	case vtinput.VK_RIGHT:
		if len(fn.node.Children) > 0 {
			if !fn.node.Expanded {
				fn.node.Expanded = true
				t.Flatten()
			} else if t.SelectPos < len(t.flatNodes)-1 {
				// Jump to first child
				t.SelectPos++
			}
		}
	case vtinput.VK_RETURN, vtinput.VK_SPACE:
		if len(fn.node.Children) > 0 {
			fn.node.Expanded = !fn.node.Expanded
			t.Flatten()
		} else if t.OnAction != nil {
			t.OnAction(fn.node)
		}
		return true
	case vtinput.VK_PRIOR: // PgUp
		t.SelectPos -= height
		if t.SelectPos < 0 { t.SelectPos = 0 }
	case vtinput.VK_NEXT: // PgDn
		t.SelectPos += height
		if t.SelectPos >= len(t.flatNodes) { t.SelectPos = len(t.flatNodes) - 1 }
	case vtinput.VK_HOME:
		t.SelectPos = 0
	case vtinput.VK_END:
		t.SelectPos = len(t.flatNodes) - 1
	default:
		return false
	}

	if t.SelectPos != oldPos {
		t.EnsureVisible()
		if t.OnSelect != nil {
			t.OnSelect(t.flatNodes[t.SelectPos].node)
		}
		return true
	}

	return true // We handled an arrow key even if it didn't move
}

func (t *TreeView) EnsureVisible() {
	height := t.Y2 - t.Y1 + 1
	if height <= 0 { return }

	if t.SelectPos < t.TopPos {
		t.TopPos = t.SelectPos
	} else if t.SelectPos >= t.TopPos+height {
		t.TopPos = t.SelectPos - height + 1
	}
}

func (t *TreeView) ProcessMouse(e *vtinput.InputEvent) bool {
	if t.IsDisabled() || e.Type != vtinput.MouseEventType || len(t.flatNodes) == 0 {
		return false
	}

	mx, my := int(e.MouseX), int(e.MouseY)
	height := t.Y2 - t.Y1 + 1

	if e.WheelDirection > 0 {
		if t.TopPos > 0 { t.TopPos-- }
		return true
	} else if e.WheelDirection < 0 {
		if t.TopPos < len(t.flatNodes)-height { t.TopPos++ }
		return true
	}

	if e.ButtonState == vtinput.FromLeft1stButtonPressed && e.KeyDown {
		if mx >= t.X1 && mx <= t.X2 && my >= t.Y1 && my <= t.Y2 {
			if len(t.flatNodes) > height && mx == t.X2 {
				if my == t.Y1 {
					t.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_UP})
				} else if my == t.Y2 {
					t.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_DOWN})
				} else {
					if my < t.Y1+height/2 {
						t.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_PRIOR})
					} else {
						t.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_NEXT})
					}
				}
				return true
			}

			clickIdx := t.TopPos + (my - t.Y1)
			if clickIdx >= 0 && clickIdx < len(t.flatNodes) {
				t.SelectPos = clickIdx
				t.EnsureVisible()
				if t.OnSelect != nil {
					t.OnSelect(t.flatNodes[clickIdx].node)
				}

				// Check if click is on [+] or [-] marker
				// Marker usually starts after prefixWidth
				fn := t.flatNodes[clickIdx]
				prefixWidth := fn.level * 2
				if fn.node != t.Root || !t.ShowRoot {
					prefixWidth += 2 // '├─'
				}

				if mx >= t.X1+prefixWidth && mx < t.X1+prefixWidth+3 {
					// Clicked inside the marker
					if len(fn.node.Children) > 0 {
						fn.node.Expanded = !fn.node.Expanded
						t.Flatten()
					}
				}
				return true
			}
		}
	}
	return false
}