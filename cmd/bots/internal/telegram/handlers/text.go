package handlers

import (
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
	"nodemon/cmd/bots/internal/common"
	"nodemon/cmd/bots/internal/common/messaging"
	"nodemon/pkg/messaging/pair"
)

func AddNewNodeHandler(
	c tele.Context,
	environment *common.TelegramBotEnvironment,
	requestType chan<- pair.RequestPair,
	responsePairType <-chan pair.ResponsePair,
	url string,
	specific bool) error {
	response, err := messaging.AddNewNodeHandler(strconv.FormatInt(c.Chat().ID, 10), environment, requestType, url, specific)
	if err != nil {
		if err == messaging.IncorrectUrlError || err == messaging.InsufficientPermissionsError {
			return c.Send(
				response,
				&tele.SendOptions{ParseMode: tele.ModeDefault},
			)
		}
		return errors.Wrap(err, "failed to add a new node")
	}
	err = c.Send(
		response,
		&tele.SendOptions{ParseMode: tele.ModeHTML})
	if err != nil {
		return nil
	}
	urls, err := common.RequestNodesList(requestType, responsePairType, false)
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
	environment *common.TelegramBotEnvironment,
	requestType chan<- pair.RequestPair,
	responsePairType <-chan pair.ResponsePair,
	url string) error {
	response, err := messaging.RemoveNodeHandler(strconv.FormatInt(c.Chat().ID, 10), environment, requestType, url)
	if err != nil {
		if err == messaging.IncorrectUrlError || err == messaging.InsufficientPermissionsError {
			return c.Send(
				response,
				&tele.SendOptions{ParseMode: tele.ModeDefault},
			)
		}
		return errors.Wrap(err, "failed to remove a node")
	}
	err = c.Send(
		response,
		&tele.SendOptions{ParseMode: tele.ModeHTML})
	if err != nil {
		return err
	}

	urls, err := common.RequestNodesList(requestType, responsePairType, false)
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
	environment *common.TelegramBotEnvironment,
	alertName string) error {

	if !environment.IsEligibleForAction(strconv.FormatInt(c.Chat().ID, 10)) {
		return c.Send("Sorry, you have no right to subscribe to alerts")
	}

	alertType, ok := common.FindAlertTypeByName(alertName)
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
	if err != nil {
		return errors.Wrap(err, "failed to receive list of subscriptions")
	}
	return c.Send(
		msg,
		&tele.SendOptions{
			ParseMode: tele.ModeHTML,
		},
	)
}

func UnsubscribeHandler(
	c tele.Context,
	environment *common.TelegramBotEnvironment,
	alertName string) error {

	if !environment.IsEligibleForAction(strconv.FormatInt(c.Chat().ID, 10)) {
		return c.Send("Sorry, you have no right to unsubscribe from alerts")
	}

	alertType, ok := common.FindAlertTypeByName(alertName)
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
	err := environment.UnsubscribeFromAlert(alertType)
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
	if err != nil {
		return errors.Wrap(err, "failed to receive list of subscriptions")
	}
	return c.Send(
		msg,
		&tele.SendOptions{
			ParseMode: tele.ModeHTML,
		},
	)
}
