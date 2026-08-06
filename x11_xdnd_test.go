//go:build linux || openbsd || netbsd || dragonfly || darwin || freebsd || windows || illumos || solaris

package vtui

import (
	"reflect"
	"testing"

	"github.com/jezek/xgb/xproto"
)

// testDnd builds an instance with made up atoms, so the parts that only do
// protocol arithmetic can be tested without an X server.
func testDnd() *x11Dnd {
	return &x11Dnd{a: x11DndAtoms{
		uriList:    101,
		plainUTF8:  102,
		plain:      103,
		utf8:       104,
		actCopy:    201,
		actMove:    202,
		actLink:    203,
		actAsk:     204,
		actPrivate: 205,
		incr:       301,
	}}
}

func TestXdndPickTypePrefersFiles(t *testing.T) {
	d := testDnd()
	got := d.pickType([]xproto.Atom{103, 101, 999})
	if got != d.a.uriList {
		t.Fatalf("type = %d, want the uri list %d", got, d.a.uriList)
	}
	if got := d.pickType([]xproto.Atom{103, 102}); got != d.a.plainUTF8 {
		t.Fatalf("type = %d, want utf-8 plain text %d", got, d.a.plainUTF8)
	}
	if got := d.pickType([]xproto.Atom{777}); got != 0 {
		t.Fatalf("type = %d, want nothing for an unknown offer", got)
	}
}

func TestXdndEnterTypesFromMessage(t *testing.T) {
	d := testDnd()
	data := []uint32{42, uint32(5) << 24, 101, 0, 103}
	got := d.enterTypes(data)
	want := []xproto.Atom{101, 103}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("types = %v, want %v", got, want)
	}
	if got := d.enterTypes([]uint32{1, 2}); got != nil {
		t.Fatalf("a short message carries no types, got %v", got)
	}
}

func TestXdndActionMapping(t *testing.T) {
	d := testDnd()
	cases := []struct {
		atom xproto.Atom
		want DropAction
	}{
		{d.a.actCopy, DropCopy},
		{d.a.actMove, DropMove},
		{d.a.actLink, DropLink},
		{d.a.actAsk, DropCopy},
		{d.a.actPrivate, DropCopy},
		{0, DropNone},
		{999, DropNone},
	}
	for _, c := range cases {
		if got := d.actionOf(c.atom); got != c.want {
			t.Fatalf("actionOf(%d) = %s, want %s", c.atom, got, c.want)
		}
	}

	if got := d.atomOf(DropMove); got != d.a.actMove {
		t.Fatalf("atomOf(move) = %d, want %d", got, d.a.actMove)
	}
	if got := d.atomOf(DropCopy | DropMove); got != d.a.actMove {
		t.Fatalf("a set with move must report move, got %d", got)
	}
	if got := d.atomOf(DropNone); got != 0 {
		t.Fatalf("atomOf(none) = %d, want 0", got)
	}
}

func TestXdndKinds(t *testing.T) {
	d := testDnd()
	d.chosen = d.a.uriList
	if got := d.kinds(); !reflect.DeepEqual(got, []string{"text/uri-list"}) {
		t.Fatalf("kinds = %v", got)
	}
	d.chosen = d.a.utf8
	if got := d.kinds(); !reflect.DeepEqual(got, []string{"text/plain;charset=utf-8"}) {
		t.Fatalf("kinds = %v", got)
	}
	d.chosen = 0
	if got := d.kinds(); got != nil {
		t.Fatalf("kinds = %v, want none", got)
	}
}

func TestDndCell(t *testing.T) {
	if got := dndCell(25, 10); got != 2 {
		t.Fatalf("cell = %d, want 2", got)
	}
	if got := dndCell(25, 0); got != 0 {
		t.Fatalf("a zero cell size must not divide, got %d", got)
	}
	if got := dndCell(-5, 10); got != 0 {
		t.Fatalf("cell = %d, want 0", got)
	}
}

func TestPayloadOffersFiles(t *testing.T) {
	if (DragPayload{}).OffersFiles() {
		t.Fatal("an empty payload offers nothing")
	}
	if !(DragPayload{Kinds: []string{"TEXT/URI-LIST"}}).OffersFiles() {
		t.Fatal("an announced uri list counts as an offer of files")
	}
	if !(DragPayload{Paths: []string{"/tmp/x"}}).OffersFiles() {
		t.Fatal("actual paths count too")
	}
	if (DragPayload{Kinds: []string{"text/plain"}, Text: "hi"}).OffersFiles() {
		t.Fatal("plain text is not a file offer")
	}
}
