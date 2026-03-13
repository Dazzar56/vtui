package vtui

import (
	"encoding/base64"
	"os"
)

// SetClipboard copies text to the system clipboard via ANSI OSC 52.
func SetClipboard(text string) {
	b64 := base64.StdEncoding.EncodeToString([]byte(text))
	// ANSI OSC 52: \x1b]52;c;<base64>\x07
	os.Stdout.WriteString("\x1b]52;c;" + b64 + "\x07")
}