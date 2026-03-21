package vtui

// Window is a non-modal container for UI elements.
type Window struct {
	BaseWindow
}

func NewWindow(x1, y1, x2, y2 int, title string) *Window {
	w := &Window{
		BaseWindow: *NewBaseWindow(x1, y1, x2, y2, title),
	}
	w.ShowClose = true
	w.ShowZoom = true
	return w
}

func (w *Window) IsModal() bool { return false }
func (w *Window) GetType() FrameType { return TypeUser }