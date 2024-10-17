package internal

import (
	"fmt"
	"net/http"
)

var version = "v0.0.0"

// Version returns the version of the application.
func Version() string { return version }

func VersionHTTPHandler(w http.ResponseWriter, _ *http.Request) {
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("X-Content-Type-Options", "nosniff")
	if _, err := fmt.Fprintf(w, "{\"version\":%q}\n", Version()); err != nil {
		http.Error(w, fmt.Sprintf("{\"error\":%q}\n", err.Error()), http.StatusInternalServerError)
		return
	}
}
