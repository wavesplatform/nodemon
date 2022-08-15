package entities

import (
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

const (
	HttpScheme = "http"
)

type Node struct {
	URL     string `json:"url"`
	Enabled bool   `json:"enabled"`
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
		u.Scheme = HttpScheme
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.Errorf("unsupported URL scheme %s", u.Scheme)
	}
	return u.String(), nil
}
