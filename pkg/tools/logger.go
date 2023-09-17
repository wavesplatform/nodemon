package tools

import (
	"os"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func SetupZapLogger(logLevel string, development bool) (*zap.Logger, *zap.AtomicLevel, error) {
	level, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "invalid log level '%s'", logLevel)
	}

	var encoder zapcore.Encoder
	if development {
		encoderCfg := zap.NewDevelopmentEncoderConfig()
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	} else {
		encoderCfg := zap.NewProductionEncoderConfig()
		encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	}

	atom := zap.NewAtomicLevel()
	atom.SetLevel(level)
	core := zapcore.NewCore(encoder, zapcore.Lock(os.Stdout), atom)
	logger := zap.New(core)

	zap.ReplaceGlobals(logger)

	return logger, &atom, nil
}
