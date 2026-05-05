package httpapi

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/klauspost/compress/zstd"
)

func ZstdMiddleware(next http.Handler) http.Handler {
	return ZstdMiddlewareWithLimit(next, 0)
}

func ZstdMiddlewareWithLimit(next http.Handler, limit int64) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.EqualFold(r.Header.Get("Content-Encoding"), "zstd") {
			decoder, err := zstd.NewReader(r.Body)
			if err != nil {
				http.Error(w, fmt.Sprintf("decode zstd body: %v", err), http.StatusBadRequest)
				return
			}
			defer decoder.Close()
			reader := io.Reader(decoder)
			if limit > 0 {
				reader = io.LimitReader(decoder, limit+1)
			}
			body, err := io.ReadAll(reader)
			if err != nil {
				http.Error(w, fmt.Sprintf("decode zstd body: %v", err), http.StatusBadRequest)
				return
			}
			if limit > 0 && int64(len(body)) > limit {
				http.Error(w, http.StatusText(http.StatusRequestEntityTooLarge), http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			r.Header.Del("Content-Encoding")
		}
		next.ServeHTTP(w, r)
	})
}
