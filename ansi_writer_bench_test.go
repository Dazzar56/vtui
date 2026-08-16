package vtui

import (
	"strings"
	"testing"
)

var benchAttrCases = []struct {
	name    string
	attr    uint64
	last    uint64
	profile ColorProfile
}{
	{"NoChange", SetIndexFore(0, 9) | SetIndexBack(0, 2), SetIndexFore(0, 9) | SetIndexBack(0, 2), ColorProfileTrueColor},
	{"StyleToggle", ForegroundIntensity | SetIndexFore(0, 9), SetIndexFore(0, 9), ColorProfileTrueColor},
	{"IndexColors", SetIndexFore(0, 9) | SetIndexBack(0, 2), SetIndexFore(0, 1) | SetIndexBack(0, 4), ColorProfileTrueColor},
	{"RGB", SetRGBBoth(0, 0xff0000, 0x0033ff), SetRGBBoth(0, 0x102030, 0x405060), ColorProfileTrueColor},
	{"Index16", SetIndexFore(0, 9) | SetIndexBack(0, 2), SetIndexFore(0, 1) | SetIndexBack(0, 4), ColorProfile16},
}

func BenchmarkWriteAttributes(b *testing.B) {
	for _, tc := range benchAttrCases {
		b.Run(tc.name, func(b *testing.B) {
			var sb strings.Builder
			// Warm the builder so steady-state writes do not re-grow it;
			// Reset() would drop the buffer in Go 1.20+, so we let it grow
			// once and only account for the bytes we add.
			writeAttributesToANSI(&sb, tc.attr, tc.last, nil, tc.profile, nil)
			written := 0
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				before := sb.Len()
				writeAttributesToANSI(&sb, tc.attr, tc.last, nil, tc.profile, nil)
				written += sb.Len() - before
			}
			b.ReportMetric(float64(written)/float64(b.N), "bytes/op")
		})
	}
}

// renderBuf fills a w x h screen: every cell has a character and a colour that
// changes per column, so the renderer exercises style, colour and cursor
// emission on every cell.
func renderBuf(w, h int) (buf, shadow []CharInfo) {
	buf = make([]CharInfo, w*h)
	shadow = make([]CharInfo, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			buf[i] = CharInfo{Char: uint64('a' + (x % 26)), Attributes: SetIndexFore(0, uint8(x%16)) | SetIndexBack(0, uint8((x+y)%16))}
			shadow[i] = buf[i]
		}
	}
	return
}

func newBenchRenderer() *AnsiRenderer {
	return &AnsiRenderer{parent: &ScreenBuf{ColorProfile: ColorProfileTrueColor}}
}

// benchFrame composes one frame the way the event loop does: Render into the
// shared frameOut builder, then PrepareFlush resets it (dropping the buffer,
// which the next Render re-grows via frameCap).
func benchFrame(r *AnsiRenderer, buf, shadow []CharInfo, w, h int, force bool) {
	r.Render(buf, shadow, w, h, force)
	r.PrepareFlush()
}

func BenchmarkRender_Full(b *testing.B) {
	w, h := 80, 25
	buf, shadow := renderBuf(w, h)
	r := newBenchRenderer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchFrame(r, buf, shadow, w, h, true)
	}
}

func BenchmarkRender_Sparse(b *testing.B) {
	w, h := 80, 25
	buf, shadow := renderBuf(w, h)
	// A handful of changed cells, as in an incremental update.
	buf[3] = CharInfo{Char: 'X', Attributes: SetIndexFore(0, 2) | SetIndexBack(0, 7)}
	buf[4] = CharInfo{Char: 'Y', Attributes: SetIndexFore(0, 2) | SetIndexBack(0, 7)}
	buf[50] = CharInfo{Char: 'Z', Attributes: SetIndexFore(0, 2) | SetIndexBack(0, 7)}
	r := newBenchRenderer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchFrame(r, buf, shadow, w, h, false)
	}
}

func BenchmarkRender_TextOnly(b *testing.B) {
	// No colour changes at all: the common case for a scrolling text pane.
	w, h := 80, 25
	buf, shadow := renderBuf(w, h)
	for i := range buf {
		buf[i].Attributes = 0
		shadow[i].Attributes = 0
	}
	r := newBenchRenderer()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchFrame(r, buf, shadow, w, h, true)
	}
}
