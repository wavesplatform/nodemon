package handlers

import (
	"fmt"
	"strconv"

	"nodemon/cmd/bots/internal/common"
	"nodemon/cmd/bots/internal/common/messaging"
	"nodemon/pkg/entities"
	"nodemon/pkg/messaging/pair"

	"github.com/pkg/errors"
	"gopkg.in/telebot.v3"
)

func AddNewNodeHandler(
	c telebot.Context,
	env *common.TelegramBotEnvironment,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
	url string,
	specific bool,
) error {
	chatID := strconv.FormatInt(c.Chat().ID, 10)
	response, err := messaging.AddNewNodeHandler(chatID, env, requestType, url, specific)
	if err != nil {
		if errors.Is(err, messaging.ErrIncorrectURL) || errors.Is(err, messaging.ErrInsufficientPermissions) {
			return c.Send(response, &telebot.SendOptions{ParseMode: telebot.ModeDefault})
		}
		return errors.Wrap(err, "failed to add a new node")
	}
	err = c.Send(response, &telebot.SendOptions{ParseMode: telebot.ModeHTML})
	if err != nil {
		return err
	}
	urls, err := messaging.RequestAllNodes(requestType, responsePairType)
	if err != nil {
		return errors.Wrap(err, "failed to request nodes list buttons")
	}
	message, err := env.NodesListMessage(urls)
	if err != nil {
		return errors.Wrap(err, "failed to construct nodes list message")
	}
	return c.Send(message, &telebot.SendOptions{ParseMode: telebot.ModeHTML})
}

func RemoveNodeHandler(
	c telebot.Context,
	environment *common.TelegramBotEnvironment,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
	url string,
) error {
	nodes, err := messaging.RequestAllNodes(requestType, responsePairType)
	if err != nil {
		return errors.Wrap(err, "failed to request nodes list buttons")
	}
	url = common.GetNodeURLByAlias(url, nodes)

	response, err := messaging.RemoveNodeHandler(strconv.FormatInt(c.Chat().ID, 10), environment, requestType, url)
	if err != nil {
		if errors.Is(err, messaging.ErrIncorrectURL) || errors.Is(err, messaging.ErrInsufficientPermissions) {
			return c.Send(response, &telebot.SendOptions{ParseMode: telebot.ModeDefault})
		}
		return errors.Wrap(err, "failed to remove a node")
	}
	err = c.Send(
		response,
		&telebot.SendOptions{ParseMode: telebot.ModeHTML})
	if err != nil {
		return err
	}

	urls, err := messaging.RequestAllNodes(requestType, responsePairType)
	if err != nil {
		return errors.Wrap(err, "failed to request nodes list buttons")
	}
	message, err := environment.NodesListMessage(urls)
	if err != nil {
		return errors.Wrap(err, "failed to construct nodes list message")
	}
	return c.Send(
		message,
		&telebot.SendOptions{
			ParseMode: telebot.ModeHTML,
		},
	)
}

func UpdateAliasHandler(
	c telebot.Context,
	env *common.TelegramBotEnvironment,
	requestChan chan<- pair.Request,
	url string,
	alias string,
) error {
	response, err := messaging.UpdateAliasHandler(strconv.FormatInt(c.Chat().ID, 10), env, requestChan, url, alias)
	if err != nil {
		if errors.Is(err, messaging.ErrIncorrectURL) || errors.Is(err, messaging.ErrInsufficientPermissions) {
			return c.Send(response, &telebot.SendOptions{ParseMode: telebot.ModeDefault})
		}
		return errors.Wrap(err, "failed to update a node")
	}
	return c.Send(response, &telebot.SendOptions{ParseMode: telebot.ModeHTML})
}

func SubscribeHandler(c telebot.Context, env *common.TelegramBotEnvironment, alertName entities.AlertName) error {
	if !env.IsEligibleForAction(strconv.FormatInt(c.Chat().ID, 10)) {
		return c.Send("Sorry, you have no right to subscribe to alerts")
	}

	alertType, ok := alertName.AlertType()
	if !ok {
		return c.Send("Sorry, this alert does not exist", &telebot.SendOptions{ParseMode: telebot.ModeDefault})
	}
	if env.IsAlreadySubscribed(alertType) {
		return c.Send("I am already subscribed to it", &telebot.SendOptions{ParseMode: telebot.ModeDefault})
	}
	err := env.SubscribeToAlert(alertType)
	if err != nil {
		return errors.Wrapf(err, "failed to subscribe to alert %s", alertName)
	}
	err = c.Send(fmt.Sprintf("I succesfully subscribed to %s", alertName),
		&telebot.SendOptions{ParseMode: telebot.ModeHTML},
	)
	if err != nil {
		return errors.Wrap(err, "failed to send a message")
	}
	msg, err := env.SubscriptionsList()
	if err != nil {
		return errors.Wrap(err, "failed to receive list of subscriptions")
	}
	return c.Send(msg, &telebot.SendOptions{ParseMode: telebot.ModeHTML})
}

func UnsubscribeHandler(c telebot.Context, env *common.TelegramBotEnvironment, alertName entities.AlertName) error {
	chatID := strconv.FormatInt(c.Chat().ID, 10)
	if !env.IsEligibleForAction(chatID) {
		return c.Send("Sorry, you have no right to unsubscribe from alerts")
	}

	alertType, ok := alertName.AlertType()
	if !ok {
		return c.Send("Sorry, this alert does not exist", &telebot.SendOptions{ParseMode: telebot.ModeDefault})
	}
	if !env.IsAlreadySubscribed(alertType) {
		return c.Send("I was not subscribed to it", &telebot.SendOptions{ParseMode: telebot.ModeDefault})
	}
	err := env.UnsubscribeFromAlert(alertType)
	if err != nil {
		return errors.Wrapf(err, "failed to unsubscribe from alert %s", alertName)
	}
	err = c.Send(fmt.Sprintf("I succesfully unsubscribed from %s", alertName),
		&telebot.SendOptions{ParseMode: telebot.ModeHTML},
	)
	if err != nil {
		return errors.Wrap(err, "failed to send a message")
	}
	msg, err := env.SubscriptionsList()
	if err != nil {
		return errors.Wrap(err, "failed to receive list of subscriptions")
	}
	return c.Send(msg, &telebot.SendOptions{ParseMode: telebot.ModeHTML})
}
