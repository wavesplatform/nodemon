package config

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"gopkg.in/telebot.v3"
)

const (
	PollingMethod = "polling"
	WebhookMethod = "webhook"
)

const (
	telegramRemoveWebhook        = "https://api.telegram.org/bot%s/setWebhook?remove"
	telegramRemoveWebhookTimeout = 5 * time.Second
	longPollingTimeout           = 10 * time.Second
)

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
		logger.Error("Unknown error occurred in a bot handler",
			zap.Int("update_id", updateID),
			zap.Intp("message_id", messageID),
			zap.Stringp("message_text", messageText),
			zap.Int64p("sender_id", senderID),
			zap.Stringp("sender_username", senderUsername),
			zap.Error(err),
		)
	}
}

func NewTgBotSettings(
	logger *zap.Logger,
	behavior string,
	webhookLocalAddress string,
	publicURL string,
	botToken string,
) (*telebot.Settings, error) {
	switch behavior {
	case WebhookMethod:
		if publicURL == "" {
			return nil, errors.New("no public url for webhook method was provided")
		}
		webhook := &telebot.Webhook{
			Listen:   webhookLocalAddress,
			Endpoint: &telebot.WebhookEndpoint{PublicURL: publicURL},
		}
		botSettings := telebot.Settings{
			Token:   botToken,
			Poller:  webhook,
			OnError: createOnErrorHandler(logger),
		}
		return &botSettings, nil
	case PollingMethod:
		if err := tryRemoveWebhookIfExists(botToken); err != nil {
			return nil, errors.Wrap(err, "failed to remove webhook if exists")
		}
		botSettings := telebot.Settings{
			Token:   botToken,
			Poller:  &telebot.LongPoller{Timeout: longPollingTimeout},
			OnError: createOnErrorHandler(logger),
		}
		return &botSettings, nil
	default:
		return nil, errors.Errorf("wrong type of bot behavior %q was provided", behavior)
	}
}

// tryRemoveWebhookIfExists deletes webhook if there is any.
func tryRemoveWebhookIfExists(botToken string) (err error) {
	ctx, cancel := context.WithTimeout(context.Background(), telegramRemoveWebhookTimeout)
	defer cancel()

	url := fmt.Sprintf(telegramRemoveWebhook, botToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return errors.Wrap(err, "failed to create HTTP request with context")
	}

	cl := &http.Client{Timeout: telegramRemoveWebhookTimeout}
	resp, err := cl.Do(req)
	if err != nil {
		return errors.Wrap(err, "failed to remove webhook")
	}

	resp.Close = true
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			if err != nil {
				err = errors.Wrap(err, closeErr.Error())
			} else {
				err = closeErr
			}
		}
	}()
	if resp.StatusCode == http.StatusInternalServerError {
		return errors.Wrap(err, "failed to remove webhook")
	}
	return nil
}
