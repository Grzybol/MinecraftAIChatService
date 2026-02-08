package logging

import (
	"io"
	"strings"
	"time"
)

const logTimeLayout = "2006/01/02 15:04:05.000000"

type elasticWriter struct {
	logger   *ElasticLogger
	minLevel Level
}

func NewElasticWriter(logger *ElasticLogger, minLevel Level) io.Writer {
	return &elasticWriter{logger: logger, minLevel: minLevel}
}

func (w *elasticWriter) Write(p []byte) (int, error) {
	if w.logger == nil {
		return len(p), nil
	}
	line := strings.TrimSpace(string(p))
	if line == "" {
		return len(p), nil
	}
	timestamp, remainder := parseLogTimestamp(line)
	level := parseLevelFromLine(remainder)
	if level < w.minLevel {
		return len(p), nil
	}
	message := parseMessageFromLine(remainder)
	fields := parseFieldsFromMessage(message)
	if value, ok := fields["transaction_id"].(string); ok {
		fields["transactionID"] = value
	}
	w.logger.Enqueue(logEntry{
		Timestamp: timestamp,
		Level:     level.String(),
		Message:   message,
		Fields:    fields,
	})
	return len(p), nil
}

func parseLogTimestamp(line string) (time.Time, string) {
	if len(line) >= len(logTimeLayout) {
		prefix := line[:len(logTimeLayout)]
		if ts, err := time.Parse(logTimeLayout, prefix); err == nil {
			return ts, strings.TrimSpace(line[len(logTimeLayout):])
		}
	}
	return time.Now().UTC(), line
}

func parseMessageFromLine(line string) string {
	start := strings.Index(line, "[")
	if start == -1 {
		return strings.TrimSpace(line)
	}
	end := strings.Index(line[start+1:], "]")
	if end == -1 {
		return strings.TrimSpace(line)
	}
	return strings.TrimSpace(line[start+1+end+1:])
}

func parseFieldsFromMessage(message string) map[string]interface{} {
	fields := make(map[string]interface{})
	for _, token := range strings.Fields(message) {
		parts := strings.SplitN(token, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		if key == "" {
			continue
		}
		value := strings.Trim(parts[1], ",")
		value = strings.Trim(value, "\"")
		fields[key] = value
	}
	fields["message"] = message
	return fields
}
