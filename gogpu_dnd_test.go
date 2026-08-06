//go:build !freebsd && !dragonfly && !openbsd && !netbsd && !illumos && !solaris && !arm

package vtui

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

// withInlineDropTarget installs a target and makes delivery inline, so a
// test sees the whole gesture without a UI thread to hop to.
func withInlineDropTarget(t *testing.T, f func(ev *DragEvent) DropAction) {
	t.Helper()
	prev := DragDeliverToUI
	DragDeliverToUI = false
	SetDropTarget(DropTargetFunc(f))
	t.Cleanup(func() {
		DragDeliverToUI = prev
		SetDropTarget(nil)
	})
}

func TestGogpuDragPayloadTakesPathsAndURIs(t *testing.T) {
	p := gogpuDragPayload([]string{
		"/tmp/one.txt",
		"   ",
		"file:///tmp/two%20three.txt",
		"https://example.org/x",
	})

	want := []string{"/tmp/one.txt", filepath.FromSlash("/tmp/two three.txt")}
	if !reflect.DeepEqual(p.Paths, want) {
		t.Fatalf("Paths = %v, want %v", p.Paths, want)
	}
	if !reflect.DeepEqual(p.URIs, []string{"https://example.org/x"}) {
		t.Fatalf("URIs = %v, want the remote one alone", p.URIs)
	}
	if !p.HasFiles() || !p.OffersFiles() {
		t.Fatal("a payload holding paths offers files")
	}
	if empty := gogpuDragPayload([]string{"", "  "}); !empty.IsEmpty() || len(empty.Kinds) != 0 {
		t.Fatalf("payload = %+v, want nothing at all", empty)
	}
}

func TestGogpuDropCellDividesByCellSize(t *testing.T) {
	cases := []struct {
		px   float64
		cell int
		want int
	}{
		{0, 8, 0},
		{7.9, 8, 0},
		{8, 8, 1},
		{123.5, 8, 15},
		{-4, 8, 0},
		{100, 0, 0},
	}
	for _, c := range cases {
		if got := gogpuDropCell(c.px, c.cell); got != c.want {
			t.Fatalf("gogpuDropCell(%v, %d) = %d, want %d", c.px, c.cell, got, c.want)
		}
	}
}

func TestGogpuHostDeliversDropAsEnterThenDrop(t *testing.T) {
	var phases []DragPhase
	var last DragEvent
	withInlineDropTarget(t, func(ev *DragEvent) DropAction {
		phases = append(phases, ev.Phase)
		last = *ev
		return DropCopy
	})

	host := &GogpuHost{cellW: 8, cellH: 16}
	host.deliverFileDrop([]string{"/tmp/a.txt"}, 40, 48)

	if !reflect.DeepEqual(phases, []DragPhase{DragEnter, DragDrop}) {
		t.Fatalf("phases = %v, want one enter followed by the drop", phases)
	}
	if last.X != 5 || last.Y != 3 {
		t.Fatalf("cell = %d,%d, want 5,3", last.X, last.Y)
	}
	if last.Allowed != DropCopy || last.Suggested != DropCopy {
		t.Fatalf("actions = %s / %s, want copy and only copy", last.Allowed, last.Suggested)
	}
	if !reflect.DeepEqual(last.Payload.Paths, []string{"/tmp/a.txt"}) {
		t.Fatalf("paths = %v, want the dropped file", last.Payload.Paths)
	}
}

func TestGogpuHostIgnoresEmptyDrop(t *testing.T) {
	asked := false
	withInlineDropTarget(t, func(ev *DragEvent) DropAction {
		asked = true
		return DropNone
	})

	host := &GogpuHost{cellW: 8, cellH: 16}
	host.deliverFileDrop(nil, 10, 10)
	host.deliverFileDrop([]string{"  "}, 10, 10)

	if asked {
		t.Fatal("a drop carrying nothing is not a gesture")
	}
}

func TestGogpuHostReportsDragDirections(t *testing.T) {
	host := &GogpuHost{}
	if host.AcceptsDrops() {
		t.Fatal("without a window there is nothing to drop on")
	}
	if host.CanStartDrag() {
		t.Fatal("dragging out of a gogpu window is not implemented yet")
	}
	_, err := host.StartDrag(DragPayload{Paths: []string{"/tmp/a.txt"}}, DropCopy)
	if !errors.Is(err, ErrDragUnsupported) {
		t.Fatalf("err = %v, want ErrDragUnsupported", err)
	}
}