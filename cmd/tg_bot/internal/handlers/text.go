package handlers

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
	"nodemon/cmd/tg_bot/internal"
	"nodemon/pkg/messaging/pair"
)

func AddNewNodeHandler(
	c tele.Context,
	environment *internal.TelegramBotEnvironment,
	requestType chan pair.RequestPair,
	responsePairType chan pair.ResponsePair) error {

	url := strings.TrimPrefix(c.Text(), "Add ")
	if !strings.HasPrefix(url, "http") && !strings.HasPrefix(url, "https") {
		return c.Send(
			"Sorry, the url seems to be incorrect",
			&tele.SendOptions{ParseMode: tele.ModeDefault},
		)
	}
	requestType <- &pair.InsertNewNodeRequest{Url: url}

	response := fmt.Sprintf("New node was '%s' added", url)
	err := c.Send(
		response,
		&tele.SendOptions{ParseMode: tele.ModeHTML})
	if err != nil {
		return nil
	}
	urls, err := requestNodesList(requestType, responsePairType)
	if err != nil {
		return errors.Wrap(err, "failed to request nodes list buttons")
	}
	message, err := environment.NodesListMessage(urls)
	if err != nil {
		return errors.Wrap(err, "failed to construct nodes list message")
	}
	return c.Send(
		message,
		&tele.SendOptions{
			ParseMode: tele.ModeHTML,
		},
	)
}

func RemoveNodeHandler(
	c tele.Context,
	environment *internal.TelegramBotEnvironment,
	requestType chan pair.RequestPair,
	responsePairType chan pair.ResponsePair) error {

	url := strings.TrimPrefix(c.Text(), "Remove ")
	if !strings.HasPrefix(url, "http") && !strings.HasPrefix(url, "https") {
		return c.Send(
			"Sorry, the url seems to be incorrect",
			&tele.SendOptions{ParseMode: tele.ModeDefault},
		)
	}
	requestType <- &pair.DeleteNodeRequest{Url: url}

	response := fmt.Sprintf("Node '%s' was deleted", url)
	err := c.Send(
		response,
		&tele.SendOptions{ParseMode: tele.ModeHTML})
	if err != nil {
		return err
	}
	urls, err := requestNodesList(requestType, responsePairType)
	if err != nil {
		return errors.Wrap(err, "failed to request nodes list buttons")
	}
	message, err := environment.NodesListMessage(urls)
	if err != nil {
		return errors.Wrap(err, "failed to construct nodes list message")
	}
	return c.Send(
		message,
		&tele.SendOptions{
			ParseMode: tele.ModeHTML,
		},
	)
}

func SubscribeHandler(
	c tele.Context,
	environment *internal.TelegramBotEnvironment,
) error {
	alertName := strings.TrimPrefix(c.Text(), "Subscribe to ")
	alertType, ok := environment.FindAlertTypeByName(alertName)
	if !ok {
		return c.Send(
			"Sorry, this alert does not exist",
			&tele.SendOptions{
				ParseMode: tele.ModeDefault,
			},
		)
	}
	if environment.IsAlreadySubscribed(alertType) {
		return c.Send(
			"I am already subscribed to it",
			&tele.SendOptions{
				ParseMode: tele.ModeDefault,
			},
		)
	}
	err := environment.SubscribeToAlert(alertType)
	if err != nil {
		return errors.Wrapf(err, "failed to subscribe to alert %s", alertName)
	}
	err = c.Send(
		fmt.Sprintf("I succesfully subscribed to %s", alertName),
		&tele.SendOptions{
			ParseMode: tele.ModeHTML,
		},
	)
	if err != nil {
		return errors.Wrap(err, "failed to send a message")
	}
	msg, err := environment.SubscriptionsList()
	return c.Send(
		msg,
		&tele.SendOptions{
			ParseMode: tele.ModeHTML,
		},
	)
}

func UnsubscribeHandler(
	c tele.Context,
	environment *internal.TelegramBotEnvironment,
) error {
	alertName := strings.TrimPrefix(c.Text(), "Unsubscribe from ")
	alertType, ok := environment.FindAlertTypeByName(alertName)
	if !ok {
		return c.Send(
			"Sorry, this alert does not exist",
			&tele.SendOptions{
				ParseMode: tele.ModeDefault,
			},
		)
	}
	if !environment.IsAlreadySubscribed(alertType) {
		return c.Send(
			"I was not subscribed to it",
			&tele.SendOptions{
				ParseMode: tele.ModeDefault,
			},
		)
	}
	err := environment.UnubscribeFromAlert(alertType)
	if err != nil {
		return errors.Wrapf(err, "failed to unsubscribe from alert %s", alertName)
	}
	err = c.Send(
		fmt.Sprintf("I succesfully unsubscribed from %s", alertName),
		&tele.SendOptions{
			ParseMode: tele.ModeHTML,
		},
	)
	if err != nil {
		return errors.Wrap(err, "failed to send a message")
	}
	msg, err := environment.SubscriptionsList()
	return c.Send(
		msg,
		&tele.SendOptions{
			ParseMode: tele.ModeHTML,
		},
	)
}
