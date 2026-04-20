//go:build linux || freebsd || openbsd || netbsd || dragonfly || darwin

package vtui

import (
	"image"
	"image/color"
	"time"

	"github.com/mattn/go-runewidth"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// X11Renderer implements SurfaceRenderer by drawing directly to an image.RGBA buffer
// and pushing it to the X11Host.
type glyphKey struct {
	r  rune
	fg uint32
	w  int
}

type X11Renderer struct {
	host       *X11Host
	face       font.Face
	w, h       int
	glyphCache map[glyphKey]*image.RGBA
	cursorX    int
	cursorY    int
	cursorVis  bool

	// Состояние для управления миганием и очистки "шлейфа"
	oldCursorX int
	oldCursorY int
}

func NewX11Renderer(host *X11Host, face font.Face) *X11Renderer {
	return &X11Renderer{
		host:       host,
		face:       face,
		glyphCache: make(map[glyphKey]*image.RGBA),
	}
}

func (r *X11Renderer) SetPalette(pal *[256]uint32) {
	// X11 renderer uses TrueColor naturally, no palette switching needed for the host window.
}

func (r *X11Renderer) SetCursor(x, y int, visible bool) {
	r.cursorX = x
	r.cursorY = y
	r.cursorVis = visible
}

func (r *X11Renderer) Render(buf, shadow []CharInfo, w, h int, forceRedraw bool) {
	r.host.mu.Lock()
	defer r.host.mu.Unlock()

	// Мигание на основе системного времени (период 1 сек: 500мс горит, 500мс нет)
	blinkState := (time.Now().UnixNano() / int64(500*time.Millisecond)) % 2 == 0

	r.w, r.h = w, h
	img := r.host.imgBuf
	cw, ch := r.host.cellW, r.host.cellH

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x

			// Ячейка "грязная" если:
			// 1. Изменился символ/цвет (стандартно)
			// 2. Это новая позиция курсора (нужно инвертировать)
			// 3. Это старая позиция курсора (нужно вернуть как было)
			isCursorCell := (x == r.cursorX && y == r.cursorY && r.cursorVis && blinkState)
			wasCursorCell := (x == r.oldCursorX && y == r.oldCursorY)

			if !forceRedraw && buf[idx] == shadow[idx] && !isCursorCell && !wasCursorCell {
				continue
			}

			// Mark scanlines as dirty for the host.
			// Bounds check is required as X11 resize events are asynchronous.
			for iy := 0; iy < ch; iy++ {
				lineIdx := y*ch + iy
				if lineIdx >= 0 && lineIdx < len(r.host.dirtyLines) {
					r.host.dirtyLines[lineIdx] = true
				}
			}

			cell := buf[idx]
			px := x * cw
			py := y * ch

			if cell.Char == WideCharFiller {
				continue // Already handled by the previous wide character cell
			}

			// 1. Extract Colors
			bgRGB := GetRGBBack(cell.Attributes)
			if cell.Attributes&IsBgRGB == 0 {
				bgRGB = ThemePalette[GetIndexBack(cell.Attributes)]
			}
			fgRGB := GetRGBFore(cell.Attributes)
			if cell.Attributes&IsFgRGB == 0 {
				fgRGB = ThemePalette[GetIndexFore(cell.Attributes)]
			}

			// 2. Calculate widths
			char := rune(cell.Char)
			rw := runewidth.RuneWidth(char)
			if rw < 1 { rw = 1 }
			drawW := cw * rw

			// 3. Draw Background (Direct memory fill)
			br, bg, bb := uint8(bgRGB>>16), uint8(bgRGB>>8), uint8(bgRGB)
			for iy := 0; iy < ch; iy++ {
				if py+iy >= int(r.host.height) { break }
				rowStart := ((py + iy) * img.Stride) + (px * 4)
				for ix := 0; ix < drawW; ix++ {
					if px+ix >= int(r.host.width) { break }
					off := rowStart + ix*4
					if off+3 < len(img.Pix) {
						img.Pix[off], img.Pix[off+1], img.Pix[off+2], img.Pix[off+3] = br, bg, bb, 255
					}
				}
			}

			// 4. Draw Character on top of the background
			if char != 0 && char != ' ' {
				fgColor := color.RGBA{R: uint8(fgRGB >> 16), G: uint8(fgRGB >> 8), B: uint8(fgRGB), A: 255}
				bgColor := color.RGBA{R: br, G: bg, B: bb, A: 255}
				if !r.drawCustomChar(img, char, px, py, cw, ch, fgColor) {
					r.drawCachedGlyph(img, char, px, py, rw, fgRGB, bgRGB, fgColor, bgColor)
				}
			}

			// 5. Draw Cursor (Inverted Underline)
			if isCursorCell {
				thickness := 2
				if r.host.scale > 1 {
					thickness = 4
				}
				for iy := ch - thickness; iy < ch; iy++ {
					rowStart := (py+iy)*img.Stride + px*4
					for ix := 0; ix < cw; ix++ {
						off := rowStart + ix*4
						img.Pix[off] = 255 - img.Pix[off]
						img.Pix[off+1] = 255 - img.Pix[off+1]
						img.Pix[off+2] = 255 - img.Pix[off+2]
					}
				}
			}
		}
	}
	r.oldCursorX = r.cursorX
	r.oldCursorY = r.cursorY
}

