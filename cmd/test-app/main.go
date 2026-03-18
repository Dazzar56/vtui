package main

import (
	"fmt"
	"os"

	"github.com/unxed/vtinput"
	"github.com/unxed/vtui"
	"golang.org/x/term"
)

func main() {
	// 1. Enable extended terminal mode
	restore, err := vtinput.Enable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error enabling raw mode: %v\n", err)
		return
	}
	defer restore()

	// Hide cursor and restore it on exit
	fmt.Print("\x1b[?25l")
	defer fmt.Print("\x1b[?25h")

	// 2. Get terminal size and create screen buffer
	width, height, _ := term.GetSize(int(os.Stdin.Fd()))
	scr := vtui.NewScreenBuf()
	scr.AllocBuf(width, height)

	// 3. Initialize FrameManager
	vtui.FrameManager.Init(scr)

	// Create Desktop background layer
	desktop := vtui.NewDesktop()
	vtui.FrameManager.Push(desktop)

	// 4. Create the unified, comprehensive demonstration dialog
	dlgWidth, dlgHeight := 64, 20
	x1 := (width - dlgWidth) / 2
	y1 := (height - dlgHeight) / 2
	dlg := vtui.NewDialog(x1, y1, x1+dlgWidth-1, y1+dlgHeight-1, " vtui Comprehensive Demo ")
	dlg.SetHelp("MainDialogTopic")

	// --- Left Column ---

	// Radio buttons
	dlg.AddItem(vtui.NewLabel(x1+2, y1+2, "Select &mode:", nil))
	rb1 := vtui.NewRadioButton(x1+4, y1+3, "&Fast and Dangerous")
	rb1.Selected = true
	dlg.AddItem(rb1)
	dlg.AddItem(vtui.NewRadioButton(x1+4, y1+4, "Slow and &Stable"))
	dlg.AddItem(vtui.NewRadioButton(x1+4, y1+5, "Mental &Health Mode"))

	// ComboBox
	combo := vtui.NewComboBox(x1+13, y1+8, 16, []string{"UTF-8", "CP866 (OEM)", "Windows-1251", "KOI8-R"})
	combo.Edit.SetText("UTF-8")
	dlg.AddItem(vtui.NewLabel(x1+2, y1+8, "&Encoding:", combo))
	dlg.AddItem(combo)

	// Password
	pass := vtui.NewEdit(x1+13, y1+10, 16, "")
	pass.PasswordMode = true
	dlg.AddItem(vtui.NewLabel(x1+2, y1+10, "&Password:", pass))
	dlg.AddItem(pass)


	// --- Right Column ---

	// VText separator
	dlg.AddItem(vtui.NewVText(x1+30, y1+2, "│CORE│", vtui.Palette[vtui.ColDialogText]))

	// Checkboxes
	dlg.AddItem(vtui.NewLabel(x1+34, y1+2, "S&ettings:", nil))
	dlg.AddItem(vtui.NewCheckbox(x1+36, y1+3, "Enable &AI", false))
	dlg.AddItem(vtui.NewCheckbox(x1+36, y1+4, "A&uto-update", true))
	dlg.AddItem(vtui.NewCheckbox(x1+36, y1+5, "F&orce Legacy (3-state)", true))

	// VMenu
	menu := vtui.NewVMenu(" Operations ")
	menu.SetHelp("MenuOperationsTopic")
	menu.SetPosition(x1+34, y1+8, x1+58, y1+10) // Height of 3 lines
	menu.AddItem("&Copy File")
	menu.AddItem("&Move File")
	menu.AddSeparator()
	menu.AddItem("&Delete File")
	dlg.AddItem(menu)

	// ListBox (from f4 demo)
	recentFiles := []string{"config.go", "main.go", "utils.go", "README.md", "LICENSE", "go.mod"}
	lb := vtui.NewListBox(x1+34, y1+12, 24, 2, recentFiles) // Height of 2 lines
	lb.ColorTextIdx = vtui.ColDialogEdit
	lb.ColorSelectedTextIdx = vtui.ColDialogEditSelected
	dlg.AddItem(vtui.NewLabel(x1+34, y1+11, "&Recently used:", lb))
	dlg.AddItem(lb)


	// --- Bottom Buttons ---

	btnOk := vtui.NewButton(x1+dlgWidth/2-18, y1+17, "&Ok")
	btnOk.OnClick = func() { dlg.SetExitCode(0); desktop.SetExitCode(0) }

	btnCancel := vtui.NewButton(x1+dlgWidth/2-4, y1+17, "&Cancel")
	btnCancel.OnClick = func() { dlg.SetExitCode(-1); desktop.SetExitCode(-1) }

	btnMsg := vtui.NewButton(x1+dlgWidth/2+10, y1+17, "Show &Msg")
	btnMsg.OnClick = func() {
		vtui.ShowMessage(" MessageBox Demo ", "This is a dynamic message box.\nIt supports Unicode: Привет! 🚀", []string{"&Nice!", "&Whatever"})
	}

	dlg.AddItem(btnOk)
	dlg.AddItem(btnCancel)
	dlg.AddItem(btnMsg)

	// 5. Add dialog to FrameManager stack and start event loop
	vtui.FrameManager.Push(dlg)
	vtui.FrameManager.Run()
}
