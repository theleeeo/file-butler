package server

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"net/http/httptest"
)

// InternalErrorRedacter is a middleware that will redact internal error messages.
// It will replace the response body with a generic message and an id and log the original message.
func InternalErrorRedacter() func(http.Handler) http.Handler {
	return func(h http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			respCatcher := httptest.NewRecorder()
			h.ServeHTTP(respCatcher, r)

			if respCatcher.Code == http.StatusInternalServerError {
				responseId := rand.Intn(1000000) //nolint:gosec // This is not for security purposes, it does not have to be cryptographically secure

				copyHeaders(w.Header(), respCatcher.Header())
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(fmt.Sprintf("internal error, id: %d", responseId)))

				log.Printf("internal error: %s, id: %d, path: %s", removeTrailingNewline(respCatcher.Body.String()), responseId, r.URL.Path)
				return
			}

			copyHeaders(w.Header(), respCatcher.Header())
			w.WriteHeader(respCatcher.Code)
			_, _ = w.Write(respCatcher.Body.Bytes())
		})
	}
}

func CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func removeTrailingNewline(m string) string {
	if len(m) > 0 && m[len(m)-1] == '\n' {
		return m[:len(m)-1]
	}
	return m
}

func copyHeaders(dst, src http.Header) {
	for k, v := range src {
		dst[k] = v
	}
}
