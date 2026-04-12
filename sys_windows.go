//go:build windows

package vtui

func countOpenFDs() int {
	// Not supported via simple VFS operations on Windows without CGO/Handle enumeration
	return -1
}