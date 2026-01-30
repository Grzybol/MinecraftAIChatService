package logging

import (
	"io"
	"log"
	"os"
	"strings"
	"sync/atomic"
)

type Level int32

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarning
	LevelError
	LevelException
)

var currentLevel int32 = int32(LevelInfo)

func SetLevel(level Level) {
	atomic.StoreInt32(&currentLevel, int32(level))
}

func SetLevelFromEnv(key string) Level {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		SetLevel(LevelInfo)
		return LevelInfo
	}
	if level, ok := ParseLevel(raw); ok {
		SetLevel(level)
		return level
	}
	SetLevel(LevelInfo)
	return LevelInfo
}

func ParseLevel(raw string) (Level, bool) {
	switch strings.ToUpper(strings.TrimSpace(raw)) {
	case "DEBUG":
		return LevelDebug, true
	case "INFO":
		return LevelInfo, true
	case "WARN", "WARNING":
		return LevelWarning, true
	case "ERROR", "ERRO":
		return LevelError, true
	case "EXCEPTION":
		return LevelException, true
	default:
		return LevelInfo, false
	}
}

func Enabled(level Level) bool {
	return level >= Level(atomic.LoadInt32(&currentLevel))
}

func Debugf(format string, args ...any) {
	logf(LevelDebug, format, args...)
}

func Infof(format string, args ...any) {
	logf(LevelInfo, format, args...)
}

func Warnf(format string, args ...any) {
	logf(LevelWarning, format, args...)
}

func Errorf(format string, args ...any) {
	logf(LevelError, format, args...)
}

func Exceptionf(format string, args ...any) {
	logf(LevelException, format, args...)
}

func Fatalf(format string, args ...any) {
	log.Printf("[ERROR] "+format, args...)
	os.Exit(1)
}

func logf(level Level, format string, args ...any) {
	if !Enabled(level) {
		return
	}
	log.Printf("[%s] "+format, append([]any{level.String()}, args...)...)
}

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelInfo:
		return "INFO"
	case LevelWarning:
		return "WARNING"
	case LevelError:
		return "ERROR"
	case LevelException:
		return "EXCEPTION"
	default:
		return "INFO"
	}
}

type splitWriter struct {
	stdout      io.Writer
	stdoutLevel Level
	file        io.Writer
	fileLevel   Level
}

func NewSplitWriter(stdout io.Writer, stdoutLevel Level, file io.Writer, fileLevel Level) io.Writer {
	return &splitWriter{
		stdout:      stdout,
		stdoutLevel: stdoutLevel,
		file:        file,
		fileLevel:   fileLevel,
	}
}

func (w *splitWriter) Write(p []byte) (int, error) {
	line := strings.TrimSpace(string(p))
	level := parseLevelFromLine(line)
	if level >= w.stdoutLevel {
		_, _ = w.stdout.Write(p)
	}
	if level >= w.fileLevel {
		_, _ = w.file.Write(p)
	}
	return len(p), nil
}

func parseLevelFromLine(line string) Level {
	start := strings.Index(line, "[")
	if start == -1 {
		return LevelInfo
	}
	end := strings.Index(line[start+1:], "]")
	if end == -1 {
		return LevelInfo
	}
	levelText := line[start+1 : start+1+end]
	if level, ok := ParseLevel(levelText); ok {
		return level
	}
	return LevelInfo
}
