package entities

import (
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

const (
	HTTPScheme  = "http"
	HTTPSScheme = "https"
)

type Node struct {
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
	Alias   string `json:"alias"`
}

func CheckAndUpdateURL(s string) (string, error) {
	var u *url.URL
	var err error
	if strings.Contains(s, "//") {
		u, err = url.Parse(s)
	} else {
		u, err = url.Parse("//" + s)
	}
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse URL %s", s)
	}
	if u.Scheme == "" {
		u.Scheme = HTTPScheme
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.Errorf("unsupported URL scheme %s", u.Scheme)
	}
	return u.String(), nil
}

type NodeHeight struct {
	URL    string `json:"url"`
	Height int    `json:"height"`
}
