package internal_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"nodemon/internal"
)

func TestVersionHTTPHandler(t *testing.T) {
	r := httptest.NewRecorder()
	internal.VersionHTTPHandler(r, nil)
	assert.Equal(t, http.StatusOK, r.Code)
	h := r.Header()
	assert.Equal(t, "application/json", h.Get("Content-Type"))
	assert.Equal(t, "nosniff", h.Get("X-Content-Type-Options"))
	body := r.Body.Bytes()
	assert.True(t, json.Valid(body))
	assert.Equal(t, "{\"version\":\"v0.0.0\"}\n", string(body))
}
