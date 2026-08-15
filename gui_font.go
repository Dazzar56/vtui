//go:build linux || openbsd || netbsd || dragonfly || darwin || freebsd || windows || solaris || illumos

package vtui

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// fallbackFontPaths is consulted in order, so the entries are sorted by how
// much of Unicode they carry rather than by how likely they are to exist: a
// missing file costs one failed stat, whereas a narrow font that happens to
// be listed first would answer for glyphs a later, wider font renders better.
//
// The list is deliberately long. Distributions disagree about where Noto CJK
// lives, and a Windows install outside a CJK locale has none of the Japanese
// supplemental fonts, so a short list quietly degrades to no fallback at all.
var fallbackFontPaths = []string{
	// Linux — CJK
	"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/opentype/noto/NotoSansCJK-VF.otf.ttc",
	"/usr/share/fonts/noto-cjk/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/noto-cjk/NotoSansCJK-VF.otf.ttc",
	"/usr/share/fonts/google-noto-cjk/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/noto/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/opentype/source-han-sans/SourceHanSans-Regular.otc",
	"/usr/share/fonts/adobe-source-han-sans/SourceHanSans-Regular.otc",
	"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
	"/usr/share/fonts/truetype/wqy/wqy-microhei.ttc",
	"/usr/share/fonts/truetype/wqy/wqy-zenhei.ttc",
	"/usr/share/fonts/wqy-microhei/wqy-microhei.ttc",
	"/usr/share/fonts/truetype/arphic/uming.ttc",
	"/usr/share/fonts/truetype/arphic/ukai.ttc",
	// Linux — emoji and general symbol coverage
	"/usr/share/fonts/noto/NotoColorEmoji.ttf",
	"/usr/share/fonts/truetype/noto/NotoColorEmoji.ttf",
	"/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf",
	"/usr/share/fonts/TTF/DejaVuSans.ttf",
	"/usr/share/fonts/truetype/unifont/unifont.ttf",
	// Windows — CJK. Only the Yu Gothic and Microsoft families ship outside a
	// CJK locale; msgothic/msmincho/meiryo arrive with the Japanese
	// supplemental fonts and are frequently absent.
	`C:\Windows\Fonts\msyh.ttc`,
	`C:\Windows\Fonts\msjh.ttc`,
	`C:\Windows\Fonts\YuGothM.ttc`,
	`C:\Windows\Fonts\YuGothR.ttc`,
	`C:\Windows\Fonts\simsun.ttc`,
	`C:\Windows\Fonts\malgun.ttf`,
	`C:\Windows\Fonts\msgothic.ttc`,
	`C:\Windows\Fonts\msmincho.ttc`,
	`C:\Windows\Fonts\meiryo.ttc`,
	`C:\Windows\Fonts\mingliu.ttc`,
	`C:\Windows\Fonts\batang.ttc`,
	`C:\Windows\Fonts\gulim.ttc`,
	`C:\Windows\Fonts\arialuni.ttf`,
	// Windows — emoji and symbols
	`C:\Windows\Fonts\seguiemj.ttf`,
	`C:\Windows\Fonts\seguisym.ttf`,
	`C:\Windows\Fonts\segoeui.ttf`,
	// macOS
	"/System/Library/Fonts/PingFang.ttc",
	"/System/Library/Fonts/Hiragino Sans GB.ttc",
	"/System/Library/Fonts/AppleSDGothicNeo.ttc",
	"/System/Library/Fonts/STHeiti Light.ttc",
	"/System/Library/Fonts/Supplemental/Songti.ttc",
	"/System/Library/Fonts/Apple Color Emoji.ttc",
	"/Library/Fonts/Arial Unicode.ttf",
}

// runeCoverage is a bitmap of the runes a font has glyphs for, over planes 0
// and 1 — everything a terminal plausibly draws, emoji included. Runes above
// plane 1 are so rare that "maybe" (and a re-parse to find out) is the right
// answer for them. It is shared by the gogpu backend's fallbackFontChain and
// the guiFallbackChain below.
type runeCoverage struct {
	bits [0x20000 / 64]uint64
}

func (c *runeCoverage) maybeHas(r rune) bool {
	if r < 0 {
		return false
	}
	if r >= 0x20000 {
		return true
	}
	return c.bits[r>>6]&(1<<(uint(r)&63)) != 0
}

func parseFontBytes(data []byte) (*opentype.Font, error) {
	f, err := opentype.Parse(data)
	if err == nil {
		return f, nil
	}
	col, err2 := opentype.ParseCollection(data)
	if err2 == nil && col.NumFonts() > 0 {
		f, err3 := col.Font(0)
		if err3 == nil {
			return f, nil
		}
	}
	return nil, err
}

