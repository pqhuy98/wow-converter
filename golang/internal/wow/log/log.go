// Package log provides lightweight logging helpers for WoW data operations.
package log

import (
	"fmt"
	"os"
	"time"
)

var enabled = os.Getenv("WOW_DATA_LOG") != "0"
var prefix = os.Getenv("WOW_LOG_PREFIX")

var timers []int64

// Write logs a formatted message when logging is enabled.
func Write(format string, args ...any) {
	if !enabled {
		return
	}
	if prefix == "" {
		prefix = "go"
	}
	fmt.Printf("[%s][wow] %s\n", prefix, fmt.Sprintf(format, args...))
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
