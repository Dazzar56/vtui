package vtui

import (
	"context"
)

// TaskContext provides a safe environment for background operations
// to interact with the main UI thread.
type TaskContext struct {
	context.Context
	Cancel context.CancelFunc
}

// RunOnUI safely executes the given function on the main UI thread.
// This MUST be used for any updates to ScreenObjects (changing text, showing dialogs).
func (ctx *TaskContext) RunOnUI(fn func()) {
	FrameManager.PostTask(fn)
}

// RunAsync starts a background goroutine and provides it with a TaskContext.
// This is the foundation for background plugins, VFS operations, and heavy logic.
func RunAsync(worker func(ctx *TaskContext)) *TaskContext {
	ctx, cancel := context.WithCancel(context.Background())
	taskCtx := &TaskContext{
		Context: ctx,
		Cancel:  cancel,
	}

	go func() {
		worker(taskCtx)
	}()

	return taskCtx
}