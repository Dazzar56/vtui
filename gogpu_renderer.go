//go:build !freebsd

package vtui

import (
	"image/color"
	"strings"
	"time"
	"sync"

	"github.com/gogpu/gg"
	"github.com/gogpu/gg/integration/ggcanvas"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/gg/text"
	_ "github.com/gogpu/gg/gpu" // Включаем аппаратное ускорение рендеринга
)

var (
	debugLastPhysW, debugLastPhysH int = -1, -1
	debugDrawCount                 int = 0
)
// deviceOnlyProvider wraps DeviceProvider to HIDE the ScaleFactor() method from ggcanvas.
// This prevents ggcanvas from applying automatic internal scaling, allowing vtui
// to control scaling manually and bypass the quadrant rendering bug (#327).
type deviceOnlyProvider struct {
	gpucontext.DeviceProvider
}

type GogpuRenderer struct {
	mu           sync.Mutex
	host         *GogpuHost
	face         text.Face
	cellW, cellH int // logical cell sizes from font measurement
	cols, rows   int // dimensions of the current renderBuf

	cursorX, cursorY int
	cursorVis        bool
	cursorShape      CursorShape

	canvas    *ggcanvas.Canvas
	renderBuf []CharInfo
	dirty     bool
}

func NewGogpuRenderer(host *GogpuHost, face text.Face, cw, ch int) *GogpuRenderer {
	return &GogpuRenderer{
		host:  host,
		face:  face,
		cellW: cw,
		cellH: ch,
	}
}

func (r *GogpuRenderer) Render(buf, shadow []CharInfo, w, h int, force bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cols = w
	r.rows = h

	needsRedraw := force
	if !needsRedraw {
		for i := 0; i < len(buf); i++ {
			if buf[i] != shadow[i] {
				needsRedraw = true
				break
			}
		}
	}
	if !needsRedraw {
		return
	}

	if len(r.renderBuf) != len(buf) {
		r.renderBuf = make([]CharInfo, len(buf))
	}
	copy(r.renderBuf, buf)
	r.dirty = true
}

func (r *GogpuRenderer) SetCursor(x, y int, visible bool, shape CursorShape) {
	r.cursorX, r.cursorY = x, y
	r.cursorVis = visible
	r.cursorShape = shape
}

func (r *GogpuRenderer) SetPalette(pal *[256]uint32) {}

