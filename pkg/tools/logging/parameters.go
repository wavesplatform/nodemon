package logging

import (
	"fmt"
	"log/slog"

	gl "github.com/wavesplatform/gowaves/pkg/logging"
)

const NamespaceKey = gl.NamespaceKey

type ParametersFlags struct {
	LogLevel   string
	LoggerType string
}

type Parameters struct {
	Level slog.Level
	Type  gl.LoggerType
}

func (p *ParametersFlags) String() string {
	return fmt.Sprintf("{Level: %s, Type: %s}", p.LogLevel, p.LoggerType)
}

func ParametersFromFlags(flags ParametersFlags) (Parameters, error) {
	p := &parameters{
		flagLogLevel:   flags.LogLevel,
		flagLoggerType: flags.LoggerType,
	}
	if err := p.Parse(); err != nil {
		return Parameters{}, fmt.Errorf("failed to parse logger parameters: %w", err)
	}
	return Parameters{
		Level: p.Level,
		Type:  p.Type,
	}, nil
}

type parameters struct {
	Level slog.Level
	Type  gl.LoggerType

	flagLogLevel   string
	flagLoggerType string
}

// Parse parses the command line parameters for logging.
func (p *parameters) Parse() error {
	var err error
	p.Level, err = p.parseLevel(p.flagLogLevel)
	if err != nil {
		return fmt.Errorf("failed to parse logger parameters: %w", err)
	}
	p.Type, err = p.parseType(p.flagLoggerType)
	if err != nil {
		return fmt.Errorf("failed to parse logger parameters: %w", err)
	}
	return nil
}

func (p *parameters) parseLevel(l string) (slog.Level, error) {
	var level slog.Level
	err := level.UnmarshalText([]byte(l))
	if err != nil {
		return slog.Level(0), fmt.Errorf("invalid log level: %w", err)
	}
	return level, nil
}

func (p *parameters) parseType(t string) (gl.LoggerType, error) {
	var lt gl.LoggerType
	err := lt.UnmarshalText([]byte(t))
	if err != nil {
		return gl.LoggerText, fmt.Errorf("invalid logger type: %w", err)
	}
	return lt, nil
}
