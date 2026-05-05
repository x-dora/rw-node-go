package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Response any `json:"response"`
}

func WriteEnvelope(c *gin.Context, status int, response any) {
	c.JSON(status, Envelope{Response: response})
}

func WriteJSON(c *gin.Context, status int, response any) {
	c.JSON(status, response)
}

func WriteHTTPEnvelope(w http.ResponseWriter, status int, response any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Envelope{Response: response})
}
