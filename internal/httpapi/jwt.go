package httpapi

import (
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

func JWTMiddleware(publicKey *rsa.PublicKey) func(http.Handler) http.Handler {
	return JWTMiddlewareWithExemptPaths(publicKey, nil)
}

func JWTMiddlewareWithExemptPaths(publicKey *rsa.PublicKey, exemptPaths map[string]struct{}) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := exemptPaths[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}
			tokenValue, ok := bearerToken(r.Header.Get("Authorization"))
			if !ok {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			token, err := jwt.Parse(tokenValue, func(token *jwt.Token) (any, error) {
				if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok || token.Method.Alg() != jwt.SigningMethodRS256.Alg() {
					return nil, fmt.Errorf("unexpected signing method: %s", token.Header["alg"])
				}
				return publicKey, nil
			})
			if err != nil || token == nil || !token.Valid {
				http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func ParseJWTPublicKey(pemValue string) (*rsa.PublicKey, error) {
	key, err := jwt.ParseRSAPublicKeyFromPEM([]byte(pemValue))
	if err != nil {
		return nil, fmt.Errorf("parse jwt public key: %w", err)
	}
	return key, nil
}

func bearerToken(header string) (string, bool) {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

func LegacyJWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}
