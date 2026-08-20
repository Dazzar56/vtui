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
		want                           int
	}{
		{name: "one times", width: 800, height: 600, pwidth: 800, pheight: 600, want: 1},
		{name: "two times", width: 800, height: 600, pwidth: 1600, pheight: 1200, want: 2},
		{name: "uses available dimension", width: 0, height: 600, pwidth: 0, pheight: 1200, want: 2},
		{name: "invalid physical size", width: 800, height: 600, pwidth: 400, pheight: 300, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := waylandScaleFromDimensions(tt.width, tt.height, tt.pwidth, tt.pheight); got != tt.want {
				t.Errorf("waylandScaleFromDimensions() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestLogicalWaylandPixelsRoundsUp(t *testing.T) {
	if got := logicalWaylandPixels(1001, 2); got != 501 {
		t.Errorf("logicalWaylandPixels(1001, 2) = %d, want 501", got)
	}
}
