package config

import (
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
)

const (
	PollingMethod = "polling"
	WebhookMethod = "webhook"
)

const (
	telegramRemoveWebhook = "https://api.telegram.org/bot%s/setWebhook?remove"
	longPollingTimeout    = 10 * time.Second
)

func NewTgBotSettings(
	behavior string,
	webhookLocalAddress string,
	publicURL string,
	botToken string,
) (*tele.Settings, error) {
	switch behavior {
	case WebhookMethod:
		if publicURL == "" {
			return nil, errors.New("no public url for webhook method was provided")
		}
		webhook := &tele.Webhook{
			Listen:   webhookLocalAddress,
			Endpoint: &tele.WebhookEndpoint{PublicURL: publicURL},
		}
		botSettings := tele.Settings{
			Token:  botToken,
			Poller: webhook}
		return &botSettings, nil
	case PollingMethod:
		if err := tryRemoveWebhookIfExists(botToken); err != nil {
			return nil, errors.Wrap(err, "failed to remove webhook if exists")
		}
		botSettings := tele.Settings{
			Token:  botToken,
			Poller: &tele.LongPoller{Timeout: longPollingTimeout},
		}
		return &botSettings, nil
	default:
		return nil, errors.Errorf("wrong type of bot behavior %q was provided", behavior)
	}
}

// tryRemoveWebhookIfExists deletes webhook if there is any.
func tryRemoveWebhookIfExists(botToken string) (err error) {
	resp, formErr := http.PostForm(fmt.Sprintf(telegramRemoveWebhook, botToken), nil)
	if formErr != nil {
		return errors.Wrap(formErr, "failed to remove webhook")
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
