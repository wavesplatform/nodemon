package tools

import (
	"os"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func SetupZapLogger(logLevel string) (*zap.Logger, *zap.AtomicLevel, error) {
	atom := zap.NewAtomicLevel()
	encoderCfg := zap.NewDevelopmentEncoderConfig()

	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.Lock(os.Stdout), atom)
	logger := zap.New(core)

	level, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "invalid log level '%s'", logLevel)
	}
	atom.SetLevel(level)
	zap.ReplaceGlobals(logger)

	return logger, &atom, nil
}
