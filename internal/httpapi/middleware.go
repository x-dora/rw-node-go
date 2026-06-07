package httpapi

import (
	"log/slog"
	"net"
	"net/http"

	"github.com/gin-gonic/gin"
)

func closeRequestConnection(w http.ResponseWriter, fallbackStatus int) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, http.StatusText(fallbackStatus), fallbackStatus)
		return
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, http.StatusText(fallbackStatus), fallbackStatus)
		return
	}
	_ = conn.Close()
}

func ginBodyLimit(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if limit > 0 && c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
		}
		c.Next()
	}
}

func ginInternalPortGuard(expectedPort int) gin.HandlerFunc {
	return func(c *gin.Context) {
		localAddr := c.Request.Context().Value(http.LocalAddrContextKey)
		tcpAddr, ok := localAddr.(*net.TCPAddr)
		if !ok || tcpAddr.Port != expectedPort || !tcpAddr.IP.IsLoopback() {
			closeRequestConnection(c.Writer, http.StatusNotFound)
			c.Abort()
			return
		}
		c.Next()
	}
}

func ginRecovery(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic in request", "method", c.Request.Method, "path", c.Request.URL.Path, "panic", recovered)
				WriteEnvelope(c, http.StatusInternalServerError, map[string]any{
					"success": false,
					"error":   "internal server error",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}
