package attrs

import (
	"encoding"
	"fmt"
	"log/slog"
)

// Error returns a slog.Attr that contains the error value with the key "error".
func Error(err error) slog.Attr {
	const key = "error"
	return slog.Any(key, err)
}

// textMarshaler is a helper function that formats a value as a slog.Attr
// and checks if the value implements encoding.TextMarshaler.
func textMarshaler(key string, value encoding.TextMarshaler) slog.Attr { return slog.Any(key, value) }

type typenamePrinter struct{ v any }

func (t typenamePrinter) MarshalText() ([]byte, error) {
	return fmt.Appendf(nil, "%T", t.v), nil
}

// Type returns a slog.Attr that contains the type name of the value.
func Type(value any) slog.Attr {
	const key = "type"
	return textMarshaler(key, typenamePrinter{v: value})
}

type byteStringPrinter []byte

func (b byteStringPrinter) MarshalText() ([]byte, error) { return b, nil }

// ByteString returns a slog.Attr that contains the byte slice as a string.
func ByteString(key string, value []byte) slog.Attr {
	return textMarshaler(key, byteStringPrinter(value))
}
