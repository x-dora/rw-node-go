package httpapi

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Response any `json:"response"`
}

type ErrorEnvelope struct {
	Timestamp string `json:"timestamp"`
	Path      string `json:"path"`
	Message   string `json:"message"`
	ErrorCode string `json:"errorCode"`
}

func WriteEnvelope(c *gin.Context, status int, response any) {
	c.JSON(status, Envelope{Response: response})
}

func WriteJSON(c *gin.Context, status int, response any) {
	c.JSON(status, response)
}

func WriteOfficialError(c *gin.Context, status int, message string, code string) {
	c.JSON(status, ErrorEnvelope{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Path:      c.Request.URL.RequestURI(),
		Message:   message,
		ErrorCode: code,
	})
}

func WriteHTTPEnvelope(w http.ResponseWriter, status int, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Response: response})
}
