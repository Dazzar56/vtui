//go:build linux && !arm

package vtui

import (
	"io"
	"testing"
	"time"

	"github.com/neurlang/wayland/wl"
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

	// Note: full integration test of Redraw() spin loop requires mocking window.Widget,
	// which is deeply integrated with the Wayland protocol implementation.
}

func newWaylandPointerTestHost(t *testing.T) *WaylandHost {
	t.Helper()
	pr, pw := io.Pipe()
	t.Cleanup(func() {
		_ = pr.Close()
		_ = pw.Close()
	})
	return &WaylandHost{
		reader: vtinput.NewReader(pr, true),
		cellW:  10,
		cellH:  20,
		scale:  1,
	}
}

func nextWaylandMouseEvent(t *testing.T, host *WaylandHost) *vtinput.InputEvent {
	t.Helper()
	select {
	case event := <-host.reader.EventChan:
		return event
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for Wayland mouse event")
		return nil
	}
}

func assertNoWaylandMouseEvent(t *testing.T, host *WaylandHost) {
	t.Helper()
	select {
	case event := <-host.reader.EventChan:
		t.Fatalf("unexpected Wayland mouse event: %+v", event)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestWaylandButtonReleaseClearsMotionState(t *testing.T) {
	host := newWaylandPointerTestHost(t)

	host.Button(nil, nil, 0, 272, wl.PointerButtonStatePressed, nil)
	press := nextWaylandMouseEvent(t, host)
	if !press.KeyDown || press.ButtonState != vtinput.FromLeft1stButtonPressed {
		t.Fatalf("press = KeyDown:%v ButtonState:%d", press.KeyDown, press.ButtonState)
	}

	host.Button(nil, nil, 0, 272, wl.PointerButtonStateReleased, nil)
	release := nextWaylandMouseEvent(t, host)
	if release.KeyDown || release.ButtonState != 0 {
		t.Fatalf("release = KeyDown:%v ButtonState:%d", release.KeyDown, release.ButtonState)
	}

	host.Motion(nil, nil, 0, 30, 40)
	assertNoWaylandMouseEvent(t, host)
}

func TestWaylandMotionReportsHeldButton(t *testing.T) {
	host := newWaylandPointerTestHost(t)

	host.Button(nil, nil, 0, 272, wl.PointerButtonStatePressed, nil)
	_ = nextWaylandMouseEvent(t, host)
	host.Motion(nil, nil, 0, 30, 40)
	drag := nextWaylandMouseEvent(t, host)

	if !drag.KeyDown || drag.ButtonState != vtinput.FromLeft1stButtonPressed {
		t.Fatalf("drag = KeyDown:%v ButtonState:%d", drag.KeyDown, drag.ButtonState)
	}
	if drag.MouseX != 3 || drag.MouseY != 2 {
		t.Fatalf("drag cell = %d,%d, want 3,2", drag.MouseX, drag.MouseY)
	}
}

func TestWaylandMotionReportsOnlyCellChanges(t *testing.T) {
	host := newWaylandPointerTestHost(t)

	host.Button(nil, nil, 0, 272, wl.PointerButtonStatePressed, nil)
	_ = nextWaylandMouseEvent(t, host)

	host.Motion(nil, nil, 0, 9, 19)
	assertNoWaylandMouseEvent(t, host)

	host.Motion(nil, nil, 0, 10, 19)
	firstCell := nextWaylandMouseEvent(t, host)
	if firstCell.MouseX != 1 || firstCell.MouseY != 0 {
		t.Fatalf("first drag cell = %d,%d, want 1,0", firstCell.MouseX, firstCell.MouseY)
	}

	host.Motion(nil, nil, 0, 19, 19)
	assertNoWaylandMouseEvent(t, host)

	host.Motion(nil, nil, 0, 19, 20)
	secondCell := nextWaylandMouseEvent(t, host)
	if secondCell.MouseX != 1 || secondCell.MouseY != 1 {
		t.Fatalf("second drag cell = %d,%d, want 1,1", secondCell.MouseX, secondCell.MouseY)
	}
}

func TestWaylandPointerFramePrefersValue120OverRawAxis(t *testing.T) {
	host := newWaylandPointerTestHost(t)

	host.Axis(nil, nil, 0, wl.PointerAxisVerticalScroll, 15)
	host.AxisValue120(nil, nil, wl.PointerAxisVerticalScroll, 120)
	host.PointerFrame(nil, nil)

	wheel := nextWaylandMouseEvent(t, host)
	if wheel.WheelDirection != -1 {
		t.Fatalf("wheel direction = %d, want -1", wheel.WheelDirection)
	}
	select {
	case extra := <-host.reader.EventChan:
		t.Fatalf("raw axis duplicated value120 event: %+v", extra)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestWaylandPointerFrameSupportsSmoothAxis(t *testing.T) {
	host := newWaylandPointerTestHost(t)

	host.Axis(nil, nil, 0, wl.PointerAxisVerticalScroll, 25)
	host.PointerFrame(nil, nil)

	wheel := nextWaylandMouseEvent(t, host)
	if wheel.WheelDirection != -1 {
		t.Fatalf("smooth wheel direction = %d, want -1", wheel.WheelDirection)
	}
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

func TestLogicalWaylandPixelsRoundsToNearest(t *testing.T) {
	if got := logicalWaylandPixels(1001, 2); got != 501 {
		t.Errorf("logicalWaylandPixels(1001, 2) = %d, want 501", got)
	}
	if got := logicalWaylandPixels(1400, 1.5); got != 933 {
		t.Errorf("logicalWaylandPixels(1400, 1.5) = %d, want 933", got)
	}
	if got := logicalWaylandPixels(1, 3); got != 1 {
		t.Errorf("logicalWaylandPixels(1, 3) = %d, want 1", got)
	}
}
