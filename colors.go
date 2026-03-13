package vtui

// Basic color and attribute constants (matching WinCompat.h)
const (
	ForegroundBlue      uint64 = 0x0001 // text color contains blue.
	ForegroundGreen     uint64 = 0x0002 // text color contains green.
	ForegroundRed       uint64 = 0x0004 // text color contains red.
	ForegroundIntensity uint64 = 0x0008 // text color is intensified.

	BackgroundBlue      uint64 = 0x0010 // background color contains blue.
	BackgroundGreen     uint64 = 0x0020 // background color contains green.
	BackgroundRed       uint64 = 0x0040 // background color contains red.
	BackgroundIntensity uint64 = 0x0080 // background color is intensified.

	ForegroundTrueColor uint64 = 0x0100 // Use 24 bit RGB colors set by SetRGBFore
	BackgroundTrueColor uint64 = 0x0200 // Use 24 bit RGB colors set by SetRGBBack

	ExplicitLineBreak   uint64 = 0x0400 // Don't concatenate next line if this char is last
	ImportantLineChar   uint64 = 0x0800 // Dont skip this character when recomposing

	CommonLvbStrikeout  uint64 = 0x2000 // Strikeout.
	CommonLvbReverse    uint64 = 0x4000 // Reverse fore/back ground attribute.
	CommonLvbUnderscore uint64 = 0x8000 // Underscore.

	// Masks for basic 16-color attributes
	ForegroundRGB = ForegroundRed | ForegroundGreen | ForegroundBlue
	BackgroundRGB = BackgroundRed | BackgroundGreen | BackgroundBlue
)

// GetRGBFore extracts 24-bit RGB text color from attributes (bits 16-39).
func GetRGBFore(attr uint64) uint32 {
	return uint32((attr >> 16) & 0xFFFFFF)
}

// GetRGBBack extracts 24-bit RGB background color from attributes (bits 40-63).
func GetRGBBack(attr uint64) uint32 {
	return uint32((attr >> 40) & 0xFFFFFF)
}

// SetRGBFore sets 24-bit RGB text color into attributes, adding ForegroundTrueColor flag.
func SetRGBFore(attr uint64, rgb uint32) uint64 {
	return (attr & 0xFFFFFF000000FFFF) | ForegroundTrueColor | ((uint64(rgb) & 0xFFFFFF) << 16)
}

// SetRGBBack sets 24-bit RGB background color into attributes, adding BackgroundTrueColor flag.
func SetRGBBack(attr uint64, rgb uint32) uint64 {
	return (attr & 0x000000FFFFFFFFFF) | BackgroundTrueColor | ((uint64(rgb) & 0xFFFFFF) << 40)
}

// SetRGBBoth sets both RGB colors into attributes at once.
func SetRGBBoth(attr uint64, rgbFore uint32, rgbBack uint32) uint64 {
	return (attr & 0xFFFF) | ForegroundTrueColor | BackgroundTrueColor |
		((uint64(rgbFore) & 0xFFFFFF) << 16) | ((uint64(rgbBack) & 0xFFFFFF) << 40)
}