func (r *X11Renderer) drawCachedGlyph(img *image.RGBA, char rune, px, py, rw int, fg, bg uint32, fgCol, bgCol color.Color) {
	key := glyphKey{char, fg, rw}
	cached, ok := r.glyphCache[key]

	cw, ch := r.host.cellW, r.host.cellH
	drawW := cw * rw
	if !ok {
		// Cache miss: render the glyph on its background to enable subpixel antialiasing.
		glyphImg := image.NewRGBA(image.Rect(0, 0, drawW, ch))

		// 1. Fill with the current background color.
		br, bg, bb, _ := bgCol.RGBA()
		br8, bg8, bb8 := uint8(br>>8), uint8(bg>>8), uint8(bb>>8)
		for i := 0; i < len(glyphImg.Pix); i += 4 {
			glyphImg.Pix[i] = br8
			glyphImg.Pix[i+1] = bg8
			glyphImg.Pix[i+2] = bb8
			glyphImg.Pix[i+3] = 255
		}

		// 2. Draw the character on top of the solid background.
		metrics := r.face.Metrics()
		d := &font.Drawer{
			Dst:  glyphImg,
			Src:  image.NewUniform(fgCol),
			Face: r.face,
			Dot:  fixed.Point26_6{X: fixed.I(0), Y: metrics.Ascent},
		}
		d.DrawString(string(char))

		// 3. "Cut out" the character by making the background transparent.
		for i := 0; i < len(glyphImg.Pix); i += 4 {
			if glyphImg.Pix[i] == br8 && glyphImg.Pix[i+1] == bg8 && glyphImg.Pix[i+2] == bb8 {
				glyphImg.Pix[i+3] = 0 // Set Alpha to 0 (transparent)
			}
		}

		cached = glyphImg
		r.glyphCache[key] = cached
	}

	// Composite the cached transparent glyph over the main buffer.
	for iy := 0; iy < ch; iy++ {
		if py+iy >= int(r.host.height) { continue }
		for ix := 0; ix < drawW; ix++ {
			if px+ix >= int(r.host.width) { break }

			glyphOff := (iy*cached.Stride + ix*4)
			if glyphOff+3 >= len(cached.Pix) { continue }
			alpha := cached.Pix[glyphOff+3]

			if alpha == 0 {
				continue // Skip transparent pixels
			}

			imgOff := ((py+iy)*img.Stride + (px+ix)*4)
			if imgOff+3 >= len(img.Pix) { continue }

			// Direct copy since alpha is 255 for non-background pixels
			img.Pix[imgOff]   = cached.Pix[glyphOff]
			img.Pix[imgOff+1] = cached.Pix[glyphOff+1]
			img.Pix[imgOff+2] = cached.Pix[glyphOff+2]
		}
	}
}

func (r *X11Renderer) Flush() {
	r.host.flushImage()
}

