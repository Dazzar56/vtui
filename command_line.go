package vtui

import (
	"github.com/unxed/vtinput"
)

// CommandLine is a simplified Edit control used for shell input.
type CommandLine struct {
	ScreenObject
	Edit   *Edit
	Prompt string
}

func NewCommandLine(prompt string) *CommandLine {
	cl := &CommandLine{
		Prompt: prompt,
		Edit:   NewEdit(0, 0, 10, ""),
	}
	cl.Edit.owner = &cl.ScreenObject
	cl.canFocus = true
	return cl
}

func (cl *CommandLine) SetPosition(x1, y1, x2, y2 int) {
	cl.ScreenObject.SetPosition(x1, y1, x2, y2)
	// Edit starts after the prompt
	promptLen := len(cl.Prompt)
	cl.Edit.SetPosition(x1+promptLen, y1, x2, y2)
}

func (cl *CommandLine) SetFocus(f bool) {
	cl.focused = f
	cl.Edit.SetFocus(f)
}

func (cl *CommandLine) Show(scr *ScreenBuf) {
	cl.ScreenObject.Show(scr)
	cl.DisplayObject(scr)
}

func (cl *CommandLine) DisplayObject(scr *ScreenBuf) {
	if !cl.IsVisible() { return }

	// 1. Draw Prompt
	scr.Write(cl.X1, cl.Y1, StringToCharInfo(cl.Prompt, Palette[ColCommandLinePrompt]))

	// 2. Draw Edit (input field)
	cl.Edit.Show(scr)
}

func (cl *CommandLine) ProcessKey(e *vtinput.InputEvent) bool {
	return cl.Edit.ProcessKey(e)
}

func (cl *CommandLine) ProcessMouse(e *vtinput.InputEvent) bool {
	return cl.Edit.ProcessMouse(e)
}
