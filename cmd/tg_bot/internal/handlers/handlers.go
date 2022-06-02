package handlers

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
	tele "gopkg.in/telebot.v3"
	"nodemon/cmd/tg_bot/internal"
	"nodemon/cmd/tg_bot/internal/buttons"
	"nodemon/cmd/tg_bot/internal/messages"
	"nodemon/pkg/messaging/pair"
)

func InitHandlers(environment *internal.TelegramBotEnvironment, requestType chan pair.RequestPair, responsePairType chan pair.ResponsePair) {

	environment.Bot.Handle("/chat", func(c tele.Context) error {

		return c.Send(fmt.Sprintf("I am sending alerts through %d chat id", environment.ChatID))
	})

	environment.Bot.Handle("/ping", func(c tele.Context) error {
		if environment.Mute {
			return c.Send(messages.PongText + " I am currently sleeping" + messages.SleepingMsg)
		}
		return c.Send(messages.PongText + " I am monitoring" + messages.MonitoringMsg)
	})

	environment.Bot.Handle("/start", func(c tele.Context) error {
		if !environment.IsEligibleForAction(c.Chat().ID) {
			return c.Send("Sorry, you have no right to start me")
		}
		if environment.Mute {
			environment.Mute = false
			return c.Send("I had been asleep, but started monitoring now... " + messages.MonitoringMsg)
		}
		return c.Send("I had already been monitoring" + messages.MonitoringMsg)
	})

	environment.Bot.Handle("/mute", func(c tele.Context) error {
		if !environment.IsEligibleForAction(c.Chat().ID) {
			return c.Send("Sorry, you have no right to mute me")
		}
		if environment.Mute {
			return c.Send("I had already been sleeping, continue sleeping.." + messages.SleepingMsg)
		}
		environment.Mute = true
		return c.Send("I had been monitoring, but going to sleep now.." + messages.SleepingMsg)
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

	environment.Bot.Handle(tele.OnText, func(c tele.Context) error {
		if strings.HasPrefix(c.Text(), "Add") {
			return AddNewNodeHandler(c, environment, requestType, responsePairType)
		}
		if strings.HasPrefix(c.Text(), "Remove") {
			return RemoveNodeHandler(c, environment, requestType, responsePairType)
		}
		if strings.HasPrefix(c.Text(), "Subscribe to") {
			return SubscribeHandler(c, environment)
		}

		if strings.HasPrefix(c.Text(), "Unsubscribe from") {
			return UnsubscribeHandler(c, environment)
		}
		return nil
	})

}

func requestNodesList(requestType chan pair.RequestPair, responsePairType chan pair.ResponsePair) ([]string, error) {
	requestType <- &pair.NodeListRequest{}
	responsePair := <-responsePairType
	nodesList, ok := responsePair.(*pair.NodeListResponse)
	if !ok {
		return nil, errors.New("failed to convert response interface to the node list type")
	}
	return nodesList.Urls, nil
}

func EditPool(
	c tele.Context,
	environment *internal.TelegramBotEnvironment,
	requestType chan pair.RequestPair,
	responsePairType chan pair.ResponsePair) error {

	urls, err := requestNodesList(requestType, responsePairType)
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
	environment *internal.TelegramBotEnvironment) error {
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
