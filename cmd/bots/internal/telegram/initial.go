package telegram

import (
	"net/http"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/telebot.v3"

	"nodemon/cmd/bots/internal/common"
	"nodemon/pkg/messaging/pair"
)

const (
	PollingMethod = "polling"
	WebhookMethod = "webhook"
)

const (
	defaultTelegramHTTPClientTimeout = 15 * time.Second
	longPollingTimeout               = 10 * time.Second
)

func InitBot(behavior string,
	deleteWebhook bool,
	webhookLocalAddress string,
	webhookPublicURL string,
	botToken string,
	chatID int64,
	logger *zap.Logger,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
) (*common.TelegramBotEnvironment, error) {
	defaultBotSettings := telebot.Settings{
		Token:   botToken,
		OnError: createOnErrorHandler(logger),
		Client:  &http.Client{Timeout: defaultTelegramHTTPClientTimeout},
	}
	bot, err := telebot.NewBot(defaultBotSettings)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start telegram bot")
	}
	botPoller, err := configureBotPoller(behavior, deleteWebhook, webhookLocalAddress, webhookPublicURL, bot, logger)
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure bot poller")
	}
	logger.Debug("Bot poller successfully set up",
		zap.String("behavior", behavior),
		zap.String("webhook-local-address", webhookLocalAddress),
		zap.String("webhook-public-url", webhookPublicURL),
	)
	bot.Poller = botPoller

	logger.Sugar().Debugf("Telegram chat id for sending alerts is %d", chatID)

	tgBotEnv := common.NewTelegramBotEnvironment(bot, chatID, false, logger, requestType, responsePairType)
	return tgBotEnv, nil
}

func createOnErrorHandler(logger *zap.Logger) func(error, telebot.Context) {
	return func(err error, teleCtx telebot.Context) {
		var (
			updateID       = teleCtx.Update().ID
			messageID      *int
			messageText    *string
			senderID       *int64
			senderUsername *string
		)
		if msg := teleCtx.Message(); msg != nil {
			messageID = &msg.ID
			messageText = &msg.Text // log injection here is possible, but we rely on telegram data correctness
		}
		if sender := teleCtx.Sender(); sender != nil {
			senderID = &sender.ID
			senderUsername = &sender.Username // log injection here is possible, but we rely on telegram data correctness
		}
		logger.Error("Unknown error has been occurred in a bot handler",
			zap.Int("update_id", updateID),
			zap.Intp("message_id", messageID),
			zap.Stringp("message_text", messageText),
			zap.Int64p("sender_id", senderID),
			zap.Stringp("sender_username", senderUsername),
			zap.Error(err),
		)
	}
}

func configureBotPoller(
	behavior string,
	deleteWebhook bool,
	webhookLocalAddress string,
	webhookPublicURL string,
	bot *telebot.Bot,
	logger *zap.Logger,
) (telebot.Poller, error) {
	existingWebhook, getErr := getExistingWebhook(bot, deleteWebhook, logger)
	if getErr != nil {
		return nil, errors.Wrap(getErr, "failed to get or clear existing webhook")
	}
	switch behavior {
	case PollingMethod:
		if url := existingWebhook.Listen; url != "" {
			return nil, errors.Errorf("failed to run bot in polling mode, webhook is set up for URL '%s'", url)
		}
		// we can set up a new long poller
		return &telebot.LongPoller{Timeout: longPollingTimeout}, nil
	case WebhookMethod:
		// here we assume that publicURL param is not emtpy
		switch url := existingWebhook.Listen; {
		case url == "": // we can set up a new webhook poller
			newWebhook := &telebot.Webhook{
				Listen:   webhookLocalAddress,
				Endpoint: &telebot.WebhookEndpoint{PublicURL: webhookPublicURL},
			}
			if setErr := bot.SetWebhook(newWebhook); setErr != nil {
				return nil, errors.Wrapf(setErr, "failed to set new webhook for URL '%s'", webhookPublicURL)
			}
			return newWebhook, nil
		case url == webhookPublicURL:
			return existingWebhook, nil // use existing settings
		case url != webhookPublicURL:
			return nil, errors.Errorf("failed to run bot in webhook mode, webhook is already set up for URL '%s'", url)
		default:
			panic("BUG, CREATE REPORT: unreachable point reached")
		}
	default:
		return nil, errors.Errorf("wrong type of bot behavior '%s' was provided", behavior)
	}
}

func getExistingWebhook(bot *telebot.Bot, deleteWebhook bool, logger *zap.Logger) (*telebot.Webhook, error) {
	existingWebhook, err := bot.Webhook()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get webhook info from telegram API")
	}
	if deleteWebhook && existingWebhook.Listen != "" {
		if removeErr := bot.RemoveWebhook(); removeErr != nil {
			return nil, errors.Wrapf(err, "failed to remove webhook for URL '%s'", existingWebhook.Listen)
		}
		logger.Info("Existing webhook successfully deleted", zap.String("url", existingWebhook.Listen))
		return &telebot.Webhook{}, nil // return an empty struct because no webhook exists since removal
	}
	return existingWebhook, nil
}
