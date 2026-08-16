package vtui

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"image"
	"image/png"
	"math"
	"strings"
	"testing"
)

// BenchmarkKittyInitialUpload measures the first frame for a fresh image:
// the full base64 encode and chunked upload plus the placement.
func BenchmarkKittyInitialUpload(b *testing.B) {
	photo := benchPhoto(1920, 1080)
	e := newKittyEncoder()
	list := []ImagePlacement{{Surface: photo, Col: 1, Row: 1, Cols: 80, Rows: 24}}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.uploaded = make(map[uint64]uint32)
		e.order = e.order[:0]
		e.hasPlaced = false
		var sb strings.Builder
		e.Render(&sb, list)
	}
}

// BenchmarkKittyPanReplacement measures the pan/zoom hot path after the
// image is uploaded: only the delete-all and the re-placement are emitted,
// with no base64 encoding of pixels.
func BenchmarkKittyPanReplacement(b *testing.B) {
	photo := benchPhoto(1920, 1080)
	e := newKittyEncoder()
	list := []ImagePlacement{{Surface: photo, Col: 1, Row: 1, Cols: 80, Rows: 24}}
	var warm strings.Builder
	e.Render(&warm, list)

	// Pan the same image: the source rectangle moves, the pixels do not.
	pan := []ImagePlacement{{Surface: photo, Col: 1, Row: 1, Cols: 80, Rows: 24, SrcX: 0, SrcY: 0, SrcW: 1600, SrcH: 900}}
	var sb byteBuffer
	sb.Grow(1 << 16)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sb = sb[:0]
		pan[0].SrcX = i % 320
		e.Render(&sb, pan)
	}
}

// benchPhotoNoisy is a photo-like surface: a smooth base plus deterministic
// high-frequency detail, so the entropy is representative of a real picture
// (a pure gradient would flatter any compressor).
func benchPhotoNoisy(w, h int) *ImageSurface {
	s := NewImageSurface(w, h)
	state := uint32(0x9E3779B9)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			state = state*1664525 + 1013904223
			r := int32(x*255/(w-1)) + int32((state>>24)&63) - 32
			g := int32(y*255/(h-1)) + int32((state>>16)&63) - 32
			b := int32((x+y)*128/(w+h-2)) + int32((state>>8)&63) - 32
			s.SetPixel(x, y, byte(clamp255(r)), byte(clamp255(g)), byte(clamp255(b)), 255)
		}
	}
	s.Opaque = true
	return s
}

// kittyRawRGBA flattens the surface into the contiguous RGBA buffer the kitty
// encoder streams as f=32.
func kittyRawRGBA(s *ImageSurface) []byte {
	raw := make([]byte, 0, s.Width*s.Height*4)
	for y := 0; y < s.Height; y++ {
		raw = append(raw, s.Pix[y*s.Stride:y*s.Stride+s.Width*4]...)
	}
	return raw
}

func kittyBase64(raw []byte) []byte {
	enc := make([]byte, base64.StdEncoding.EncodedLen(len(raw)))
	base64.StdEncoding.Encode(enc, raw)
	return enc
}

// kittyZlibBase64 is go-termimg's compression path: zlib BestSpeed over the
// RGBA data, sent with the kitty o=z flag.
func kittyZlibBase64(raw []byte) []byte {
	var cb bytes.Buffer
	zw, _ := zlib.NewWriterLevel(&cb, zlib.BestSpeed)
	_, _ = zw.Write(raw)
	_ = zw.Close()
	return kittyBase64(cb.Bytes())
}

// kittyPNGBase64 is rasterm's kitty path: PNG-encode (f=100) then base64.
// PNG is lossless, so display quality is identical; only bytes and CPU differ.
func kittyPNGBase64(img *image.RGBA) []byte {
	var pb bytes.Buffer
	_ = png.Encode(&pb, img)
	return kittyBase64(pb.Bytes())
}

// benchPhotoPlasma is a correlated, natural-looking surface (a 2D plasma).
// Real decoded photos sit between this and benchPhotoNoisy in entropy, so the
// two fixtures bracket the honest compression range.
func benchPhotoPlasma(w, h int) *ImageSurface {
	s := NewImageSurface(w, h)
	for y := 0; y < h; y++ {
		fy := float64(y) / float64(h)
		for x := 0; x < w; x++ {
			fx := float64(x) / float64(w)
			sin := func(a, b, c float64) float64 {
				return math.Sin(a*fx*6.283 + b*fy*6.283 + c)
			}
			s.SetPixel(x, y,
				byte(127.5+127.5*sin(3, 2, 0)),
				byte(127.5+127.5*sin(2, 4, 2.1)),
				byte(127.5+127.5*sin(5, 1, 4.2)),
				255)
		}
	}
	s.Opaque = true
	return s
}

// BenchmarkKittyPayloadRaw is our current upload transform: base64 of the raw
// RGBA stream, no compression.
func BenchmarkKittyPayloadRaw(b *testing.B) {
	photo := benchPhotoNoisy(1600, 960)
	raw := kittyRawRGBA(photo)
	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = kittyBase64(raw)
	}
	b.ReportMetric(float64(base64.StdEncoding.EncodedLen(len(raw))), "out-B")
}

// BenchmarkKittyPayloadZlib is the go-termimg o=z compression alternative.
func BenchmarkKittyPayloadZlib(b *testing.B) {
	photo := benchPhotoNoisy(1600, 960)
	raw := kittyRawRGBA(photo)
	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	var out []byte
	for i := 0; i < b.N; i++ {
		out = kittyZlibBase64(raw)
	}
	b.ReportMetric(float64(len(out)), "out-B")
}

// BenchmarkKittyPayloadPNG is the rasterm f=100 PNG alternative.
func BenchmarkKittyPayloadPNG(b *testing.B) {
	photo := benchPhotoNoisy(1600, 960)
	img := goSixelRGBA(photo)
	b.ReportAllocs()
	b.SetBytes(int64(img.Bounds().Dx() * img.Bounds().Dy() * 4))
	b.ResetTimer()
	var out []byte
	for i := 0; i < b.N; i++ {
		out = kittyPNGBase64(img)
	}
	b.ReportMetric(float64(len(out)), "out-B")
}

// BenchmarkKittyPayloadZlibPlasma repeats the go-termimg o=z path on a
// correlated plasma image, the realistic middle of the entropy range.
func BenchmarkKittyPayloadZlibPlasma(b *testing.B) {
	photo := benchPhotoPlasma(1600, 960)
	raw := kittyRawRGBA(photo)
	b.ReportAllocs()
	b.SetBytes(int64(len(raw)))
	b.ResetTimer()
	var out []byte
	for i := 0; i < b.N; i++ {
		out = kittyZlibBase64(raw)
	}
	b.ReportMetric(float64(len(out)), "out-B")
}

// BenchmarkKittyPayloadPNGPlasma repeats the rasterm f=100 path on the same
// plasma image.
func BenchmarkKittyPayloadPNGPlasma(b *testing.B) {
	photo := benchPhotoPlasma(1600, 960)
	img := goSixelRGBA(photo)
	b.ReportAllocs()
	b.SetBytes(int64(img.Bounds().Dx() * img.Bounds().Dy() * 4))
	b.ResetTimer()
	var out []byte
	for i := 0; i < b.N; i++ {
		out = kittyPNGBase64(img)
	}
	b.ReportMetric(float64(len(out)), "out-B")
}
