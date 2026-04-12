//go:build !windows

package vtui

import "os"

func countOpenFDs() int {
	files, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		return -1
	}
	return len(files)
}