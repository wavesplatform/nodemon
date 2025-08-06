package logging

import (
	"fmt"
	"log/slog"

	gl "github.com/wavesplatform/gowaves/pkg/logging"
)

const NamespaceKey = gl.NamespaceKey

// SetupLogger initializes a global logger with the specified log level and type.
// It returns a pointer to the created logger and an error if any issues occur during setup.
// The log level and type are expected to be valid strings that can be unmarshaled
// into [slog.Level] and [gl.LoggerType] respectively.
func SetupLogger(logLevel, logType string) (*slog.Logger, error) {
	var logL slog.Level
	if err := logL.UnmarshalText([]byte(logLevel)); err != nil {
		return nil, fmt.Errorf("invalid log level: %w", err)
	}
	var logT gl.LoggerType
	if err := logT.UnmarshalText([]byte(logType)); err != nil {
		return nil, fmt.Errorf("invalid logger type: %w", err)
	}
	h := gl.NewHandler(logT, logL)
	logger := slog.New(h)
	slog.SetDefault(logger)
	return logger, nil
}
