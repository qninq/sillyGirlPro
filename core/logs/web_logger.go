package logs

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// LogEntry represents a single log entry for the web UI.
type LogEntry struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

// WebLogger stores log messages in a ring buffer and notifies SSE subscribers.
type WebLogger struct {
	mu          sync.RWMutex
	buffer      []LogEntry
	maxBuffer   int
	subscribers []chan LogEntry
	formatter   LogFormatter
	Level       int `json:"level"`
}

var (
	webLoggerInstance *WebLogger
	webLoggerOnce     sync.Once
	webLevelNames     = []string{"EMERGENCY", "ALERT", "CRITICAL", "ERROR", "WARNING", "NOTICE", "INFO", "DEBUG"}
)

const webLoggerAdapterName = "web_logger"

// GetWebLogger returns the singleton WebLogger instance.
func GetWebLogger() *WebLogger {
	webLoggerOnce.Do(func() {
		webLoggerInstance = &WebLogger{
			buffer:    make([]LogEntry, 0, 1000),
			maxBuffer: 1000,
			Level:     LevelDebug,
		}
		webLoggerInstance.formatter = webLoggerInstance
	})
	return webLoggerInstance
}

// GetRecentLogs returns the last n log entries from the buffer.
func (w *WebLogger) GetRecentLogs(n int) []LogEntry {
	w.mu.RLock()
	defer w.mu.RUnlock()
	if n <= 0 || n >= len(w.buffer) {
		result := make([]LogEntry, len(w.buffer))
		copy(result, w.buffer)
		return result
	}
	result := make([]LogEntry, n)
	copy(result, w.buffer[len(w.buffer)-n:])
	return result
}

// Subscribe returns a channel that receives new log entries.
func (w *WebLogger) Subscribe() chan LogEntry {
	w.mu.Lock()
	defer w.mu.Unlock()
	ch := make(chan LogEntry, 256)
	w.subscribers = append(w.subscribers, ch)
	return ch
}

// Unsubscribe removes a subscriber channel.
func (w *WebLogger) Unsubscribe(ch chan LogEntry) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for i, sub := range w.subscribers {
		if sub == ch {
			w.subscribers = append(w.subscribers[:i], w.subscribers[i+1:]...)
			close(ch)
			return
		}
	}
}

func (w *WebLogger) write(level int, msg string) {
	entry := LogEntry{
		Time:    time.Now().Format("2006-01-02 15:04:05.000"),
		Level:   levelName(level),
		Message: msg,
	}

	w.mu.Lock()
	// Append to ring buffer
	if len(w.buffer) >= w.maxBuffer {
		w.buffer = append(w.buffer[1:], entry)
	} else {
		w.buffer = append(w.buffer, entry)
	}
	// Notify subscribers (non-blocking)
	for _, ch := range w.subscribers {
		select {
		case ch <- entry:
		default:
			// drop if subscriber is too slow
		}
	}
	w.mu.Unlock()
}

func levelName(level int) string {
	if level >= 0 && level < len(webLevelNames) {
		return webLevelNames[level]
	}
	return fmt.Sprintf("LEVEL(%d)", level)
}

// Logger interface implementation

func (w *WebLogger) Init(config string) error {
	if len(config) == 0 || config == "{}" {
		return nil
	}
	return json.Unmarshal([]byte(config), w)
}

func (w *WebLogger) WriteMsg(lm *LogMsg) error {
	if lm.Level > w.Level {
		return nil
	}
	msg := w.formatter.Format(lm)
	w.write(lm.Level, msg)
	return nil
}

func (w *WebLogger) Destroy() {}

func (w *WebLogger) Flush() {}

func (w *WebLogger) SetFormatter(f LogFormatter) {
	w.formatter = f
}

// Format formats a LogMsg for the web logger (plain text, no colors).
func (w *WebLogger) Format(lm *LogMsg) string {
	return lm.OldStyleFormat()
}

func init() {
	Register(webLoggerAdapterName, func() Logger {
		return GetWebLogger()
	})
}