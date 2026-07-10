package exportlog

import (
	"io"
	"log"
	"regexp"
	"strings"
	"sync"
)

const maxLines = 2

var (
	mu         sync.Mutex
	lines      []string
	active     bool
	underlying io.Writer
	logPrefix  = regexp.MustCompile(`^\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}(?:\.\d+)? `)
)

type captureWriter struct{}

func (captureWriter) Write(p []byte) (int, error) {
	appendLine(stripLogPrefix(string(p)))
	return underlying.Write(p)
}

func stripLogPrefix(msg string) string {
	msg = strings.TrimRight(msg, "\r\n")
	return logPrefix.ReplaceAllString(msg, "")
}

func appendLine(line string) {
	if line == "" {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if !active {
		return
	}
	lines = append(lines, line)
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
}

// Install hooks the default logger so export jobs can stream recent lines to the UI.
func Install() {
	underlying = log.Writer()
	log.SetOutput(captureWriter{})
}

// Begin clears the rolling buffer and starts capturing log lines for the active export job.
func Begin() {
	mu.Lock()
	lines = nil
	active = true
	mu.Unlock()
}

// End stops capturing log lines for the active export job.
func End() {
	mu.Lock()
	active = false
	mu.Unlock()
}

// Snapshot returns the most recent captured log lines.
func Snapshot() []string {
	mu.Lock()
	defer mu.Unlock()
	if len(lines) == 0 {
		return []string{}
	}
	out := make([]string, len(lines))
	copy(out, lines)
	return out
}
