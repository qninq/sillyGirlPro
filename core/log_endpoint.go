package core

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/qninq/sillyGirlPro/core/logs"
)

func initLogEndpoint() {
	// Register the web logger adapter so it captures all log output.
	_ = logs.SetLogger("web_logger", "{}")

	// SSE endpoint for streaming logs to the admin panel.
	// RequireAuth falls back to the token query parameter because
	// EventSource cannot set custom headers.
	GinApi(GET, "/api/admin/logs/stream", RequireAuth, func(c *gin.Context) {
		flusher, ok := c.Writer.(http.Flusher)
		if !ok {
			ApiError(c, http.StatusInternalServerError, "Streaming not supported")
			return
		}

		c.Header("Content-Type", "text/event-stream")
		c.Header("Cache-Control", "no-cache")
		c.Header("Connection", "keep-alive")
		c.Header("X-Accel-Buffering", "no")

		wl := logs.GetWebLogger()

		// Send recent logs as initial batch
		lines := 200
		if n := c.Query("lines"); n != "" {
			if v, err := strconv.Atoi(n); err == nil && v > 0 {
				lines = v
			}
		}
		recent := wl.GetRecentLogs(lines)
		for _, entry := range recent {
			_, _ = fmt.Fprintf(c.Writer, "data: %s [%s] %s\n\n", entry.Time, entry.Level, entry.Message)
		}
		flusher.Flush()

		// Subscribe to new log entries
		ch := wl.Subscribe()
		defer wl.Unsubscribe(ch)

		clientGone := c.Request.Context().Done()
		for {
			select {
			case <-clientGone:
				return
			case entry, ok := <-ch:
				if !ok {
					return
				}
				_, _ = fmt.Fprintf(c.Writer, "data: %s [%s] %s\n\n", entry.Time, entry.Level, entry.Message)
				flusher.Flush()
			case <-time.After(30 * time.Second):
				// Send keepalive comment to prevent proxy timeout
				_, _ = io.WriteString(c.Writer, ": keepalive\n\n")
				flusher.Flush()
			}
		}
	})
}