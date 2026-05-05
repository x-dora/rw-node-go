package httpapi

import "net/http"

func ZstdMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO(M1): decode zstd-compressed JSON bodies.
		next.ServeHTTP(w, r)
	})
}
