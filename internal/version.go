package internal

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
)

var version = "v0.0.0"

// Version returns the version of the application.
func Version() string { return version }

func VersionHTTPHandler(w http.ResponseWriter, _ *http.Request) {
	type resp struct {
		Version string `json:"version"`
	}
	h := w.Header()
	h.Set("Content-Type", "application/json")
	h.Set("X-Content-Type-Options", "nosniff")
	if err := json.NewEncoder(w).Encode(resp{Version: Version()}); err != nil {
		msg := fmt.Sprintf("{\"error\":%q}\n", html.EscapeString(err.Error()))
		http.Error(w, msg, http.StatusInternalServerError)
		return
	}
}
