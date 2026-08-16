package vtui

import (
	"bytes"
	"image"
	"image/color"
	"io"
	"strings"
	"testing"

	rasterm "github.com/BourgeoisBear/rasterm"
	gosixel "github.com/mattn/go-sixel"
)

// benchPhoto builds a deterministic opaque RGBA surface with gradient and
// banding that exercises the quantiser and the RLE encoder like a real photo.
func benchPhoto(w, h int) *ImageSurface {
	s := NewImageSurface(w, h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r := byte((x * 255) / (w - 1))
			g := byte((y * 255) / (h - 1))
			b := byte(((x + y) * 128) / (w + h - 2))
			s.SetPixel(x, y, r, g, b, 255)
		}
	}
	s.Opaque = true // real decoders mark opaque photos; enables the box scaler
	return s
}

// 80x24 cells on the real device-pixel grid: Windows Terminal rasterises
// sixel into a fixed 10x20 virtual cell, the non-WT fallback into 8x16.
const (
	benchDWwt = 80 * 10
	benchDHwt = 24 * 20
	benchDWfb = 80 * 8
	benchDHfb = 24 * 16
)

func BenchmarkSixelEncodePhotoWT(b *testing.B) {
	photo := benchPhoto(1920, 1080)
	e := newSixelEncoder()
	b.ReportAllocs()
	b.ResetTimer()
	var out string
	for i := 0; i < b.N; i++ {
		out = e.encode(photo, 0, 0, photo.Width, photo.Height, benchDWwt, benchDHwt)
	}
	b.ReportMetric(float64(len(out)), "out-B")
}

func BenchmarkSixelEncodePhotoFallback(b *testing.B) {
	photo := benchPhoto(1920, 1080)
	e := newSixelEncoder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = e.encode(photo, 0, 0, photo.Width, photo.Height, benchDWfb, benchDHfb)
	}
}

func BenchmarkSixelEncodeSameSize(b *testing.B) {
	photo := benchPhoto(benchDWwt, benchDHwt)
	e := newSixelEncoder()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = e.encode(photo, 0, 0, photo.Width, photo.Height, benchDWwt, benchDHwt)
	}
}

func BenchmarkSixelQuantize(b *testing.B) {
	src := benchPhoto(benchDWwt, benchDHwt)
	idx := make([]byte, src.Width*src.Height)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sixelQuantize(src, idx)
	}
}

func BenchmarkSixelEmitData(b *testing.B) {
	src := benchPhoto(benchDWwt, benchDHwt)
	idx := make([]byte, src.Width*src.Height)
	sixelQuantize(src, idx)
	remap, regs := sixelRegisters(idx)
	e := newSixelEncoder()
	var sb strings.Builder
	sb.Grow(1 << 20)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sb.Reset()
		e.sixelEmitData(&sb, idx, remap, len(regs), src.Width, src.Height)
	}
	b.ReportMetric(float64(sb.Len()), "out-B")
}

// BenchmarkSixelScaleBox isolates the box downsampler the sixel path uses
// for opaque photos: 1920x1080 to an 80x24 WT cell grid (800x480).
func BenchmarkSixelScaleBox(b *testing.B) {
	photo := benchPhoto(1920, 1080)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = scaleSurfaceBox(photo, benchDWwt, benchDHwt)
	}
}

// BenchmarkSixelEncodePan models one pan step: the destination size stays
// fixed while the source crop offset moves, so the cache misses and the full
// scale+quantise+emit pipeline runs again.
func BenchmarkSixelEncodePan(b *testing.B) {
	photo := benchPhoto(1920, 1080)
	e := newSixelEncoder()
	const (
		visW = 1600
		visH = 900
	)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sx := (i * 7) % (photo.Width - visW)
		sy := (i * 5) % (photo.Height - visH)
		_ = e.encode(photo, sx, sy, visW, visH, benchDWwt, benchDHwt)
	}
}

