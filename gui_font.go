//go:build linux || openbsd || netbsd || dragonfly || darwin || freebsd || windows

package vtui

import (
	"os"
	"path/filepath"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
)

// loadBestFont attempts to find a suitable monospace TTF font on the system.
// If none is found, it falls back to a built-in bitmap font.
func loadBestFont(size float64, dpi float64) (font.Face, int, int) {
	// Paths for common Linux distributions and Windows
	fontPaths := []string{
		`C:\Windows\Fonts\consola.ttf`,
		`C:\Windows\Fonts\lucon.ttf`,
		`C:\Windows\Fonts\cour.ttf`, // Courier New
		"/usr/share/fonts/truetype/ubuntu/UbuntuMono-R.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf",
		"/usr/share/fonts/TTF/DejaVuSansMono.ttf",
		"/System/Library/Fonts/Supplemental/Courier New.ttf", // macOS path
	}

	for _, path := range fontPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			if !os.IsNotExist(err) {
				DebugLog("GUI_FONT: Error reading %s: %v", path, err)
			}
			continue
		}

		f, err := opentype.Parse(data)
		if err != nil {
			DebugLog("GUI_FONT: Error parsing %s: %v", path, err)
			continue
		}

		face, err := opentype.NewFace(f, &opentype.FaceOptions{
			Size:    size,
			DPI:     dpi,
			Hinting: font.HintingFull,
		})
		if err != nil {
			DebugLog("GUI_FONT: Error creating face for %s: %v", path, err)
			continue
		}

		// Calculate cell size from metrics
		metrics := face.Metrics()
		cellH := (metrics.Ascent + metrics.Descent).Ceil()

		// For monospaced fonts, advance of any character (e.g. 'A') is the cell width
		advance, _ := face.GlyphAdvance('A')
		cellW := advance.Ceil()

		DebugLog("GUI_FONT: Successfully loaded %s, metrics: %dx%d", filepath.Base(path), cellW, cellH)
		return face, cellW, cellH
	}

	// Fallback to basicfont if no TTF found
	DebugLog("GUI_FONT: CRITICAL - No TTF font found! Falling back to basicfont 7x13 (ASCII only!)")
	return basicfont.Face7x13, 7, 13
}
