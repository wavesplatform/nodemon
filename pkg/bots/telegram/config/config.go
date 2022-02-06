package config

import (
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"os"
)

const (
	EnvPort          = "BOT_ADDRESS"
	EnvBotToken      = "BOT_TOKEN"
	EnvBotWebhookUrl = "BOT_WEBHOOK_URL"
)

var (
	InvalidParameters = errors.New("invalid parameters ")
)

type BotConfig struct {
	port           string // default is 8081
	botToken       string
	webHookAddress string
}

func LoadAppConfig() (*BotConfig, error) {
	port := os.Getenv(EnvPort)
	if port == "" {
		port = ":8081"
	}
	botToken := os.Getenv(EnvBotToken)
	if botToken == "" {
		return nil, InvalidParameters
	}
	webHookAddress := os.Getenv(EnvBotWebhookUrl)

	return &BotConfig{port: port, botToken: botToken, webHookAddress: webHookAddress}, nil

}

func (config *BotConfig) Port() string {
	return config.port
}

func (config *BotConfig) BotToken() string {
	return config.botToken
}

func (config *BotConfig) WebHookAddress() string {
	return config.webHookAddress
}


func InitLog() error {
	var zapLoglevel zapcore.Level
	if err := zapLoglevel.Set("info"); err != nil {
		return errors.Wrapf(err, "failed to  parse set log level")
	}
	zapLogConfig := zap.NewProductionConfig()
	zapLogConfig.Level.SetLevel(zapLoglevel)

	logger, err := zapLogConfig.Build()
	if err != nil {
		return errors.Wrapf(err, "failed to build logger")
	}
	zap.ReplaceGlobals(logger)

	return nil
}
