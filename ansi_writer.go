package vtui

import (
	"strconv"
	"strings"
)

// attributesToANSI генерирует минимальную ANSI-последовательность для перехода между состояниями аттрибутов.
func attributesToANSI(attr, lastAttr uint64, activePal *[256]uint32, profile ColorProfile, quantCache map[uint32]uint8) string {
	var b strings.Builder
	writeAttributesToANSI(&b, attr, lastAttr, activePal, profile, quantCache)
	return b.String()
}

// writeAttributesToANSI merges style, fg and bg into a single CSI
// ("\x1b[1;38;2;R;G;B;48;2;R;G;Bm") instead of one escape per component:
// identical terminal semantics, ~4 bytes shorter per component. Stack
// buffers only, so the Render hot path stays allocation-free.
func writeAttributesToANSI(b *strings.Builder, attr, lastAttr uint64, activePal *[256]uint32, profile ColorProfile, quantCache map[uint32]uint8) {
	if attr == lastAttr {
		return
	}

	resetTriggered := false
	const flagsMask = (ForegroundIntensity | ForegroundDim | CommonLvbUnderscore | CommonLvbReverse | CommonLvbStrikeout)
	if (lastAttr&flagsMask)&^(attr&flagsMask) != 0 {
		b.WriteString("\x1b[0m")
		lastAttr = 0
		resetTriggered = true
	}

	var buf [64]byte
	n := 0
	first := true
	writeSep := func() {
		if !first {
			buf[n] = ';'
			n++
		}
		first = false
	}

	if attr&ForegroundIntensity != 0 && lastAttr&ForegroundIntensity == 0 {
		writeSep()
		buf[n] = '1'
		n++
	}
	if attr&ForegroundDim != 0 && lastAttr&ForegroundDim == 0 {
		writeSep()
		buf[n] = '2'
		n++
	}
	if attr&CommonLvbUnderscore != 0 && lastAttr&CommonLvbUnderscore == 0 {
		writeSep()
		buf[n] = '4'
		n++
	}
	if attr&CommonLvbReverse != 0 && lastAttr&CommonLvbReverse == 0 {
		writeSep()
		buf[n] = '7'
		n++
	}
	if attr&CommonLvbStrikeout != 0 && lastAttr&CommonLvbStrikeout == 0 {
		writeSep()
		buf[n] = '9'
		n++
	}

	fgMask := IsFgRGB | (0xFF << 16)
	if resetTriggered || attr&fgMask != lastAttr&fgMask || (attr&IsFgRGB != 0 && GetRGBFore(attr) != GetRGBFore(lastAttr)) {
		writeSep()
		n += writeColorANSI(buf[n:], false, attr, activePal, profile, quantCache)
	}

	bgMask := IsBgRGB | (0xFF << 40)
	if resetTriggered || attr&bgMask != lastAttr&bgMask || (attr&IsBgRGB != 0 && GetRGBBack(attr) != GetRGBBack(lastAttr)) {
		writeSep()
		n += writeColorANSI(buf[n:], true, attr, activePal, profile, quantCache)
	}

	if n > 0 {
		b.WriteByte('\x1b')
		b.WriteByte('[')
		b.Write(buf[:n])
		b.WriteByte('m')
	}
}

// writeColorANSI appends one colour component — "P;2;R;G;B" (true colour) or
// "P;5;N" (palette), P being 38 for fg and 48 for bg — without a CSI prefix
// or trailing 'm'; returns the byte count.
func writeColorANSI(dst []byte, isBg bool, attr uint64, activePal *[256]uint32, profile ColorProfile, quantCache map[uint32]uint8) int {
	var rgbVal uint32
	var idxVal uint8
	if isBg {
		rgbVal = GetRGBBack(attr)
		idxVal = GetIndexBack(attr)
	} else {
		rgbVal = GetRGBFore(attr)
		idxVal = GetIndexFore(attr)
	}

	flag := IsFgRGB
	if isBg {
		flag = IsBgRGB
	}
	if attr&flag != 0 {
		if profile != ColorProfileTrueColor {
			if quantCache == nil {
				idxVal = findNearestColor(rgbVal, activePal, 256)
			} else if cachedIdx, ok := quantCache[rgbVal]; ok {
				idxVal = cachedIdx
			} else {
				maxColors := 256
				if profile == ColorProfile16 {
					maxColors = 16
				}
				idxVal = findNearestColor(rgbVal, activePal, maxColors)
				quantCache[rgbVal] = idxVal
			}
			if profile == ColorProfile16 {
				return copy(dst, idxTo16ColorANSI(isBg, idxVal))
			}
			return appendColor256(dst, isBg, idxVal)
		}
		r, g, b := rgb(rgbVal)
		return appendColorRGB(dst, isBg, r, g, b)
	}

	if profile == ColorProfile16 {
		return copy(dst, idxTo16ColorANSI(isBg, idxVal))
	}
	return appendColor256(dst, isBg, idxVal)
}

// appendColor256 appends "P;5;N" to dst and returns the byte count.
func appendColor256(dst []byte, isBg bool, idx uint8) int {
	if isBg {
		copy(dst, "48;5;")
	} else {
		copy(dst, "38;5;")
	}
	n := 5
	return n + copy(dst[n:], strconv.AppendInt(dst[n:n], int64(idx), 10))
}

// appendColorRGB appends "P;2;R;G;B" to dst and returns the byte count.
func appendColorRGB(dst []byte, isBg bool, r, g, b uint8) int {
	if isBg {
		copy(dst, "48;2;")
	} else {
		copy(dst, "38;2;")
	}
	n := 5
	n += copy(dst[n:], strconv.AppendInt(dst[n:n], int64(r), 10))
	dst[n] = ';'
	n++
	n += copy(dst[n:], strconv.AppendInt(dst[n:n], int64(g), 10))
	dst[n] = ';'
	n++
	return n + copy(dst[n:], strconv.AppendInt(dst[n:n], int64(b), 10))
}

// colorToANSI returns one colour component (no CSI, no 'm') as a string,
// keeping the string-based API the tests use.
func colorToANSI(isBg bool, attr uint64, activePal *[256]uint32, profile ColorProfile, quantCache map[uint32]uint8) string {
	var buf [24]byte
	n := writeColorANSI(buf[:], isBg, attr, activePal, profile, quantCache)
	return string(buf[:n])
}

var idx16FG = [...]string{
	"30", "31", "32", "33", "34", "35", "36", "37",
	"90", "91", "92", "93", "94", "95", "96", "97",
}
var idx16BG = [...]string{
	"40", "41", "42", "43", "44", "45", "46", "47",
	"100", "101", "102", "103", "104", "105", "106", "107",
}

func idxTo16ColorANSI(isBg bool, idx uint8) string {
	if idx > 15 {
		idx = idx % 16 // safe fallback
	}
	if isBg {
		return idx16BG[idx]
	}
	return idx16FG[idx]
}

func findNearestColor(rgbVal uint32, pal *[256]uint32, maxColors int) uint8 {
	if pal == nil {
		pal = &XTerm256Palette
	}
	r, g, b := rgb(rgbVal)
	var bestIdx uint8 = 0
	var bestDist int = 1000000

	for i := 0; i < maxColors; i++ {
		pr, pg, pb := rgb(pal[i])
		dr := int(r) - int(pr)
		dg := int(g) - int(pg)
		db := int(b) - int(pb)
		dist := dr*dr + dg*dg + db*db
		if dist < bestDist {
			bestDist = dist
			bestIdx = uint8(i)
			if dist == 0 {
				break
			}
		}
	}
	return bestIdx
}
