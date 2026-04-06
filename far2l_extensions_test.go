package vtui

import "testing"

func TestFar2lClipboard_Disabled(t *testing.T) {
	Far2lEnabled = false
	ok := SetFar2lClipboard("test")
	if ok {
		t.Error("SetFar2lClipboard should return false when Far2lEnabled is false")
	}

	_, ok = GetFar2lClipboard()
	if ok {
		t.Error("GetFar2lClipboard should return false when Far2lEnabled is false")
	}
}