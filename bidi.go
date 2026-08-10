package vtui

import (
	"strings"
	"unicode/utf8"

	"github.com/rivo/uniseg"
	"golang.org/x/text/unicode/bidi"
)

// BidiMode selects how much of UAX #9 is applied.
type BidiMode int

const (
	BidiOff     BidiMode = iota // strings are laid out as stored
	BidiDisplay                 // strings are reordered for display
	BidiFull                    // reordering plus caret and input support
)

var DefaultBidiMode = BidiDisplay

// HasRTL checks if the string contains any strong RTL characters.
func HasRTL(s string) bool {
	isASCII := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			isASCII = false
			break
		}
	}
	if isASCII {
		return false
	}

	for _, r := range s {
		if r >= 0x0590 && r <= 0x08FF { // Hebrew, Arabic, Syriac, Thaana, Samaritan, N'Ko, Mandaic, etc.
			return true
		}
		if r >= 0xFB1D && r <= 0xFEFC { // Presentation Forms (Hebrew, Arabic)
			return true
		}
	}
	return false
}

// VisualString reorders s from logical to visual order.
func VisualString(s string) string {
	if DefaultBidiMode == BidiOff || !HasRTL(s) {
		return s
	}
	v, _ := VisualStringWithMap(s)
	return v
}

// VisualStringWithMap does the same and returns, for each cluster in
// visual order, the byte offset it had in the logical string.
func VisualStringWithMap(s string) (string, []int) {
	if DefaultBidiMode == BidiOff || !HasRTL(s) {
		return s, trivialMap(s)
	}

	type logicalCluster struct {
		text    string
		byteOff int
		runeIdx int
	}

	var logicalClusters []logicalCluster
	runeIdx := 0
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		from, to := g.Positions()
		logicalClusters = append(logicalClusters, logicalCluster{
			text:    s[from:to],
			byteOff: from,
			runeIdx: runeIdx,
		})
		runeIdx += utf8.RuneCountInString(s[from:to])
	}

	p := bidi.Paragraph{}
	_, err := p.SetString(s)
	if err != nil {
		return s, trivialMap(s)
	}
	order, err := p.Order()
	if err != nil {
		return s, trivialMap(s)
	}

	var visualBuilder strings.Builder
	var visualOffsets []int

	numRuns := order.NumRuns()
	for i := 0; i < numRuns; i++ {
		run := order.Run(i)
		start, end := run.Pos()

		// Collect clusters in this run
		var runClusters []logicalCluster
		for _, c := range logicalClusters {
			if c.runeIdx >= start && c.runeIdx <= end {
				runClusters = append(runClusters, c)
			}
		}

		isRTL := run.Direction() == bidi.RightToLeft
		if isRTL {
			// Reverse clusters in RTL run
			for i, j := 0, len(runClusters)-1; i < j; i, j = i+1, j-1 {
				runClusters[i], runClusters[j] = runClusters[j], runClusters[i]
			}
			// Apply bracket mirroring for single-rune clusters
			for i := range runClusters {
				if utf8.RuneCountInString(runClusters[i].text) == 1 {
					runClusters[i].text = bidi.ReverseString(runClusters[i].text)
				}
			}
		}

		for _, c := range runClusters {
			visualBuilder.WriteString(c.text)
			visualOffsets = append(visualOffsets, c.byteOff)
		}
	}

	return visualBuilder.String(), visualOffsets
}

func trivialMap(s string) []int {
	var offsets []int
	g := uniseg.NewGraphemes(s)
	for g.Next() {
		from, _ := g.Positions()
		offsets = append(offsets, from)
	}
	return offsets
}