// guiFallbackChain is the x/image twin of the gogpu backend's
// fallbackFontChain: it opens a fallback font file the first time a rune
// actually needs it, and keeps in memory only the fonts that have supplied a
// glyph. The eager version parsed every fallback font on the machine at
// startup — on a Linux install with Noto CJK that is hundreds of megabytes
// held for glyphs that almost never render (the same bug measured at ~800 MB
// on macOS in the gogpu backend).
//
// A parsed font that lacks the requested rune is condensed into a 16 KB
// runeCoverage bitmap and dropped, so a rune nobody covers cannot make the
// chain retain everything, and each file is parsed at most twice over the
// process lifetime: once to learn its coverage, once more only if a covered
// rune ever shows up.
type guiFallbackChain struct {
	mu      sync.Mutex
	size    float64
	dpi     float64
	entries []guiFallbackEntry
}

type guiFallbackEntry struct {
	path   string
	failed bool          // unreadable or unparseable; never try again
	cov    *runeCoverage // built on first parse, nil until then
	face   font.Face     // retained only once the font has supplied a glyph
}

// faceFor returns the first fallback face that owns a glyph for r, walking
// the fonts in priority order, or nil when none of them covers it.
func (c *guiFallbackChain) faceFor(r rune) font.Face {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.entries {
		e := &c.entries[i]
		if e.failed {
			continue
		}
		if e.face != nil {
			if _, ok := e.face.GlyphAdvance(r); ok {
				return e.face
			}
			continue
		}
		if e.cov != nil && !e.cov.maybeHas(r) {
			continue
		}

		face, err := openFallbackFace(e.path, c.size, c.dpi)
		if err != nil {
			e.failed = true
			DebugLog("GUI_FONT: fallback present but unusable: %s: %v", e.path, err)
			continue
		}
		if e.cov == nil {
			cov := &runeCoverage{}
			for probe := rune(0); probe < 0x20000; probe++ {
				if probe >= 0xD800 && probe <= 0xDFFF {
					continue // surrogates are not runes a cmap can hold
				}
				if _, ok := face.GlyphAdvance(probe); ok {
					cov.bits[probe>>6] |= 1 << (uint(probe) & 63)
				}
			}
			e.cov = cov
		}
		if _, ok := face.GlyphAdvance(r); ok {
			e.face = face
			DebugLog("GUI_FONT: fallback loaded for U+%04X: %s", r, e.path)
			return face
		}
		// Parsed, mapped into the bitmap, not needed: dropped here, and the
		// bitmap answers for this font from now on.
		_ = face.Close()
		DebugLog("GUI_FONT: fallback probed for U+%04X and dropped: %s", r, e.path)
	}
	return nil
}

func (c *guiFallbackChain) close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var err error
	for i := range c.entries {
		if f := c.entries[i].face; f != nil {
			if e := f.Close(); e != nil {
				err = e
			}
			c.entries[i].face = nil
		}
	}
	return err
}

func openFallbackFace(path string, size, dpi float64) (font.Face, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := parseFontBytes(data)
	if err != nil {
		return nil, err
	}
	return opentype.NewFace(f, &opentype.FaceOptions{
		Size:    size,
		DPI:     dpi,
		Hinting: font.HintingFull,
	})
}

type fallbackFace struct {
	faces []font.Face
	chain *guiFallbackChain
}

func (f *fallbackFace) Close() error {
	var err error
	for _, face := range f.faces {
		if e := face.Close(); e != nil {
			err = e
		}
	}
	if f.chain != nil {
		if e := f.chain.close(); e != nil {
			err = e
		}
	}
	return err
}

func (f *fallbackFace) Metrics() font.Metrics {
	if len(f.faces) > 0 {
		return f.faces[0].Metrics()
	}
	return font.Metrics{}
}

func (f *fallbackFace) Kern(r0, r1 rune) fixed.Int26_6 {
	if len(f.faces) > 0 {
		return f.faces[0].Kern(r0, r1)
	}
	return 0
}

// chainFace resolves r through the lazy chain, or nil when the chain is
// absent or exhausted. Each glyph method consults it only after every
// already-open face has answered "no glyph".
func (f *fallbackFace) chainFace(r rune) font.Face {
	if f.chain == nil {
		return nil
	}
	return f.chain.faceFor(r)
}

func (f *fallbackFace) GlyphBounds(r rune) (bounds fixed.Rectangle26_6, advance fixed.Int26_6, ok bool) {
	for _, face := range f.faces {
		bounds, advance, ok = face.GlyphBounds(r)
		if ok {
			return bounds, advance, ok
		}
	}
	if face := f.chainFace(r); face != nil {
		return face.GlyphBounds(r)
	}
	if len(f.faces) > 0 {
		return f.faces[0].GlyphBounds(r)
	}
	return fixed.Rectangle26_6{}, 0, false
}

func (f *fallbackFace) GlyphAdvance(r rune) (advance fixed.Int26_6, ok bool) {
	for _, face := range f.faces {
		advance, ok = face.GlyphAdvance(r)
		if ok {
			return advance, ok
		}
	}
	if face := f.chainFace(r); face != nil {
		return face.GlyphAdvance(r)
	}
	if len(f.faces) > 0 {
		return f.faces[0].GlyphAdvance(r)
	}
	return 0, false
}

