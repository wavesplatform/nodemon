package tools

import (
	"os"

	"github.com/pkg/errors"
	zapLogger "go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func SetupZapLogger(logLevel string) (*zapLogger.Logger, *zapLogger.AtomicLevel, error) {
	atom := zapLogger.NewAtomicLevel()
	encoderCfg := zapLogger.NewDevelopmentEncoderConfig()

	core := zapcore.NewCore(zapcore.NewConsoleEncoder(encoderCfg), zapcore.Lock(os.Stdout), atom)
	zap := zapLogger.New(core)

	level, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "invalid log level '%s'", logLevel)
	}
	atom.SetLevel(level)
	zapLogger.ReplaceGlobals(zap)

	return zap, &atom, nil
}
