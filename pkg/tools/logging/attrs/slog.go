package attrs

import (
	"encoding"
	"encoding/base64"
	"fmt"
	"log/slog"
	"strings"
	"unicode/utf8"

	gl "github.com/wavesplatform/gowaves/pkg/logging"
)

// Error returns a slog.Attr that contains the error value with the key "error".
func Error(err error) slog.Attr {
	return gl.Error(err)
}

// textMarshaler is a helper function that formats a value as a slog.Attr
// and checks if the value implements encoding.TextMarshaler.
func textMarshaler(key string, value encoding.TextMarshaler) slog.Attr { return slog.Any(key, value) }

// Type returns a slog.Attr that contains the type name of the value.
func Type(value any) slog.Attr {
	return gl.Type(value)
}

type byteStringPrinter []byte

func (b byteStringPrinter) MarshalText() ([]byte, error) {
	if !utf8.Valid(b) { // Check if the byte slice is valid UTF-8
		return base64.StdEncoding.AppendEncode(nil, b), nil // Encode as base64 if not valid UTF-8
	}
	return b, nil // Return the byte slice as a string if it is valid UTF-8
}

// ByteString returns a slog.Attr that contains the byte slice as a string.
// If the byte slice is not valid UTF-8 string, it will be encoded as base64.
// This is useful for logging byte slices that may contain non-printable characters.
func ByteString(key string, value []byte) slog.Attr {
	return textMarshaler(key, byteStringPrinter(value))
}

// Stringer is a helper function that formats a value as a slog.Attr by calling slog.Any
// with the provided key and value. It is intended for use with values that implement fmt.Stringer.
func Stringer(key string, value fmt.Stringer) slog.Attr {
	return slog.Any(key, value) // can use slog.Any because value will be printed with fmt.Sprintf internally
}

type stringSlicePrinter []string

func (s stringSlicePrinter) MarshalText() ([]byte, error) {
	return []byte(strings.Join(s, ",")), nil
}

// Strings returns a slog.Attr that contains a slice of strings formatted as a comma-separated string.
func Strings(key string, value []string) slog.Attr {
	return textMarshaler(key, stringSlicePrinter(value))
}

type binaryPrinter []byte

func (b binaryPrinter) MarshalText() ([]byte, error) {
	return base64.StdEncoding.AppendEncode(nil, b), nil
}

// Binary returns a slog.Attr that contains a byte slice formatted as a base64-encoded string.
// This is useful for logging binary data in a human-readable format.
func Binary(key string, value []byte) slog.Attr {
	return textMarshaler(key, binaryPrinter(value))
}
