package vtui

import (
	"os"
	"github.com/unxed/vtinput"
)

const (
	seqAltScreenOn      = "\x1b[?1049h"
	seqAltScreenOff     = "\x1b[?1049l"
	seqBlinkingUnderline = "\x1b[3 q"
	seqDefaultCursor     = "\x1b[0 q"
	seqResetPalette      = "\x1b]104\x07"
	seqResetAttributes   = "\x1b[0m"
)

// PrepareTerminal puts the terminal into raw mode and sets up the
// environment (AltScreen, cursor). Returns a restore function.
func PrepareTerminal() (func(), error) {
	// 1. Enable input protocols via vtinput
	restoreInput, err := vtinput.Enable()
	if err != nil {
		return nil, err
	}

	// 2. Setup environment
	os.Stdout.WriteString(seqAltScreenOn + seqBlinkingUnderline)

	restore := func() {
		// Cleanup in reverse order
		os.Stdout.WriteString(seqAltScreenOff + seqDefaultCursor + seqResetPalette + seqResetAttributes)
		os.Stdout.Sync()
		restoreInput()
	}

	return restore, nil
}