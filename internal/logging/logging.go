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
	fmt.Fprintf(l.f, "%s %s exec %s %v output=%q err=%v\n", ts, label, cmd, args, output, err)
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