func (r *GogpuRenderer) Flush() {
	r.host.mu.Lock()
	ctx := r.host.ctx
	app := r.host.app
	r.host.mu.Unlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if ctx == nil {
		if r.dirty && app != nil {
			app.RequestRedraw()
		}
		return
	}

	if len(r.renderBuf) == 0 {
		return
	}

	w, h := ctx.Width(), ctx.Height()

	if debugLastCtxW != w || debugLastCtxH != h {
		DebugLog("GOGPU_RENDERER_RESIZE: CtxLog:%dx%d HostCell:%dx%d HostGrid:%dx%d ExpectedLog:%dx%d",
			w, h, r.cellW, r.cellH, r.host.cols, r.host.rows, r.host.cols*r.cellW, r.host.rows*r.cellH)
		debugLastCtxW, debugLastCtxH = w, h
	}

	// Workaround for gogpu bug #327: Use PHYSICAL dimensions for the canvas
	// but tell ggcanvas to ignore system scaling by using deviceOnlyProvider.
	fw, fh := ctx.FramebufferWidth(), ctx.FramebufferHeight()

	if r.canvas == nil {
		provider := app.GPUContextProvider()
		if provider == nil { return }
		r.canvas, _ = ggcanvas.New(&deviceOnlyProvider{provider}, fw, fh)
	} else {
		r.canvas.Resize(fw, fh)
	}

	if r.dirty {
		drawStart := time.Now()
		var totalFills, totalGlyphs int
		var timeFills, timeGlyphs time.Duration
		
		r.canvas.Draw(func(dc *gg.Context) {
			// Workaround for gogpu bug #328: Clear state before drawing.
			dc.Identity()
			dc.ClearPath()

			drawCols := r.cols
			drawRows := r.rows

			if r.face != nil {
				dc.SetFont(r.face)
			}
			metrics := r.face.Metrics()
			ascent := float64(metrics.Ascent)

			for y := 0; y < drawRows; y++ {
				rowOff := y * drawCols
				for x := 0; x < drawCols; {
					cell := r.renderBuf[rowOff+x]
					fg, bg := r.getCellColors(cell)

					spanW := 0
					for x+spanW < drawCols {
						nextCell := r.renderBuf[rowOff+x+spanW]
						if nextCell.Char == WideCharFiller {
							spanW++
							continue
						}
						nextFg, nextBg := r.getCellColors(nextCell)
						if nextBg != bg || nextFg != fg {
							break
						}
						spanW++
					}

					lx := float64(x * r.cellW)
					ly := float64(y * r.cellH)
					spanPixW := float64(spanW * r.cellW)

					t_fill_0 := time.Now()
					dc.SetColor(bg)
					dc.DrawRectangle(lx, ly, spanPixW, float64(r.cellH))
					dc.Fill()
					timeFills += time.Since(t_fill_0)
					totalFills++

					var sb strings.Builder
					hasText := false

					for sx := 0; sx < spanW; {
						idx := rowOff + x + sx
						currCell := r.renderBuf[idx]

						if currCell.Char == WideCharFiller {
							sx++
							continue
						}

						rw := 1
						if x+sx+1 < drawCols && r.renderBuf[idx+1].Char == WideCharFiller {
							rw = 2
						}

						if currCell.Char != 0 && currCell.Char != ' ' && r.face != nil {
							sb.WriteRune(rune(currCell.Char))
							hasText = true
							totalGlyphs++
						} else {
							// Добавляем пробел(ы) для сохранения позиционирования последующих символов
							sb.WriteRune(' ')
							if rw == 2 {
								sb.WriteRune(' ')
							}
						}
						sx += rw
					}

					if hasText && r.face != nil {
						t_glyph_0 := time.Now()
						dc.SetColor(fg)
						// Отрисовываем всю строку (батч) целиком за 1 команду
						dc.DrawString(sb.String(), lx, ly+ascent)
						timeGlyphs += time.Since(t_glyph_0)
					}

					x += spanW
				}
			}

			if r.cursorVis && (time.Now().UnixMilli()/500)%2 == 0 {
				dc.SetColor(color.White)
				cx := float64(r.cursorX * r.cellW)
				cy := float64(r.cursorY * r.cellH)
				if r.cursorShape == CursorShapeUnderline {
					cy += float64(r.cellH) - 2
					dc.DrawRectangle(cx, cy, float64(r.cellW), 2)
				} else {
					dc.DrawRectangle(cx, cy, float64(r.cellW), float64(r.cellH))
				}
				dc.Fill()
			}
		})
		r.dirty = false
		drawDur := time.Since(drawStart)
		if drawDur > 5*time.Millisecond {
			DebugLog("GOGPU_RENDERER_PERF: DrawTotal: %v, Fills(%d): %v, Glyphs(%d): %v",
				drawDur, totalFills, timeFills, totalGlyphs, timeGlyphs)
		}
	}

	renderStart := time.Now()
	r.canvas.Render(ctx.RenderTarget())
	renderDur := time.Since(renderStart)
	if renderDur > 5*time.Millisecond {
		DebugLog("GOGPU_RENDERER: ggcanvas.Render took %v", renderDur)
	}
}

func (r *GogpuRenderer) getCellColors(cell CharInfo) (color.Color, color.Color) {
	bg := GetRGBBack(cell.Attributes)
	if cell.Attributes&IsBgRGB == 0 {
		bg = ThemePalette[GetIndexBack(cell.Attributes)]
	}
	fg := GetRGBFore(cell.Attributes)
	if cell.Attributes&IsFgRGB == 0 {
		fg = ThemePalette[GetIndexFore(cell.Attributes)]
	}

	f := color.RGBA{uint8(fg >> 16), uint8(fg >> 8), uint8(fg), 255}
	b := color.RGBA{uint8(bg >> 16), uint8(bg >> 8), uint8(bg), 255}
	return f, b
}