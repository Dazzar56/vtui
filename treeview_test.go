package vtui

import (
	"testing"

	"github.com/unxed/vtinput"
)

func createTestTree() *TreeNode {
	root := &TreeNode{Text: "Root", Expanded: true}

	child1 := &TreeNode{Text: "Child 1", Expanded: false}
	child1.AddChild(&TreeNode{Text: "Leaf 1.1"})
	child1.AddChild(&TreeNode{Text: "Leaf 1.2"})

	child2 := &TreeNode{Text: "Child 2", Expanded: true}
	child2.AddChild(&TreeNode{Text: "Leaf 2.1"})

	root.AddChild(child1)
	root.AddChild(child2)
	return root
}

func TestTreeView_Flattening(t *testing.T) {
	tree := NewTreeView(0, 0, 20, 10, createTestTree())

	// Root is expanded, Child 1 is collapsed, Child 2 is expanded
	// Expected flat nodes:
	// 0: Root
	// 1: Child 1
	// 2: Child 2
	// 3: Leaf 2.1

	if len(tree.flatNodes) != 4 {
		t.Fatalf("Expected 4 flat nodes, got %d", len(tree.flatNodes))
	}
	if tree.flatNodes[1].node.Text != "Child 1" {
		t.Errorf("Expected node 1 to be Child 1, got %q", tree.flatNodes[1].node.Text)
	}

	// Expand Child 1
	tree.flatNodes[1].node.Expanded = true
	tree.Flatten()

	if len(tree.flatNodes) != 6 {
		t.Errorf("Expected 6 flat nodes after expanding, got %d", len(tree.flatNodes))
	}
	if tree.flatNodes[2].node.Text != "Leaf 1.1" {
		t.Errorf("Expected node 2 to be Leaf 1.1, got %q", tree.flatNodes[2].node.Text)
	}
}

func TestTreeView_NavigationKeys(t *testing.T) {
	tree := NewTreeView(0, 0, 20, 10, createTestTree())

	// 1. Initial Selection
	if tree.SelectPos != 0 {
		t.Error("Initial select pos should be 0")
	}

	// 2. Down
	tree.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_DOWN})
	if tree.SelectPos != 1 {
		t.Error("Down arrow should move to index 1 (Child 1)")
	}

	// 3. Right on collapsed node (expands it)
	tree.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_RIGHT})
	if !tree.flatNodes[1].node.Expanded {
		t.Error("Right arrow should expand collapsed node")
	}
	if tree.SelectPos != 1 {
		t.Error("Right arrow expanding should not move cursor")
	}

	// 4. Right on expanded node (moves to first child)
	tree.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_RIGHT})
	if tree.SelectPos != 2 {
		t.Errorf("Right arrow on expanded should move to first child, got %d", tree.SelectPos)
	}
	if tree.flatNodes[2].node.Text != "Leaf 1.1" {
		t.Errorf("Should be on Leaf 1.1, got %q", tree.flatNodes[2].node.Text)
	}

	// 5. Left on child node (jumps to parent)
	tree.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_LEFT})
	if tree.SelectPos != 1 {
		t.Errorf("Left arrow on child should jump to parent, got %d", tree.SelectPos)
	}

	// 6. Left on expanded parent (collapses it)
	tree.ProcessKey(&vtinput.InputEvent{Type: vtinput.KeyEventType, KeyDown: true, VirtualKeyCode: vtinput.VK_LEFT})
	if tree.flatNodes[1].node.Expanded {
		t.Error("Left arrow on expanded parent should collapse it")
	}
}

func TestTreeView_MouseClick(t *testing.T) {
	tree := NewTreeView(0, 0, 20, 10, createTestTree())

	// Click on row 1 (Child 1) to select it
	tree.ProcessMouse(&vtinput.InputEvent{
		Type: vtinput.MouseEventType, KeyDown: true,
		ButtonState: vtinput.FromLeft1stButtonPressed, MouseX: 10, MouseY: 1,
	})

	if tree.SelectPos != 1 {
		t.Errorf("Expected selection 1 after mouse click, got %d", tree.SelectPos)
	}

	// Click exactly on the [+] marker of Child 1 to expand it
	// Prefix width: 2 spaces (for level 0) + "├─" = 4 chars
	// So marker starts at X=4. Marker is "[+] ". Click at X=5 ('+').
	tree.ProcessMouse(&vtinput.InputEvent{
		Type: vtinput.MouseEventType, KeyDown: true,
		ButtonState: vtinput.FromLeft1stButtonPressed, MouseX: 5, MouseY: 1,
	})

	if !tree.flatNodes[1].node.Expanded {
		t.Error("Clicking on [+] marker should expand the node")
	}
}

func TestTreeView_Rendering(t *testing.T) {
	SetDefaultPalette()
	scr := NewScreenBuf()
	scr.AllocBuf(30, 10)

	tree := NewTreeView(0, 0, 30, 10, createTestTree())
	tree.SetFocus(true)
	tree.Show(scr)

	// Row 0: Root "[-] Root"
	checkCell(t, scr, 0, 0, '[', Palette[ColDialogHighlightSelectedButton])
	checkCell(t, scr, 1, 0, '-', Palette[ColDialogHighlightSelectedButton])

	// Row 1: Child 1 "  ├─[+] Child 1"
	// ├ is boxSymbols[6]
	checkCell(t, scr, 0, 1, ' ', Palette[ColTableBox])
	checkCell(t, scr, 1, 1, ' ', Palette[ColTableBox])
	checkCell(t, scr, 2, 1, uint64(boxSymbols[6]), Palette[ColTableBox])
	checkCell(t, scr, 3, 1, uint64(boxSymbols[bsH]), Palette[ColTableBox])
	checkCell(t, scr, 4, 1, '[', Palette[ColTableText])
	checkCell(t, scr, 5, 1, '+', Palette[ColTableText])

	// Row 2: Child 2 "  └─[-] Child 2"
	// └ is boxSymbols[4]
	checkCell(t, scr, 0, 2, ' ', Palette[ColTableBox])
	checkCell(t, scr, 1, 2, ' ', Palette[ColTableBox])
	checkCell(t, scr, 2, 2, uint64(boxSymbols[4]), Palette[ColTableBox])
	checkCell(t, scr, 3, 2, uint64(boxSymbols[bsH]), Palette[ColTableBox])
	checkCell(t, scr, 4, 2, '[', Palette[ColTableText])
	checkCell(t, scr, 5, 2, '-', Palette[ColTableText])
}