// benchCube255 matches the fixed palette our encoder quantises to: the
// 6x6x6 colour cube plus 39 grey levels.
func benchCube255() color.Palette {
	p := make(color.Palette, 0, 255)
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				p = append(p, color.NRGBA{uint8(r * 51), uint8(g * 51), uint8(b * 51), 255})
			}
		}
	}
	for g := 0; g < 39; g++ {
		v := uint8(g * 255 / 38)
		p = append(p, color.NRGBA{v, v, v, 255})
	}
	return p
}

// goSixelRGBA copies the bench surface into an image.RGBA for go-sixel.
func goSixelRGBA(s *ImageSurface) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, s.Width, s.Height))
	for y := 0; y < s.Height; y++ {
		copy(img.Pix[y*img.Stride:(y+1)*img.Stride], s.Pix[y*s.Stride:y*s.Stride+s.Width*4])
	}
	return img
}

// BenchmarkGoSixelFixedDither matches our encoder: a fixed 255-colour palette,
// Floyd-Steinberg dither, and the same 800x480 raster.
func BenchmarkGoSixelFixedDither(b *testing.B) {
	photo := benchPhoto(benchDWwt, benchDHwt)
	img := goSixelRGBA(photo)
	enc := gosixel.NewEncoder(io.Discard)
	enc.Palette = benchCube255()
	enc.Dither = true
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(img); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkGoSixelAdaptiveDither is go-sixel's default strength: a median-cut
// adaptive palette plus Floyd-Steinberg dither.
func BenchmarkGoSixelAdaptiveDither(b *testing.B) {
	photo := benchPhoto(benchDWwt, benchDHwt)
	img := goSixelRGBA(photo)
	enc := gosixel.NewEncoder(io.Discard)
	enc.Dither = true
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(img); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSixelRenderCachedHit(b *testing.B) {
	photo := benchPhoto(benchDWwt, benchDHwt)
	e := newSixelEncoder()
	list := []ImagePlacement{{Surface: photo, Col: 1, Row: 1, Cols: 80, Rows: 24}}
	// Prime the cache.
	var warm strings.Builder
	e.Render(&warm, list, 8, 16)

	// The screen buffer reuses its backing array across frames (byteBuffer);
	// model that with a retained-capacity buffer reset via [:0].
	var sb byteBuffer
	sb.Grow(1 << 20)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sb = sb[:0]
		e.Render(&sb, list, 8, 16)
	}
}

// benchPalettedFromIdx quantises the surface with our fixed-palette ditherer
// and returns it as an image.Paletted, the input rasterm's SixelWriteImage
// requires (rasterm leaves dithering to the caller).
func benchPalettedFromIdx(s *ImageSurface) *image.Paletted {
	idx := make([]byte, s.Width*s.Height)
	sixelQuantize(s, idx)
	pal := make(color.Palette, sixelMaxColors)
	for i := 0; i < sixelMaxColors; i++ {
		c := sixelPalette[i]
		pal[i] = color.NRGBA{R: c[0], G: c[1], B: c[2], A: 255}
	}
	img := image.NewPaletted(image.Rect(0, 0, s.Width, s.Height), pal)
	for i, v := range idx {
		if v == sixelIndexTransparent {
			v = 0
		}
		img.Pix[i] = v
	}
	return img
}

// BenchmarkRastermSixelEmit measures rasterm's emit stage on an already
// quantised image — the fair peer of BenchmarkSixelEmitData.
func BenchmarkRastermSixelEmit(b *testing.B) {
	photo := benchPhoto(benchDWwt, benchDHwt)
	img := benchPalettedFromIdx(photo)
	var out bytes.Buffer
	out.Grow(1 << 20)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out.Reset()
		if err := rasterm.SixelWriteImage(&out, img); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(out.Len()), "out-B")
}

// BenchmarkRastermSixelFull is rasterm's honest end-to-end cost inside our
// app: quantise + build the paletted image, then emit.
func BenchmarkRastermSixelFull(b *testing.B) {
	photo := benchPhoto(benchDWwt, benchDHwt)
	var out bytes.Buffer
	out.Grow(1 << 20)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out.Reset()
		img := benchPalettedFromIdx(photo)
		if err := rasterm.SixelWriteImage(&out, img); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(out.Len()), "out-B")
}
