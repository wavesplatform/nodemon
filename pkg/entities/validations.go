package entities

import (
	"net/url"
	"strings"

	"github.com/pkg/errors"
)

const (
	defaultScheme = "http"
)

func ValidateNodeURL(s string) (string, error) {
	var u *url.URL
	var err error
	if strings.Contains(s, "//") {
		u, err = url.Parse(s)
	} else {
		u, err = url.Parse("//" + s)
	}
	if err != nil {
		return "", err
	}
	if u.Scheme == "" {
		u.Scheme = defaultScheme
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.Errorf("unsupported URL scheme '%s'", u.Scheme)
	}
	return u.String(), nil
}
