// Package log provides lightweight logging helpers for WoW data operations.
package log

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var enabled = os.Getenv("WOW_DATA_LOG") != "0"
var prefix = os.Getenv("WOW_LOG_PREFIX")

var timers []int64

var (
	loadingMu              sync.RWMutex
	loadingDepth           int
	latestLoadingMessage   string
)

// BeginLoadingProgress marks the start of a CASC load; Write updates the latest UI message while active.
func BeginLoadingProgress() {
	loadingMu.Lock()
	defer loadingMu.Unlock()
	loadingDepth++
	if loadingDepth == 1 {
		latestLoadingMessage = ""
	}
}

// EndLoadingProgress marks the end of a CASC load started with BeginLoadingProgress.
func EndLoadingProgress() {
	loadingMu.Lock()
	defer loadingMu.Unlock()
	if loadingDepth > 0 {
		loadingDepth--
	}
}

// IsLoadingProgressActive reports whether a CASC load is in progress.
func IsLoadingProgressActive() bool {
	loadingMu.RLock()
	defer loadingMu.RUnlock()
	return loadingDepth > 0
}

// LatestLoadingMessage returns the most recent log line emitted during an active CASC load.
func LatestLoadingMessage() string {
	loadingMu.RLock()
	defer loadingMu.RUnlock()
	return latestLoadingMessage
}

func noteLoadingMessage(msg string) {
	loadingMu.Lock()
	defer loadingMu.Unlock()
	if loadingDepth > 0 {
		latestLoadingMessage = msg
	}
}

// Write logs a formatted message when logging is enabled.
func Write(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	noteLoadingMessage(msg)
	if !enabled {
		return
	}
	if prefix == "" {
		prefix = "go"
	}
	fmt.Printf("[%s][wow] %s\n", prefix, msg)
}

// TimeLog starts a timer.
func TimeLog() {
	timers = append(timers, time.Now().UnixMilli())
}

// TimeEnd ends the most recent timer and logs elapsed milliseconds.
func TimeEnd(format string, args ...any) {
	var start int64
	if len(timers) > 0 {
		start = timers[len(timers)-1]
		timers = timers[:len(timers)-1]
	}
	elapsed := int64(-1)
	if start >= 0 {
		elapsed = time.Now().UnixMilli() - start
	}
	if format != "" {
		Write(format+" (took %dms)", append(args, elapsed)...)
	} else {
		Write("Timer ended (took %dms)", elapsed)
	}
}
