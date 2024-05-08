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
				responseId := rand.Intn(1000000)

				copyHeaders(w.Header(), respCatcher.Header())
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(fmt.Sprintf("internal error, id: %d", responseId)))

				log.Printf("internal error: %s, id: %d, path: %s", removeTrailingNewline(respCatcher.Body.String()), responseId, r.URL.Path)
				return
			}

			copyHeaders(w.Header(), respCatcher.Header())
			w.WriteHeader(respCatcher.Code)
			w.Write(respCatcher.Body.Bytes())
		})
	}
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
