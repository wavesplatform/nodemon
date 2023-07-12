package handlers

import (
	"fmt"
	"strconv"

	"nodemon/cmd/bots/internal/common"
	"nodemon/cmd/bots/internal/common/messaging"
	"nodemon/pkg/entities"
	"nodemon/pkg/messaging/pair"

	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
)

func AddNewNodeHandler(
	c tele.Context,
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
			return c.Send(response, &tele.SendOptions{ParseMode: tele.ModeDefault})
		}
		return errors.Wrap(err, "failed to add a new node")
	}
	err = c.Send(response, &tele.SendOptions{ParseMode: tele.ModeHTML})
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
	return c.Send(message, &tele.SendOptions{ParseMode: tele.ModeHTML})
}

func RemoveNodeHandler(
	c tele.Context,
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
			return c.Send(response, &tele.SendOptions{ParseMode: tele.ModeDefault})
		}
		return errors.Wrap(err, "failed to remove a node")
	}
	err = c.Send(
		response,
		&tele.SendOptions{ParseMode: tele.ModeHTML})
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
		&tele.SendOptions{
			ParseMode: tele.ModeHTML,
		},
	)
}

func UpdateAliasHandler(
	c tele.Context,
	env *common.TelegramBotEnvironment,
	requestChan chan<- pair.Request,
	url string,
	alias string,
) error {
	response, err := messaging.UpdateAliasHandler(strconv.FormatInt(c.Chat().ID, 10), env, requestChan, url, alias)
	if err != nil {
		if errors.Is(err, messaging.ErrIncorrectURL) || errors.Is(err, messaging.ErrInsufficientPermissions) {
			return c.Send(response, &tele.SendOptions{ParseMode: tele.ModeDefault})
		}
		return errors.Wrap(err, "failed to update a node")
	}
	return c.Send(response, &tele.SendOptions{ParseMode: tele.ModeHTML})
}

func SubscribeHandler(c tele.Context, environment *common.TelegramBotEnvironment, alertName entities.AlertName) error {
	if !environment.IsEligibleForAction(strconv.FormatInt(c.Chat().ID, 10)) {
		return c.Send("Sorry, you have no right to subscribe to alerts")
	}

	alertType, ok := alertName.AlertType()
	if !ok {
		return c.Send("Sorry, this alert does not exist", &tele.SendOptions{ParseMode: tele.ModeDefault})
	}
	if environment.IsAlreadySubscribed(alertType) {
		return c.Send("I am already subscribed to it", &tele.SendOptions{ParseMode: tele.ModeDefault})
	}
	err := environment.SubscribeToAlert(alertType)
	if err != nil {
		return errors.Wrapf(err, "failed to subscribe to alert %s", alertName)
	}
	err = c.Send(fmt.Sprintf("I succesfully subscribed to %s", alertName),
		&tele.SendOptions{ParseMode: tele.ModeHTML},
	)
	if err != nil {
		return errors.Wrap(err, "failed to send a message")
	}
	msg, err := environment.SubscriptionsList()
	if err != nil {
		return errors.Wrap(err, "failed to receive list of subscriptions")
	}
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeHTML})
}

func UnsubscribeHandler(c tele.Context, env *common.TelegramBotEnvironment, alertName entities.AlertName) error {
	chatID := strconv.FormatInt(c.Chat().ID, 10)
	if !env.IsEligibleForAction(chatID) {
		return c.Send("Sorry, you have no right to unsubscribe from alerts")
	}

	alertType, ok := alertName.AlertType()
	if !ok {
		return c.Send("Sorry, this alert does not exist", &tele.SendOptions{ParseMode: tele.ModeDefault})
	}
	if !env.IsAlreadySubscribed(alertType) {
		return c.Send("I was not subscribed to it", &tele.SendOptions{ParseMode: tele.ModeDefault})
	}
	err := env.UnsubscribeFromAlert(alertType)
	if err != nil {
		return errors.Wrapf(err, "failed to unsubscribe from alert %s", alertName)
	}
	err = c.Send(fmt.Sprintf("I succesfully unsubscribed from %s", alertName),
		&tele.SendOptions{ParseMode: tele.ModeHTML},
	)
	if err != nil {
		return errors.Wrap(err, "failed to send a message")
	}
	msg, err := env.SubscriptionsList()
	if err != nil {
		return errors.Wrap(err, "failed to receive list of subscriptions")
	}
	return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeHTML})
}
