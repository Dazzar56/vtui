package vtui

import (
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/unxed/vtinput"
	"golang.org/x/term"
)

// FrameType defines the type of a frame for introspection.
type FrameType int

const (
	TypeDesktop FrameType = iota
	TypeDialog
	TypeMenu
	TypeUser
)

// Frame is the interface that all top-level screen objects (windows, dialogs, menus) must implement.
type Frame interface {
	ProcessKey(e *vtinput.InputEvent) bool
	ProcessMouse(e *vtinput.InputEvent) bool
	Show(scr *ScreenBuf)
	ResizeConsole(w, h int)
	GetType() FrameType
	SetExitCode(code int)
	IsDone() bool
	IsBusy() bool // If true, FrameManager may skip the rendering phase
}

// frameManager manages the stack of frames and the main application loop.
type frameManager struct {
	frames         []Frame
	scr            *ScreenBuf
	RedrawChan     chan struct{}
	EventFilter    func(*vtinput.InputEvent) bool
	injectedEvents []*vtinput.InputEvent
}

// FrameManager is the global instance of the frame manager.
var FrameManager = &frameManager{}

// Init initializes the FrameManager with a ScreenBuf.
func (fm *frameManager) Init(scr *ScreenBuf) {
	fm.scr = scr
	fm.frames = make([]Frame, 0, 10)
	fm.RedrawChan = make(chan struct{}, 1)
	fm.injectedEvents = make([]*vtinput.InputEvent, 0)
	SetDefaultPalette()
	// Hide cursor globally at start
	fm.scr.SetCursorVisible(false)
}

// Push adds a new frame to the top of the stack.
func (fm *frameManager) Push(f Frame) {
	fm.frames = append(fm.frames, f)
}

// Pop removes the top-most frame from the stack.
func (fm *frameManager) Pop() {
	if len(fm.frames) > 0 {
		fm.frames = fm.frames[:len(fm.frames)-1]
	}
}
// Redraw triggers an asynchronous redraw request.
func (fm *frameManager) Redraw() {
	select {
	case fm.RedrawChan <- struct{}{}:
	default:
	}
}
// InjectEvents adds simulated input events to the front of the queue.
func (fm *frameManager) InjectEvents(events []*vtinput.InputEvent) {
	fm.injectedEvents = append(fm.injectedEvents, events...)
}
// Shutdown clears all frames, effectively stopping the application loop.
func (fm *frameManager) Shutdown() {
	fm.frames = nil
}

func (fm *frameManager) GetScreenSize() int {
	if fm.scr == nil { return 80 }
	return fm.scr.width
}

// Run starts the main application event loop.
func (fm *frameManager) Run() {
	// Restore cursor visibility on exit
	defer func() {
		fm.scr.SetCursorVisible(true)
		fm.scr.Flush()
	}()
	// Configure channel for receiving vtinput events
	reader := vtinput.NewReader(os.Stdin)
	eventChan := make(chan *vtinput.InputEvent, 1)
	go func() {
		for {
			e, err := reader.ReadEvent()
			if err != nil {
				if err != io.EOF {
					// TODO: Log error
				}
				close(eventChan)
				return
			}
			eventChan <- e
		}
	}()

	// Configure channel for tracking window resizing
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)

	// --- Main application loop ---
	for {
		if len(fm.frames) == 0 {
			return // No more frames, exit application.
		}

		// 1. Rendering
		topFrame := fm.frames[len(fm.frames)-1]

		// If the frame is "busy" (e.g., mass insertion in progress), skip drawing
		// and Flush to avoid flickering and save CPU.
		if !topFrame.IsBusy() {
			fm.scr.SetCursorVisible(false)
			for _, frame := range fm.frames {
				frame.Show(fm.scr)
			}
			fm.scr.Flush()
		}

		// 2. Dispatch helper
		dispatch := func(ev *vtinput.InputEvent, is_injected bool) {
			if len(fm.frames) == 0 { return }
			if !is_injected && fm.EventFilter != nil && fm.EventFilter(ev) { return }

			topFrame := fm.frames[len(fm.frames)-1]
			if ev.Type == vtinput.KeyEventType {
				if ev.VirtualKeyCode == vtinput.VK_Q && (ev.ControlKeyState&(vtinput.LeftCtrlPressed|vtinput.RightCtrlPressed)) != 0 {
					fm.frames = nil
					return
				}
				topFrame.ProcessKey(ev)
			} else if ev.Type == vtinput.MouseEventType {
				topFrame.ProcessMouse(ev)
			}
			if topFrame.IsDone() { fm.Pop() }
		}

		// 3. Event waiting (Blocking)
		var e *vtinput.InputEvent
		injected := false

		if len(fm.injectedEvents) > 0 {
			e = fm.injectedEvents[0]
			fm.injectedEvents = fm.injectedEvents[1:]
			injected = true
		} else {
			select {
			case <-fm.RedrawChan: continue
			case <-sigChan:
				width, height, _ := term.GetSize(int(os.Stdin.Fd()))
				fm.scr.AllocBuf(width, height)
				for _, f := range fm.frames { f.ResizeConsole(width, height) }
				continue
			case ev, ok := <-eventChan:
				if !ok { return }
				e = ev
			}
		}

		// Process the first received event
		dispatch(e, injected)

		// 4. Queue "Drain"
		// If events arrive in a dense stream (insertion), process them in a batch.
		for {
			select {
			case ev, ok := <-eventChan:
				if !ok { return }
				dispatch(ev, false)
				continue
			case <-time.After(2 * time.Millisecond):
				// If nothing arrived within 2ms, consider the burst finished.
				// This is critical for "instant" Bracketed Paste, because the terminal
				// sends data in chunks, and a normal drain may be interrupted too early.
			}
			break
		}
	}
}
