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

func InitHandlers(bot *tele.Bot, environment *internal.TelegramBotEnvironment, requestType chan pair.RequestPair, responsePairType chan pair.ResponsePair) {
	bot.Handle("/chat", func(c tele.Context) error {
		return c.Send(fmt.Sprintf("I am sending alerts through %d chat id", environment.ChatID))
	})

	bot.Handle("/ping", func(c tele.Context) error {
		if environment.Mute {
			return c.Send(messages.PongText + " I am currently sleeping" + messages.SleepingMsg)
		}
		return c.Send(messages.PongText + " I am monitoring" + messages.MonitoringMsg)
	})

	bot.Handle("/start", func(c tele.Context) error {
		if !environment.IsEligibleForAction(c.Chat().ID) {
			return c.Send("Sorry, you have no right to start me")
		}
		if environment.Mute {
			environment.Mute = false
			return c.Send("I had been asleep, but started monitoring now... " + messages.MonitoringMsg)
		}
		return c.Send("I had already been monitoring" + messages.MonitoringMsg)
	})

	bot.Handle("/mute", func(c tele.Context) error {
		if !environment.IsEligibleForAction(c.Chat().ID) {
			return c.Send("Sorry, you have no right to mute me")
		}
		if environment.Mute {
			return c.Send("I had already been sleeping, continue sleeping.." + messages.SleepingMsg)
		}
		environment.Mute = true
		return c.Send("I had been monitoring, but going to sleep now.." + messages.SleepingMsg)
	})

	bot.Handle("/help", func(c tele.Context) error {
		return c.Send(
			messages.HelpInfoText,
			&tele.SendOptions{ParseMode: tele.ModeHTML})
	})

	bot.Handle("\f"+buttons.AddNewNode, func(c tele.Context) error {
		return c.Send(
			messages.AddNewNodeMsg,
			&tele.SendOptions{ParseMode: tele.ModeDefault})
	})
	bot.Handle("\f"+buttons.RemoveNode, func(c tele.Context) error {
		return c.Send(
			messages.RemoveNode,
			&tele.SendOptions{
				ParseMode: tele.ModeDefault,
			},
		)
	})

	bot.Handle("/pool", func(c tele.Context) error {
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
	})
	bot.Handle(tele.OnText, func(c tele.Context) error {

		if strings.HasPrefix(c.Text(), "Add") {
			url := strings.TrimPrefix(c.Text(), "Add ")
			if !strings.HasPrefix(url, "http") && !strings.HasPrefix(url, "https") {
				return c.Send(
					"Sorry, the url seems to be incorrect",
					&tele.SendOptions{ParseMode: tele.ModeDefault},
				)
			}
			requestType <- &pair.InsertNewNodeRequest{Url: url}

			response := fmt.Sprintf("New node '%s' added", url)
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
		if strings.HasPrefix(c.Text(), "Remove") {
			url := strings.TrimPrefix(c.Text(), "Remove ")
			if !strings.HasPrefix(url, "http") && !strings.HasPrefix(url, "https") {
				return c.Send(
					"Sorry, the url seems to be incorrect",
					&tele.SendOptions{ParseMode: tele.ModeDefault},
				)
			}
			requestType <- &pair.DeleteNodeRequest{Url: url}

			response := fmt.Sprintf("Node '%s' deleted", url)
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
