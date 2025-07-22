package attrs

import (
	"log/slog"
)

// Error returns a slog.Attr that contains the error value with the key "error".
func Error(err error) slog.Attr {
	const key = "error"
	return slog.Any(key, err)
}
