package config

import (
	"fmt"
	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
	"net/http"
	"time"
)

const (
	pollingMethod = "polling"
	webhookMethod = "webhook"
)

var (
	InvalidParameters = errors.New("invalid parameters ")
)

type BotConfig struct {
	Settings tele.Settings
}

func NewBotConfig(behavior string, webhookLocalAddress string, publicURL string, botToken string, ) (*BotConfig, error) {

	if behavior == webhookMethod {
		if publicURL == "" {
			return nil, errors.New("no public url for webhook method was provided")
		}
		webhook := &tele.Webhook{
			Listen:   webhookLocalAddress,
			Endpoint: &tele.WebhookEndpoint{PublicURL: publicURL},
		}
		fmt.Println(webhook.Endpoint)
		botSettings := tele.Settings{
			Token:  botToken,
			Poller: webhook}
		return &BotConfig{Settings: botSettings}, nil
	}
	if behavior == pollingMethod {
		// delete webhook if there is any
		url := "https://api.telegram.org" + "/bot" + botToken + "/" + "setWebhook?remove"
		resp, err := http.PostForm(url, nil)
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
		return &BotConfig{Settings: botSettings}, nil

	}

	return nil, errors.New("wrong type of bot behavior was provided")
}
