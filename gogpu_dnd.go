//go:build !freebsd && !dragonfly && !openbsd && !netbsd && !illumos && !solaris && !arm

package vtui

import "strings"

// gogpuDropAllowed is what an incoming drop is allowed to do. Copy only:
// gogpu hands us a finished drop and nothing else - not the actions the
// source permits, not the modifiers held, and there is no way back to the
// source to say what we did. A move under those conditions would delete
// somebody's files on a guess. See DRAGDROP.md.
const gogpuDropAllowed = DropCopy

// AcceptsDrops implements DragBackend: a gogpu window is a drop target on
// every platform gogpu supports, as soon as the window exists.
func (h *GogpuHost) AcceptsDrops() bool { return h != nil && h.app != nil }

// CanStartDrag implements DragSource. Dragging out needs gogpu's own drag
// source, which is a separate protocol on every platform and lands next, so
// the direction is reported as unavailable rather than half working.
func (h *GogpuHost) CanStartDrag() bool { return false }

// StartDrag implements DragBackend.
func (h *GogpuHost) StartDrag(payload DragPayload, allowed DropAction) (DropAction, error) {
	return DropNone, ErrDragUnsupported
}

// handleFileDrop is what gogpu calls when files are dropped on the window.
// It returns at once and delivers in the background: delivery waits for the
// UI thread, and this runs on the loop that draws the window, which the UI
// is about to need.
func (h *GogpuHost) handleFileDrop(paths []string, x, y float64) {
	go h.deliverFileDrop(paths, x, y)
}

// deliverFileDrop replays a finished gogpu drop as the gesture the core
// expects: exactly one enter, then the drop. There is nothing in between
// because gogpu reports nothing in between - no motion, no leave, and no
// word from the source about what it would allow.
func (h *GogpuHost) deliverFileDrop(paths []string, x, y float64) {
	payload := gogpuDragPayload(paths)
	if payload.IsEmpty() {
		return
	}

	h.mu.Lock()
	cellW, cellH, mods := h.cellW, h.cellH, h.currentMods
	h.mu.Unlock()

	ev := DragEvent{
		Phase:     DragEnter,
		X:         gogpuDropCell(x, cellW),
		Y:         gogpuDropCell(y, cellH),
		Modifiers: mods,
		Allowed:   gogpuDropAllowed,
		Suggested: DropCopy,
		Payload:   payload,
	}
	DeliverDragEvent(&ev)

	ev.Phase = DragDrop
	action := DeliverDragEvent(&ev)
	DebugLog("GOGPU_DND: %d file(s) dropped at %d,%d, handled as %s",
		len(payload.Paths), ev.X, ev.Y, action)

	if h.app != nil {
		h.app.RequestRedraw()
	}
}

// gogpuDragPayload turns what gogpu hands over into a payload. Its platform
// backends produce plain local paths, but a file: URI is decoded as well,
// and anything else is kept as a URI so a target can still make sense of it.
func gogpuDragPayload(entries []string) DragPayload {
	var p DragPayload
	for _, e := range entries {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(e), "file:") {
			if path, ok := URIToLocalPath(e); ok {
				p.Paths = append(p.Paths, path)
				continue
			}
			p.URIs = append(p.URIs, e)
			continue
		}
		if strings.Contains(e, "://") {
			p.URIs = append(p.URIs, e)
			continue
		}
		p.Paths = append(p.Paths, e)
	}
	if len(p.Paths) > 0 || len(p.URIs) > 0 {
		p.Kinds = []string{"text/uri-list"}
	}
	return p
}

// gogpuDropCell turns the pixel offset of a drop into a cell index. gogpu
// reports it in the same pixels as its pointer callbacks, so it is divided
// by the cell size exactly as those are.
func gogpuDropCell(px float64, cell int) int {
	if cell <= 0 || px <= 0 {
		return 0
	}
	return int(px) / cell
}