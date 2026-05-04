//go:build linux

package vtui

import (
	"syscall"
	"unsafe"
)

var (
	shmId   int
	shmAddr uintptr
	shmData []byte
)

func init() {
	// Константы для системных вызовов IPC (Linux)
	const (
		ipcPrivate = 0
		ipcCreat   = 01000
		ipcRmid    = 0
	)

	// Аллоцируем сегмент 32MB (хватит для 4K дисплея)
	size := 3840 * 2160 * 4

	r1, _, err := syscall.Syscall(syscall.SYS_SHMGET, uintptr(ipcPrivate), uintptr(size), uintptr(ipcCreat|0600))
	if err != 0 {
		DebugLog("X11: shmget failed: %v", err)
		return
	}
	id := int(r1)

	r1, _, err = syscall.Syscall(syscall.SYS_SHMAT, uintptr(id), 0, 0)
	if err != 0 {
		syscall.Syscall(syscall.SYS_SHMCTL, uintptr(id), uintptr(ipcRmid), 0)
		DebugLog("X11: shmat failed: %v", err)
		return
	}
	addr := r1

	shmId = id
	shmAddr = addr
	shmData = unsafe.Slice((*byte)(unsafe.Pointer(shmAddr)), size)
	DebugLog("X11: Allocated shared memory segment (ID: %d)", shmId)
}