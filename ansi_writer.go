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

// writeAttributesToANSI writes styles, fg and bg into one CSI — no
// allocations on the Render hot path.
// Все компоненты (styles, fg, bg) сливаются в ОДИН CSI "\x1b[1;38;2;R;G;B;48;2;R;G;Bm" —
// семантика идентична раздельным последовательностям, но на ~4 байта короче на компонент.
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

	// 1. Style Flags
	var styles [5]byte
	sn := 0
	if attr&ForegroundIntensity != 0 && lastAttr&ForegroundIntensity == 0 {
		styles[sn] = '1'
		sn++
	}
	if attr&ForegroundDim != 0 && lastAttr&ForegroundDim == 0 {
		styles[sn] = '2'
		sn++
	}
	if attr&CommonLvbUnderscore != 0 && lastAttr&CommonLvbUnderscore == 0 {
		styles[sn] = '4'
		sn++
	}
	if attr&CommonLvbReverse != 0 && lastAttr&CommonLvbReverse == 0 {
		styles[sn] = '7'
		sn++
	}
	if attr&CommonLvbStrikeout != 0 && lastAttr&CommonLvbStrikeout == 0 {
		styles[sn] = '9'
		sn++
	}
	for i := 0; i < sn; i++ {
		writeSep()
		buf[n] = styles[i]
		n++
	}

	// 2. Foreground Color
	fgMask := IsFgRGB | (0xFF << 16)
	if resetTriggered || attr&fgMask != lastAttr&fgMask || (attr&IsFgRGB != 0 && GetRGBFore(attr) != GetRGBFore(lastAttr)) {
		writeSep()
		n += writeColorANSI(buf[n:], false, attr, activePal, profile, quantCache)
	}

	// 3. Background Color
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

// writeColorANSI appends a colour code (no CSI, no 'm') to dst and returns
// the byte count.
func writeColorANSI(dst []byte, isBg bool, attr uint64, activePal *[256]uint32, profile ColorProfile, quantCache map[uint32]uint8) int {
	isRGBFlag := IsFgRGB
	cmd := 38
	var rgbVal uint32
	var idxVal uint8

	if isBg {
		isRGBFlag = IsBgRGB
		cmd = 48
	}

	isRGB := (attr & isRGBFlag) != 0

	if isRGB {
		if isBg {
			rgbVal = GetRGBBack(attr)
		} else {
			rgbVal = GetRGBFore(attr)
		}

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
			n := copy(dst, strconv.AppendInt(dst[:0], int64(cmd), 10))
			dst[n] = ';'
			n++
			dst[n] = '5'
			n++
			dst[n] = ';'
			n++
			n += copy(dst[n:], strconv.AppendInt(dst[n:n], int64(idxVal), 10))
			return n
		}

		r, g, b := rgb(rgbVal)
		n := copy(dst, strconv.AppendInt(dst[:0], int64(cmd), 10))
		dst[n] = ';'
		n++
		dst[n] = '2'
		n++
		dst[n] = ';'
		n++
		n += copy(dst[n:], strconv.AppendInt(dst[n:n], int64(r), 10))
		dst[n] = ';'
		n++
		n += copy(dst[n:], strconv.AppendInt(dst[n:n], int64(g), 10))
		dst[n] = ';'
		n++
		n += copy(dst[n:], strconv.AppendInt(dst[n:n], int64(b), 10))
		return n
	}

	if isBg {
		idxVal = GetIndexBack(attr)
	} else {
		idxVal = GetIndexFore(attr)
	}

	if profile == ColorProfile16 {
		return copy(dst, idxTo16ColorANSI(isBg, idxVal))
	}
	n := copy(dst, strconv.AppendInt(dst[:0], int64(cmd), 10))
	dst[n] = ';'
	n++
	dst[n] = '5'
	n++
	dst[n] = ';'
	n++
	n += copy(dst[n:], strconv.AppendInt(dst[n:n], int64(idxVal), 10))
	return n
}

// colorToANSI returns a colour code (no CSI, no 'm') as a string; wrapper
// kept for the tests.
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
