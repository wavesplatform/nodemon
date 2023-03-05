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

	telegramRemoveWebhook = "https://api.telegram.org/bot%s/setWebhook?remove"
)

func NewTgBotSettings(behavior string, webhookLocalAddress string, publicURL string, botToken string) (_ *tele.Settings, err error) {

	if behavior == WebhookMethod {
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
	}
	if behavior == PollingMethod {
		// delete webhook if there is any
		resp, formErr := http.PostForm(fmt.Sprintf(telegramRemoveWebhook, botToken), nil)
		if formErr != nil {
			return nil, errors.Wrap(formErr, "failed to remove webhook")
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
			return nil, errors.Wrap(err, "failed to remove webhook")
		}

		botSettings := tele.Settings{
			Token:  botToken,
			Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		}
		return &botSettings, nil

	}

	return nil, errors.New("wrong type of bot behavior was provided")
}
