package vtui

import "testing"

func TestColors_IndexAndRGB(t *testing.T) {
	// Test 1: SetIndexFore
	attr := uint64(0)
	attr = SetIndexFore(attr, 42)
	if attr&IsFgRGB != 0 {
		t.Error("SetIndexFore should not set IsFgRGB flag")
	}
	if GetIndexFore(attr) != 42 {
		t.Errorf("GetIndexFore expected 42, got %d", GetIndexFore(attr))
	}

	// Test 2: SetRGBFore overwrites index and sets flag
	attr = SetRGBFore(attr, 0xAABBCC)
	if attr&IsFgRGB == 0 {
		t.Error("SetRGBFore should set IsFgRGB flag")
	}
	if GetRGBFore(attr) != 0xAABBCC {
		t.Errorf("GetRGBFore expected AABBCC, got %X", GetRGBFore(attr))
	}

	// Test 3: SetIndexBack clears IsBgRGB
	attr = SetRGBBack(attr, 0x112233)
	attr = SetIndexBack(attr, 7)
	if attr&IsBgRGB != 0 {
		t.Error("SetIndexBack should clear IsBgRGB flag")
	}
	if GetIndexBack(attr) != 7 {
		t.Errorf("GetIndexBack expected 7, got %d", GetIndexBack(attr))
	}

	// Test 4: SetIndexBoth
	attr = SetIndexBoth(0, 5, 6)
	if GetIndexFore(attr) != 5 || GetIndexBack(attr) != 6 {
		t.Error("SetIndexBoth failed")
	}
	if attr&IsFgRGB != 0 || attr&IsBgRGB != 0 {
		t.Error("SetIndexBoth should not set RGB flags")
	}
}