package vtui

// isBoxDrawRune reports whether a rune is worth offering to drawBoxGlyph /
// drawCustomChar. A cheap range test in front of the shape switch, which
// would otherwise run for every letter on screen. Being generous costs only
// a switch that returns false; being too narrow would silently send a frame
// character back to the font.
//
// No build tag: gogpu_renderer.go compiles on a wider platform set than
// gui_boxdraw.go (which holds the actual shape rasterizers).
func isBoxDrawRune(r rune) bool {
	return (r >= 0x2500 && r <= 0x259F) || (r >= 0x2190 && r <= 0x2195) ||
		r == 0x25B2 || r == 0x25BC
}
