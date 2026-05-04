//go:build freebsd || openbsd || netbsd || dragonfly || darwin

package vtui

var (
	shmId   int
	shmAddr uintptr
	shmData []byte
)