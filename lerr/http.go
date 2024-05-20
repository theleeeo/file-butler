package lerr

import (
	"fmt"
	"net/http"
)

func ToHTTP(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(Code(err))
	fmt.Fprintln(w, err.Error())
}
