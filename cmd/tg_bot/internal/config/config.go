package config

import (
	"fmt"
	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
	"net/http"
	"time"
)

const (
	PollingMethod = "polling"
	WebhookMethod = "webhook"

	telegramRemoveWebhook = "https://api.telegram.org/bot%s/setWebhook?remove"
)

func NewBotSettings(behavior string, webhookLocalAddress string, publicURL string, botToken string) (*tele.Settings, error) {

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
		resp, err := http.PostForm(fmt.Sprintf(telegramRemoveWebhook, botToken), nil)
		if err != nil {
			return nil, errors.Wrap(err, "failed to remove webhook")
		}
		resp.Close = true
		defer resp.Body.Close()

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
