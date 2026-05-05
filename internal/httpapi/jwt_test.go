package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"github.com/x-dora/rw-node-go/internal/testkit"
)

func TestJWTMiddleware(t *testing.T) {
	bundle := testkit.NewCertBundle(t)
	publicKey, err := ParseJWTPublicKey(bundle.Payload.JWTPublicKey)
	if err != nil {
		t.Fatalf("ParseJWTPublicKey() error = %v", err)
	}

	handler := JWTMiddleware(publicKey)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	t.Run("missing bearer", func(t *testing.T) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("valid RS256", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+testkit.NewRS256Token(t, bundle.JWTPrivateKey))
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}
	})

	t.Run("rejects HS256", func(t *testing.T) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "panel"})
		signed, err := token.SignedString([]byte("secret"))
		if err != nil {
			t.Fatalf("sign HS256 token: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+signed)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})
}
