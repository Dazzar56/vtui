package vtui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var (
	logMu              sync.Mutex
	logRotated         bool
	logFile            *os.File
	currentLogFilename string
	testLogger         func(string, ...any)
)

func rotateLogs(basePath string) {
	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
	ext := filepath.Ext(basePath)
	prefix := strings.TrimSuffix(basePath, ext)

	// Keep up to 5 backups: debug.4.log -> debug.5.log, etc.
	for i := 4; i >= 1; i-- {
		oldPath := fmt.Sprintf("%s.%d%s", prefix, i, ext)
		newPath := fmt.Sprintf("%s.%d%s", prefix, i+1, ext)
		os.Rename(oldPath, newPath)
	}

	// debug.log -> debug.1.log
	_ = os.Rename(basePath, prefix+".1"+ext)
}

var diskLoggingEnabled = true

// ConfigDiskLogging allows enabling or disabling writing to debug.log on disk.
// If disabled, logs are still kept in the in-memory ring buffer for crash reports.
func ConfigDiskLogging(enabled bool) {
	logMu.Lock()
	diskLoggingEnabled = enabled
	if !enabled && logFile != nil {
		logFile.Close()
		logFile = nil
	}
	logMu.Unlock()
}

// SetTestLogger installs the callback used by DebugLog when VTUI_DEBUG=test.
// It returns a restore function so a test can scope the global logger safely:
//
//	restore := SetTestLogger(t.Logf)
//	defer restore()
//
// The callback may be called from any goroutine, just like DebugLog itself.
func SetTestLogger(logger func(string, ...any)) func() {
	logMu.Lock()
	previous := testLogger
	testLogger = logger
	logMu.Unlock()

	return func() {
		logMu.Lock()
		testLogger = previous
		logMu.Unlock()
	}
}

// DebugLog writes a timestamped message to debug.log file.
// If the file exists at the start of the session, it is rotated
// (up to 3 files: debug.log, debug.1.log, debug.2.log).
func DebugLog(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	timestamp := time.Now().Format("15:04:05.000")
	fullMsg := fmt.Sprintf("[%s][P%d] %s", timestamp, os.Getpid(), msg)

	recordLogMemory(fullMsg)

	env := os.Getenv("VTUI_DEBUG")
	if env == "" {
		return
	}

	if env == "stderr" {
		logMu.Lock()
		_, _ = fmt.Fprintln(os.Stderr, fullMsg)
		logMu.Unlock()
		return
	}

	if env == "test" {
		logMu.Lock()
		logger := testLogger
		logMu.Unlock()
		if logger != nil {
			logger("%s", fullMsg)
			return
		}
		// Keep VTUI_DEBUG=test useful for ad-hoc test commands even when the
		// caller did not install a testing.T logger.
		logMu.Lock()
		_, _ = fmt.Fprintln(os.Stderr, fullMsg)
		logMu.Unlock()
		return
	}

	logMu.Lock()
	enabled := diskLoggingEnabled
	if !enabled {
		logMu.Unlock()
		return
	}

	filename := "debug.log"
	if env != "1" && env != "true" {
		filename = env
	}

	if !logRotated {
		if _, err := os.Stat(filename); err == nil {
			rotateLogs(filename)
		}
		logRotated = true
	}

	if logFile != nil && currentLogFilename != filename {
		logFile.Close()
		logFile = nil
	}

	if logFile == nil {
		var err error
		logFile, err = os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		currentLogFilename = filename
		recordLogMemory(fmt.Sprintf("[SYS] Opened new log file %q (Err: %v, PID: %d)", filename, err, os.Getpid()))
	}
	if logFile != nil {
		fmt.Fprintln(logFile, fullMsg)
		logFile.Sync()
	}
	logMu.Unlock()
}

func GetCurrentLogs() []string {
	return getMemLogs()
}
