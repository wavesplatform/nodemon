package config

import (
	"context"
	"encoding/json"
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

func NewTgBotSettings(
	behavior string,
	publicURL string,
	botToken string,
	logger *zap.Logger,
) (*telebot.Settings, http.Handler, error) {
	switch behavior {
	case WebhookMethod:
		if publicURL == "" {
			return nil, nil, errors.New("no public url for webhook method was provided")
		}
		webhook := &telebot.Webhook{
			Listen:   "", // In this case it is up to the caller to add the Webhook to a http-mux.
			Endpoint: &telebot.WebhookEndpoint{PublicURL: publicURL},
		}
		botSettings := telebot.Settings{
			Token:  botToken,
			Poller: webhook}
		return &botSettings, http.Handler(webhook), nil
	case PollingMethod:
		if err := tryRemoveWebhookIfExists(botToken); err != nil {
			return nil, nil, errors.Wrap(err, "failed to remove webhook if exists")
		}
		botSettings := telebot.Settings{
			Token:  botToken,
			Poller: &telebot.LongPoller{Timeout: longPollingTimeout},
		}
		// TODO: is this shutdown necessary?
		httpNoWebhookInformer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if m := r.Method; m != http.MethodGet && m != http.MethodPost {
				const httpCode = http.StatusMethodNotAllowed
				http.Error(w, http.StatusText(httpCode), httpCode)
				return
			}
			type jsonResp struct {
				Error string `json:"error"`
			}
			w.Header().Set("Content-Type", "application/json")
			msg := jsonResp{Error: "bot uses polling method for receiving updates"}
			if err := json.NewEncoder(w).Encode(msg); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				logger.Error("Failed to encode json response", zap.Error(err))
				return
			}
			w.WriteHeader(http.StatusForbidden)
		})
		return &botSettings, httpNoWebhookInformer, nil
	default:
		return nil, nil, errors.Errorf("wrong type of bot behavior %q was provided", behavior)
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
