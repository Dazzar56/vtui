package main

import (
	"fmt"
	"os"

	"github.com/unxed/vtinput"
	"github.com/unxed/vtui"
	"golang.org/x/term"
)

func main() {
	// Enable advanced terminal mode
	restore, err := vtinput.Enable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error enabling raw mode: %v\n", err)
		return
	}
	defer restore()

	// Hide cursor during execution and restore it on exit
	fmt.Print("\x1b[?25l")
	defer fmt.Print("\x1b[?25h")

	// Get initial terminal size and create the screen buffer
	width, height, _ := term.GetSize(int(os.Stdin.Fd()))
	scr := vtui.NewScreenBuf()
	scr.AllocBuf(width, height)

	// --- Initialize FrameManager ---
	vtui.FrameManager.Init(scr)

	// Create and push the root Desktop frame
	desktop := vtui.NewDesktop()
	vtui.FrameManager.Push(desktop)

	// --- Create a Dialog to show ---
	dlgWidth, dlgHeight := 40, 14
	x1 := (width - dlgWidth) / 2
	y1 := (height - dlgHeight) / 2
	dlg := vtui.NewDialog(x1, y1, x1+dlgWidth-1, y1+dlgHeight-1, " Action Dialog ")

	// Create and add items to the dialog
	label := vtui.NewText(x1+5, y1+1, "Enter task name:", vtui.SetRGBFore(0, 0xFFFFFF))
	edit := vtui.NewEdit(x1+5, y1+2, 20, "f4 project")
	menu := vtui.NewVMenu(" Select Action ")
	menu.SetPosition(x1+5, y1+5, x1+30, y1+10)
	menu.AddItem("Copy File")
	menu.AddItem("Move File")
	menu.AddSeparator()
	menu.AddItem("Delete File")
	btnOk := vtui.NewButton(x1+5, y1+12, "Ok")
	btnCancel := vtui.NewButton(x1+15, y1+12, "Cancel")

	// Set button actions to close the dialog
	btnCancel.OnClick = func() {
		dlg.SetExitCode(-1)
		desktop.SetExitCode(-1)
	}
	btnOk.OnClick = func() {
		dlg.SetExitCode(0)
		desktop.SetExitCode(0)
	}

	dlg.AddItem(label)
	dlg.AddItem(edit)
	dlg.AddItem(menu)
	dlg.AddItem(btnOk)
	dlg.AddItem(btnCancel)

	// Push the dialog onto the frame stack
	vtui.FrameManager.Push(dlg)

	// Start the main application loop
	vtui.FrameManager.Run()
}