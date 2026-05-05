package httpapi

import "net/http"

func JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO(M1): validate Authorization Bearer token with RS256 and SECRET_KEY jwtPublicKey.
		next.ServeHTTP(w, r)
	})
}
