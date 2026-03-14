package wallpaper

import (
	"fmt"
	"io"
	"sync"
)

type Logger struct {
	level int
	out   io.Writer
	err   io.Writer
	mu    sync.Mutex
}

const (
	logLevelError = iota
	logLevelInfo
	logLevelDebug
)

func NewLogger(level string, out, err io.Writer) *Logger {
	return &Logger{
		level: parseLogLevel(level),
		out:   out,
		err:   err,
	}
}

func parseLogLevel(level string) int {
	switch normalizeLogLevel(level) {
	case "debug":
		return logLevelDebug
	case "error":
		return logLevelError
	default:
		return logLevelInfo
	}
}

func (l *Logger) Debugf(format string, args ...any) {
	if l.level < logLevelDebug {
		return
	}
	l.printf(l.out, "DEBUG", format, args...)
}

func (l *Logger) Infof(format string, args ...any) {
	if l.level < logLevelInfo {
		return
	}
	l.printf(l.out, "INFO", format, args...)
}

func (l *Logger) Errorf(format string, args ...any) {
	l.printf(l.err, "ERROR", format, args...)
}

func (l *Logger) printf(w io.Writer, level, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fmt.Fprintf(w, "[eumetsat-wallpaper] %s: %s\n", level, fmt.Sprintf(format, args...))
}
