package scraping

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestJoinPath(t *testing.T) {
	tests := []struct {
		base string
		part string
		url  string
	}{
		{"http://some-url.biz/4/l/1/", "/new/part/", "http://some-url.biz/4/l/1/new/part"},
		{"http://some-url.biz/432/l/1", "new/part/", "http://some-url.biz/432/l/1/new/part"},
		{"http://some-url.biz/4/ldgdf/1", "new/part/", "http://some-url.biz/4/ldgdf/1/new/part"},
		{"some-url.biz/4/l/1/QWE", "new/part/", "some-url.biz/4/l/1/QWE/new/part"},
	}
	for _, tc := range tests {
		url, err := joinPath(tc.base, tc.part)
		assert.NoError(t, err)
		assert.Equal(t, tc.url, url.String())
	}
}
