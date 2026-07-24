package logging

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Level int

const (
	LevelError Level = iota
	LevelWarn
	LevelInfo
	LevelTrace
)

func ParseLevel(s string) (Level, error) {
	switch strings.ToUpper(s) {
	case "ERROR":
		return LevelError, nil
	case "WARN":
		return LevelWarn, nil
	case "INFO":
		return LevelInfo, nil
	case "TRACE":
		return LevelTrace, nil
	default:
		return LevelInfo, fmt.Errorf("unknown log level %q", s)
	}
}

type Logger struct {
	level Level
	f     *os.File
}

func New(level Level) (*Logger, error) {
	f, err := os.OpenFile("simmer.log", os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return &Logger{level: level, f: f}, nil
}

// Writer returns the underlying log file so external loggers (e.g. logrus) can
// be redirected into the same file. Returns nil when the logger is nil.
func (l *Logger) Writer() *os.File {
	if l == nil {
		return nil
	}
	return l.f
}

func (l *Logger) Close() {
	if l != nil && l.f != nil {
		l.f.Close()
	}
}

func (l *Logger) LogExec(cmd string, args []string, output string, err error) {
	if l == nil {
		return
	}
	lvl := LevelInfo
	if err != nil {
		lvl = LevelError
	}
	if lvl > l.level {
		return
	}
	label := "INFO"
	if err != nil {
		label = "ERROR"
	}
	ts := time.Now().Format(time.RFC3339)
	if l.level >= LevelTrace {
		fmt.Fprintf(l.f, "%s %s exec %s %v output=%q err=%v\n", ts, label, cmd, args, output, err)
	} else {
		fmt.Fprintf(l.f, "%s %s exec %s %v err=%v\n", ts, label, cmd, args, err)
	}
}

func (l *Logger) LogStart(cmd string, args []string, err error) {
	if l == nil {
		return
	}
	lvl := LevelInfo
	if err != nil {
		lvl = LevelError
	}
	if lvl > l.level {
		return
	}
	label := "INFO"
	if err != nil {
		label = "ERROR"
	}
	ts := time.Now().Format(time.RFC3339)
	fmt.Fprintf(l.f, "%s %s start %s %v err=%v\n", ts, label, cmd, args, err)
}

// LogError logs a non-exec failure (e.g. a resolution or parsing error) with
// a short op label for context. No-op when err is nil.
func (l *Logger) LogError(op string, err error) {
	if l == nil || err == nil || LevelError > l.level {
		return
	}
	ts := time.Now().Format(time.RFC3339)
	fmt.Fprintf(l.f, "%s ERROR %s err=%v\n", ts, op, err)
}
