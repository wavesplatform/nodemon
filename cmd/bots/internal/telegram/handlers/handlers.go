package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
	"nodemon/cmd/bots/internal/common"
	commonMessages "nodemon/cmd/bots/internal/common/messages"
	"nodemon/cmd/bots/internal/common/messaging"
	"nodemon/cmd/bots/internal/telegram/buttons"
	"nodemon/cmd/bots/internal/telegram/messages"
	"nodemon/pkg/entities"
	"nodemon/pkg/messaging/pair"
)

func InitTgHandlers(environment *common.TelegramBotEnvironment, requestType chan<- pair.RequestPair, responsePairType <-chan pair.ResponsePair) {

	environment.Bot.Handle("/chat", func(c tele.Context) error {

		return c.Send(fmt.Sprintf("I am sending alerts through %d chat id", environment.ChatID))
	})

	environment.Bot.Handle("/ping", func(c tele.Context) error {
		if environment.Mute {
			return c.Send(messages.PongText + " I am currently sleeping" + commonMessages.SleepingMsg)
		}
		return c.Send(messages.PongText + " I am monitoring" + commonMessages.MonitoringMsg)
	})

	environment.Bot.Handle("/start", func(c tele.Context) error {
		if !environment.IsEligibleForAction(strconv.FormatInt(c.Chat().ID, 10)) {
			return c.Send("Sorry, you have no right to start me")
		}
		if environment.Mute {
			environment.Mute = false
			return c.Send("I had been asleep, but started monitoring now... " + commonMessages.MonitoringMsg)
		}
		return c.Send("I had already been monitoring" + commonMessages.MonitoringMsg)
	})

	environment.Bot.Handle("/mute", func(c tele.Context) error {
		if !environment.IsEligibleForAction(strconv.FormatInt(c.Chat().ID, 10)) {
			return c.Send("Sorry, you have no right to mute me")
		}
		if environment.Mute {
			return c.Send("I had already been sleeping, continue sleeping.." + commonMessages.SleepingMsg)
		}
		environment.Mute = true
		return c.Send("I had been monitoring, but going to sleep now.." + commonMessages.SleepingMsg)
	})

	environment.Bot.Handle("/help", func(c tele.Context) error {
		return c.Send(
			messages.HelpInfoText,
			&tele.SendOptions{ParseMode: tele.ModeHTML})
	})

	environment.Bot.Handle("\f"+buttons.AddNewNode, func(c tele.Context) error {
		return c.Send(
			messages.AddNewNodeMsg,
			&tele.SendOptions{ParseMode: tele.ModeDefault})
	})

	environment.Bot.Handle("\f"+buttons.RemoveNode, func(c tele.Context) error {
		return c.Send(
			messages.RemoveNode,
			&tele.SendOptions{
				ParseMode: tele.ModeDefault,
			},
		)
	})

	environment.Bot.Handle("\f"+buttons.SubscribeTo, func(c tele.Context) error {
		return c.Send(
			messages.SubscribeTo,
			&tele.SendOptions{ParseMode: tele.ModeDefault})
	})
	environment.Bot.Handle("\f"+buttons.UnsubscribeFrom, func(c tele.Context) error {
		return c.Send(
			messages.UnsubscribeFrom,
			&tele.SendOptions{
				ParseMode: tele.ModeDefault,
			},
		)
	})

	environment.Bot.Handle("/pool", func(c tele.Context) error {
		return EditPool(c, environment, requestType, responsePairType)
	})
	environment.Bot.Handle("/subscriptions", func(c tele.Context) error {
		return EditSubscriptions(c, environment)
	})
	environment.Bot.Handle("/add", func(c tele.Context) error {
		args := c.Args()
		if len(args) > 1 {
			return c.Send(
				messages.AddedMoreThanOne,
				&tele.SendOptions{
					ParseMode: tele.ModeDefault,
				},
			)
		}
		if len(args) < 1 {
			return c.Send(
				messages.AddedLessThanOne,
				&tele.SendOptions{
					ParseMode: tele.ModeDefault,
				},
			)
		}
		response := AddNewNodeHandler(c, environment, requestType, responsePairType, args[0], false)
		err := c.Send(
			response,
			&tele.SendOptions{ParseMode: tele.ModeHTML})
		if err != nil {
			return nil
		}
		urls, err := messaging.RequestNodesUrls(requestType, responsePairType, false)
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

	})
	environment.Bot.Handle("/remove", func(c tele.Context) error {
		args := c.Args()
		if len(args) > 1 {
			return c.Send(
				messages.RemovedMoreThanOne,
				&tele.SendOptions{
					ParseMode: tele.ModeDefault,
				},
			)
		}
		if len(args) < 1 {
			return c.Send(
				messages.RemovedLessThanOne,
				&tele.SendOptions{
					ParseMode: tele.ModeDefault,
				},
			)
		}
		return RemoveNodeHandler(c, environment, requestType, responsePairType, args[0])

	})

	environment.Bot.Handle("/add_alias", func(c tele.Context) error {
		args := c.Args()
		if len(args) != 2 {
			return c.Send(
				messages.AliasWrongFormat,
				&tele.SendOptions{
					ParseMode: tele.ModeDefault,
				},
			)
		}
		return UpdateAliasHandler(c, environment, requestType, args[0], args[1])
	})

	environment.Bot.Handle("/aliases", func(c tele.Context) error {
		nodes, err := messaging.RequestAllNodes(requestType, responsePairType)
		if err != nil {
			environment.Zap.Error("failed to request nodes list", zap.Error(err))
			return errors.Wrap(err, "failed to get nodes list")
		}
		var msg string
		for _, n := range nodes {
			if n.Alias != "" {
				msg += fmt.Sprintf("Node: %s\nAlias: %s\n\n", n.URL, n.Alias)
			}
		}
		if msg == "" {
			msg = "No aliases have been found"
		}

		return c.Send(
			msg,
			&tele.SendOptions{
				ParseMode: tele.ModeHTML,
			},
		)
	})

	environment.Bot.Handle("/subscribe", func(c tele.Context) error {
		args := c.Args()
		if len(args) > 1 {
			return c.Send(
				messages.SubscribedToMoreThanOne,
				&tele.SendOptions{
					ParseMode: tele.ModeDefault,
				},
			)
		}
		if len(args) < 1 {
			return c.Send(
				messages.SubscribedToLessThanOne,
				&tele.SendOptions{
					ParseMode: tele.ModeDefault,
				},
			)
		}
		return SubscribeHandler(c, environment, args[0])
	})
	environment.Bot.Handle("/unsubscribe", func(c tele.Context) error {
		args := c.Args()
		if len(args) > 1 {
			return c.Send(
				messages.UnsubscribedFromMoreThanOne,
				&tele.SendOptions{
					ParseMode: tele.ModeDefault,
				},
			)
		}
		if len(args) < 1 {
			return c.Send(
				messages.UnsubscribedFromLessThanOne,
				&tele.SendOptions{
					ParseMode: tele.ModeDefault,
				},
			)
		}
		return UnsubscribeHandler(c, environment, args[0])
	})

	environment.Bot.Handle("/statement", func(c tele.Context) error {
		args := c.Args()
		if len(args) > 2 || len(args) < 1 {
			return c.Send(
				messages.StatementWrongFormat,
				&tele.SendOptions{
					ParseMode: tele.ModeDefault,
				},
			)
		}
		url := args[0]
		updatedUrl, err := entities.CheckAndUpdateURL(url)
		if err != nil {
			return c.Send(
				messages.InvalidURL,
				&tele.SendOptions{
					ParseMode: tele.ModeDefault,
				},
			)
		}
		height, err := strconv.Atoi(args[1])
		if err != nil {
			return c.Send(
				fmt.Sprintf("failed to parse height: %s", err.Error()),
				&tele.SendOptions{
					ParseMode: tele.ModeDefault,
				},
			)
		}
		statement, err := messaging.RequestNodeStatement(requestType, responsePairType, updatedUrl, height)
		if err != nil {
			environment.Zap.Error("failed to request nodes list buttons", zap.Error(err))
			return err
		}

		msg, err := common.HandleNodeStatement(statement, common.Html)
		if err != nil {
			environment.Zap.Error("failed to handle status of nodes", zap.Error(err))
			return err
		}

		return c.Send(
			msg,
			&tele.SendOptions{
				ParseMode: tele.ModeHTML,
			},
		)
	})

	environment.Bot.Handle(tele.OnText, func(c tele.Context) error {
		command := strings.ToLower(c.Text())

		if strings.HasPrefix(command, "add specific") {
			u := strings.TrimPrefix(command, "add specific ")
			return AddNewNodeHandler(c, environment, requestType, responsePairType, u, true)
		}

		if strings.HasPrefix(command, "add") {
			u := strings.TrimPrefix(command, "add ")
			return AddNewNodeHandler(c, environment, requestType, responsePairType, u, false)
		}

		if strings.HasPrefix(command, "remove") {
			u := strings.TrimPrefix(command, "remove ")
			return RemoveNodeHandler(c, environment, requestType, responsePairType, u)
		}
		if strings.HasPrefix(command, "subscribe to") {
			alertName := strings.TrimPrefix(command, "subscribe to ")
			return SubscribeHandler(c, environment, alertName)
		}

		if strings.HasPrefix(command, "unsubscribe from") {
			alertName := strings.TrimPrefix(command, "unsubscribe from ")
			return UnsubscribeHandler(c, environment, alertName)
		}
		return nil
	})

	environment.Bot.Handle("/status", func(c tele.Context) error {
		nodes, err := messaging.RequestAllNodes(requestType, responsePairType)
		if err != nil {
			environment.Zap.Error("failed to get nodes list", zap.Error(err))
		}
		urls := messaging.NodesToUrls(nodes)

		nodesStatus, err := messaging.RequestNodesStatus(requestType, responsePairType, urls)
		if err != nil {
			environment.Zap.Error("failed to request status of nodes", zap.Error(err))
			return err
		}

		msg, statusCondition, err := common.HandleNodesStatus(nodesStatus, common.Html, nodes)
		if err != nil {
			environment.Zap.Error("failed to handle status of nodes", zap.Error(err))
			return err
		}

		if statusCondition.AllNodesAreOk {
			msg = fmt.Sprintf("<b>%d</b> %s", statusCondition.NodesNumber, msg)
		}

		return c.Send(
			msg,
			&tele.SendOptions{
				ParseMode: tele.ModeHTML,
			},
		)
	})

}

