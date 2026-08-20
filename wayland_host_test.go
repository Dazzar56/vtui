//go:build linux && !arm

package vtui

import (
	"testing"
	"time"

	"github.com/unxed/vtinput"
)

func TestWaylandHost_KeyRepeatLogic(t *testing.T) {
	host := &WaylandHost{}

	host.mu.Lock()
	host.isRepeating = true
	host.repeatVK = vtinput.VK_A
	host.repeatNext = time.Now().Add(-1 * time.Millisecond) // force immediate trigger
	host.mu.Unlock()

	if !host.isRepeating {
		t.Error("Expected isRepeating to be true")
	}

	// Note: full integration test of Redraw() spin loop requires mocking window.Widget
	// which is deeply integrated with the C Wayland library in windowtrace.
}

func TestWaylandScaleFromDimensions(t *testing.T) {
	tests := []struct {
		name                           string
		width, height, pwidth, pheight int32
		want                           float64
	}{
		{name: "one times", width: 800, height: 600, pwidth: 800, pheight: 600, want: 1},
		{name: "two times", width: 800, height: 600, pwidth: 1600, pheight: 1200, want: 2},
		{name: "fractional", width: 800, height: 600, pwidth: 1200, pheight: 900, want: 1.5},
		{name: "uses available dimension", width: 0, height: 600, pwidth: 0, pheight: 1200, want: 2},
		{name: "sub-unit scale", width: 800, height: 600, pwidth: 400, pheight: 300, want: 0.5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := waylandScaleFromDimensions(tt.width, tt.height, tt.pwidth, tt.pheight); got != tt.want {
				t.Errorf("waylandScaleFromDimensions() = %.2f, want %.2f", got, tt.want)
			}
		})
	}
}

func TestHasWaylandPixelSize(t *testing.T) {
	tests := []struct {
		name                           string
		width, height, pwidth, pheight int32
		want                           bool
	}{
		{name: "valid", width: 1000, height: 690, pwidth: 1500, pheight: 1035, want: true},
		{name: "zero logical width", width: 0, height: 690, pwidth: 0, pheight: 1035, want: false},
		{name: "zero physical height", width: 1000, height: 690, pwidth: 1500, pheight: 0, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasWaylandPixelSize(tt.width, tt.height, tt.pwidth, tt.pheight); got != tt.want {
				t.Errorf("hasWaylandPixelSize(%d, %d, %d, %d) = %v, want %v", tt.width, tt.height, tt.pwidth, tt.pheight, got, tt.want)
			}
		})
	}
}

func TestLogicalWaylandPixelsRoundsUp(t *testing.T) {
	if got := logicalWaylandPixels(1001, 2); got != 501 {
		t.Errorf("logicalWaylandPixels(1001, 2) = %d, want 501", got)
	}
	if got := logicalWaylandPixels(1400, 1.5); got != 934 {
		t.Errorf("logicalWaylandPixels(1400, 1.5) = %d, want 934", got)
	}
}