// drawCustomChar performs pixel-perfect drawing of lines and blocks.
// Returns true if the character was handled.
func (r *X11Renderer) drawCustomChar(img *image.RGBA, char rune, px, py, cw, ch int, col color.Color) bool {
	mx := px + cw/2
	my := py + ch/2
	thick := r.host.scale

	cr, cg, cb, _ := col.RGBA()
	r8, g8, b8 := uint8(cr>>8), uint8(cg>>8), uint8(cb>>8)

	drawHLine := func(x1, x2, y int) {
		for x := x1; x <= x2; x++ {
			for t := 0; t < thick; t++ {
				off := (y+t)*img.Stride + x*4
				img.Pix[off], img.Pix[off+1], img.Pix[off+2], img.Pix[off+3] = r8, g8, b8, 255
			}
		}
	}
	drawVLine := func(x, y1, y2 int) {
		for y := y1; y <= y2; y++ {
			for t := 0; t < thick; t++ {
				off := y*img.Stride + (x+t)*4
				img.Pix[off], img.Pix[off+1], img.Pix[off+2], img.Pix[off+3] = r8, g8, b8, 255
			}
		}
	}

	// Double line specifics
	ofs := cw / 4
	if ofs < 1 { ofs = 1 }

	switch char {
	// Single Lines
	case '─': drawHLine(px, px+cw-1, my); return true
	case '│': drawVLine(mx, py, py+ch-1); return true
	case '┌': drawHLine(mx, px+cw-1, my); drawVLine(mx, my, py+ch-1); return true
	case '┐': drawHLine(px, mx, my); drawVLine(mx, my, py+ch-1); return true
	case '└': drawHLine(mx, px+cw-1, my); drawVLine(mx, py, my); return true
	case '┘': drawHLine(px, mx, my); drawVLine(mx, py, my); return true
	case '├': drawHLine(mx, px+cw-1, my); drawVLine(mx, py, py+ch-1); return true
	case '┤': drawHLine(px, mx, my); drawVLine(mx, py, py+ch-1); return true
	case '┬': drawHLine(px, px+cw-1, my); drawVLine(mx, my, py+ch-1); return true
	case '┴': drawHLine(px, px+cw-1, my); drawVLine(mx, py, my); return true
	case '┼': drawHLine(px, px+cw-1, my); drawVLine(mx, py, py+ch-1); return true

	// Double Lines
	case '═': drawHLine(px, px+cw-1, my-ofs); drawHLine(px, px+cw-1, my+ofs); return true
	case '║': drawVLine(mx-ofs, py, py+ch-1); drawVLine(mx+ofs, py, py+ch-1); return true
	case '╔':
		drawHLine(mx-ofs, px+cw-1, my-ofs); drawHLine(mx+ofs, px+cw-1, my+ofs)
		drawVLine(mx-ofs, my-ofs, py+ch-1); drawVLine(mx+ofs, my+ofs, py+ch-1)
		return true
	case '╗':
		drawHLine(px, mx+ofs, my-ofs); drawHLine(px, mx-ofs, my+ofs)
		drawVLine(mx+ofs, my-ofs, py+ch-1); drawVLine(mx-ofs, my+ofs, py+ch-1)
		return true
	case '╚':
		drawHLine(mx-ofs, px+cw-1, my-ofs); drawHLine(mx+ofs, px+cw-1, my+ofs)
		drawVLine(mx-ofs, py, my-ofs); drawVLine(mx+ofs, py, my+ofs)
		return true
	case '╝':
		drawHLine(px, mx+ofs, my-ofs); drawHLine(px, mx-ofs, my+ofs)
		drawVLine(mx+ofs, py, my-ofs); drawVLine(mx-ofs, py, my+ofs)
		return true
	case '╠':
		drawHLine(mx-ofs, px+cw-1, my-ofs); drawHLine(mx+ofs, px+cw-1, my+ofs)
		drawVLine(mx-ofs, py, py+ch-1); drawVLine(mx+ofs, py, py+ch-1)
		return true
	case '╣':
		drawHLine(px, mx+ofs, my-ofs); drawHLine(px, mx-ofs, my+ofs)
		drawVLine(mx-ofs, py, py+ch-1); drawVLine(mx+ofs, py, py+ch-1)
		return true
	case '╩':
		drawHLine(px, px+cw-1, my+ofs)
		drawHLine(px, mx-ofs, my-ofs); drawHLine(mx+ofs, px+cw-1, my-ofs)
		drawVLine(mx-ofs, py, my-ofs); drawVLine(mx+ofs, py, my-ofs)
		return true
	case '╦':
		drawHLine(px, px+cw-1, my-ofs)
		drawHLine(px, mx-ofs, my+ofs); drawHLine(mx+ofs, px+cw-1, my+ofs)
		drawVLine(mx-ofs, my+ofs, py+ch-1); drawVLine(mx+ofs, my+ofs, py+ch-1)
		return true

	case '█':
		for y := py; y < py+ch; y++ {
			rowStart := y*img.Stride + px*4
			for x := 0; x < cw; x++ {
				off := rowStart + x*4
				img.Pix[off], img.Pix[off+1], img.Pix[off+2], img.Pix[off+3] = r8, g8, b8, 255
			}
		}
		return true
	}
	return false
}