func EditPool(
	c tele.Context,
	environment *common.TelegramBotEnvironment,
	requestType chan<- pair.RequestPair,
	responsePairType <-chan pair.ResponsePair) error {

	urls, err := messaging.RequestNodesUrls(requestType, responsePairType, false)
	if err != nil {
		return errors.Wrap(err, "failed to request nodes list buttons")
	}
	message, err := environment.NodesListMessage(urls)
	if err != nil {
		return errors.Wrap(err, "failed to construct nodes list message")
	}
	err = c.Send(
		message,
		&tele.SendOptions{
			ParseMode: tele.ModeHTML,
		},
	)
	if err != nil {
		return err
	}

	keyboardAddDelete := [][]tele.InlineButton{{
		{
			Text:   "Add new node",
			Unique: buttons.AddNewNode,
		},
		{
			Text:   "Remove node",
			Unique: buttons.RemoveNode,
		},
	}}

	return c.Send(
		"Please choose",
		&tele.SendOptions{

			ParseMode: tele.ModeHTML,
			ReplyMarkup: &tele.ReplyMarkup{
				InlineKeyboard:  keyboardAddDelete,
				ResizeKeyboard:  true,
				OneTimeKeyboard: true},
		},
	)
}

func EditSubscriptions(
	c tele.Context,
	environment *common.TelegramBotEnvironment) error {
	msg, err := environment.SubscriptionsList()
	if err != nil {
		return errors.Wrap(err, "failed to request subscriptions")
	}
	err = c.Send(
		msg,
		&tele.SendOptions{
			ParseMode: tele.ModeHTML,
		},
	)
	if err != nil {
		return err
	}

	keyboardSubUnsub := [][]tele.InlineButton{{
		{
			Text:   "Subscribe to",
			Unique: buttons.SubscribeTo,
		},
		{
			Text:   "Unsubscribe from",
			Unique: buttons.UnsubscribeFrom,
		},
	}}

	return c.Send(
		"Please choose",
		&tele.SendOptions{

			ParseMode: tele.ModeHTML,
			ReplyMarkup: &tele.ReplyMarkup{
				InlineKeyboard:  keyboardSubUnsub,
				ResizeKeyboard:  true,
				OneTimeKeyboard: true},
		},
	)
}