func (f *fallbackFace) Glyph(dot fixed.Point26_6, r rune) (dr image.Rectangle, mask image.Image, maskp image.Point, advance fixed.Int26_6, ok bool) {
	for _, face := range f.faces {
		dr, mask, maskp, advance, ok = face.Glyph(dot, r)
		if ok {
			return dr, mask, maskp, advance, ok
		}
	}
	if face := f.chainFace(r); face != nil {
		return face.Glyph(dot, r)
	}
	if len(f.faces) > 0 {
		return f.faces[0].Glyph(dot, r)
	}
	return image.Rectangle{}, nil, image.Point{}, 0, false
}

func getFontCandidates(fontName string) []string {
	var candidates []string
	if fontName != "" {
		candidates = append(candidates, fontName)
		if !strings.HasSuffix(strings.ToLower(fontName), ".ttf") {
			candidates = append(candidates, fontName+".ttf")
		}
		dirs := []string{
			`C:\Windows\Fonts`,
			"/usr/share/fonts/truetype",
			"/usr/share/fonts/TTF",
			"/usr/local/share/fonts",
			"/System/Library/Fonts/Supplemental",
			"/System/Library/Fonts",
		}
		for _, dir := range dirs {
			candidates = append(candidates, filepath.Join(dir, fontName))
			if !strings.HasSuffix(strings.ToLower(fontName), ".ttf") {
				candidates = append(candidates, filepath.Join(dir, fontName+".ttf"))
			}
			entries, err := os.ReadDir(dir)
			if err == nil {
				for _, e := range entries {
					if e.IsDir() {
						candidates = append(candidates, filepath.Join(dir, e.Name(), fontName))
						if !strings.HasSuffix(strings.ToLower(fontName), ".ttf") {
							candidates = append(candidates, filepath.Join(dir, e.Name(), fontName+".ttf"))
						}
					}
				}
			}
		}
	}

	defaultPaths := []string{
		`C:\Windows\Fonts\consola.ttf`,
		`C:\Windows\Fonts\lucon.ttf`,
		`C:\Windows\Fonts\cour.ttf`,
		`C:\Windows\Fonts\arial.ttf`,
		"/usr/share/fonts/truetype/ubuntu/UbuntuMono-R.ttf",
		"/usr/share/fonts/truetype/dejavu/DejaVuSansMono.ttf",
		"/usr/share/fonts/truetype/liberation/LiberationMono-Regular.ttf",
		"/usr/share/fonts/TTF/DejaVuSansMono.ttf",
		"/System/Library/Fonts/Supplemental/Courier New.ttf",
		"/System/Library/Fonts/Monaco.ttf",
	}
	candidates = append(candidates, defaultPaths...)
	return candidates
}

// loadBestFont attempts to find a suitable monospace TTF font on the system.
// If none is found, it falls back to a built-in bitmap font.
func loadBestFont(fontName string, size float64, dpi float64) (font.Face, int, int) {
	if size <= 0 {
		size = 18.0
	}

	var primaryFace font.Face
	var cellW, cellH int

	for _, path := range getFontCandidates(fontName) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		f, err := parseFontBytes(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "GUI_FONT: Error parsing %s: %v\n", path, err)
			continue
		}

		face, err := opentype.NewFace(f, &opentype.FaceOptions{
			Size:    size,
			DPI:     dpi,
			Hinting: font.HintingFull,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "GUI_FONT: Error creating face for %s: %v\n", path, err)
			continue
		}

		metrics := face.Metrics()
		cellH = (metrics.Ascent + metrics.Descent).Ceil()
		advance, _ := face.GlyphAdvance('A')
		cellW = advance.Ceil()

		msg := fmt.Sprintf("GUI_FONT: Successfully loaded %s (%dx%d)", path, cellW, cellH)
		fmt.Fprintln(os.Stderr, msg)
		DebugLog("%s", msg)
		primaryFace = face
		break
	}

	if primaryFace == nil {
		// Fallback to basicfont if no TTF found
		DebugLog("GUI_FONT: CRITICAL - No TTF font found! Falling back to basicfont 7x13 (ASCII only!)")
		return basicfont.Face7x13, 7, 13
	}

	// Existence is all the startup probe checks; the files themselves are
	// opened by the chain on first use. The old loop read and parsed every
	// fallback font here, which held hundreds of megabytes for glyphs most
	// sessions never draw.
	var chain *guiFallbackChain
	for _, path := range fallbackFontPaths {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		DebugLog("GUI_FONT: fallback present, deferred until first use: %s", path)
		if chain == nil {
			chain = &guiFallbackChain{size: size, dpi: dpi}
		}
		chain.entries = append(chain.entries, guiFallbackEntry{path: path})
	}

	return &fallbackFace{faces: []font.Face{primaryFace}, chain: chain}, cellW, cellH
}
