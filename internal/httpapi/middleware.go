package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

func LimitBody(next http.Handler, limit int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if limit > 0 && r.Body != nil {
			r.Body = http.MaxBytesReader(w, r.Body, limit)
		}
		next.ServeHTTP(w, r)
	})
}

func InternalTokenMiddleware(next http.Handler, token string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if token != "" && r.URL.Query().Get("token") != token {
			closeRequestConnection(w, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

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

func Recover(next http.Handler, logger *slog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic in request", "method", r.Method, "path", r.URL.Path, "panic", recovered)
				WriteHTTPEnvelope(w, http.StatusInternalServerError, map[string]any{
					"success": false,
					"error":   "internal server error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
