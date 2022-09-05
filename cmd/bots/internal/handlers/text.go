package handlers

import (
	"fmt"

	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
	"nodemon/cmd/bots/internal"
	"nodemon/pkg/entities"
	"nodemon/pkg/messaging/pair"
)

func AddNewNodeHandler(
	c tele.Context,
	environment *internal.TelegramBotEnvironment,
	requestType chan<- pair.RequestPair,
	responsePairType <-chan pair.ResponsePair,
	url string,
	specific bool) error {

	if !environment.IsEligibleForAction(c.Chat().ID) {
		return c.Send("Sorry, you have no right to add a new node")
	}

	updatedUrl, err := entities.CheckAndUpdateURL(url)
	if err != nil {
		print(err)
		return c.Send(
			"Sorry, the url seems to be incorrect",
			&tele.SendOptions{ParseMode: tele.ModeDefault},
		)
	}
	requestType <- &pair.InsertNewNodeRequest{Url: updatedUrl, Specific: specific}

	if specific {
		response := fmt.Sprintf("New specific node was '%s' added", updatedUrl)
		err = c.Send(
			response,
			&tele.SendOptions{ParseMode: tele.ModeHTML})
		if err != nil {
			return errors.Wrap(err, "failed to send the message")
		}
		return nil
	}

	response := fmt.Sprintf("New node was '%s' added", updatedUrl)
	err = c.Send(
		response,
		&tele.SendOptions{ParseMode: tele.ModeHTML})
	if err != nil {
		return nil
	}
	urls, err := internal.RequestNodesList(requestType, responsePairType, false)
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
	requestType chan<- pair.RequestPair,
	responsePairType <-chan pair.ResponsePair,
	url string) error {

	if !environment.IsEligibleForAction(c.Chat().ID) {
		return c.Send("Sorry, you have no right to remove a node")
	}

	urlUpdated, err := entities.CheckAndUpdateURL(url)
	if err != nil {
		return c.Send(
			"Sorry, the url seems to be incorrect",
			&tele.SendOptions{ParseMode: tele.ModeDefault},
		)
	}
	requestType <- &pair.DeleteNodeRequest{Url: urlUpdated}

	response := fmt.Sprintf("Node '%s' was deleted", url)
	err = c.Send(
		response,
		&tele.SendOptions{ParseMode: tele.ModeHTML})
	if err != nil {
		return err
	}

	urls, err := internal.RequestNodesList(requestType, responsePairType, false)
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
	alertName string) error {

	if !environment.IsEligibleForAction(c.Chat().ID) {
		return c.Send("Sorry, you have no right to subscribe to alerts")
	}

	alertType, ok := internal.FindAlertTypeByName(alertName)
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
	environment *internal.TelegramBotEnvironment,
	alertName string) error {

	if !environment.IsEligibleForAction(c.Chat().ID) {
		return c.Send("Sorry, you have no right to unsubscribe from alerts")
	}

	alertType, ok := internal.FindAlertTypeByName(alertName)
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
