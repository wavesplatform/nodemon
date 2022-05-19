package handlers

import (
	"fmt"
	tele "gopkg.in/telebot.v3"
	"nodemon/cmd/tg_bot/internal"
	"nodemon/cmd/tg_bot/internal/messages"
	"nodemon/pkg/messaging/pair"
	"strings"
)

var addNewNode = "addNewNode"
var deleteNode = "deleteNode"

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

	bot.Handle("\f"+addNewNode, func(c tele.Context) error {
		return c.Send(
			"Please type the url of the node you want to add."+
				"Example: Add <url>",
			&tele.SendOptions{ParseMode: tele.ModeDefault})
	})
	bot.Handle("\f"+deleteNode, func(c tele.Context) error {
		return c.Send(
			"Please type the url of the node you want to remove."+
				"Example: Remove <url>",
			&tele.SendOptions{
				ParseMode: tele.ModeDefault,
			},
		)
	})

	bot.Handle("/editPool", func(c tele.Context) error {
		requestType <- &pair.NodeListRequest{}
		responsePairType := <-responsePairType
		nodesList, ok := responsePairType.(*pair.NodeListResponse)
		if !ok {
			return nil
		}
		//urls := []string{"mainnet.com", "testnet.com", "stagenet.com", "keknet.com"}
		var keyboard = make([][]tele.InlineButton, 0)
		for idx, url := range nodesList.Urls {
			if idx%2 == 0 {
				keyboard = append(keyboard, []tele.InlineButton{})
			}
			keyboard[idx/2] = append(keyboard[idx/2], tele.InlineButton{Text: url})
		}

		markup := &tele.ReplyMarkup{
			InlineKeyboard:  keyboard,
			OneTimeKeyboard: true,
		}
		err := c.Send(
			"Here is the list of nodes being monitored",
			&tele.SendOptions{
				ParseMode:   tele.ModeHTML,
				ReplyMarkup: markup,
			},
		)
		if err != nil {
			return err
		}

		keyboardAddDelete := [][]tele.InlineButton{{
			{
				Text:   "Add new node",
				Unique: addNewNode,
			},
			{
				Text:   "Delete node",
				Unique: deleteNode,
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
			requestType <- &pair.NodeListRequest{}
			responsePairType := <-responsePairType
			nodesList, ok := responsePairType.(*pair.NodeListResponse)
			if !ok {
				return nil
			}
			//urls := []string{"mainnet.com", "testnet.com", "stagenet.com", "keknet.com"}
			var keyboard = make([][]tele.InlineButton, 0)
			for idx, url := range nodesList.Urls {
				if idx%2 == 0 {
					keyboard = append(keyboard, []tele.InlineButton{})
				}
				keyboard[idx/2] = append(keyboard[idx/2], tele.InlineButton{Text: url})
			}
			markup := &tele.ReplyMarkup{
				InlineKeyboard:  keyboard,
				OneTimeKeyboard: true,
			}
			return c.Send(
				"New list of nodes being monitored",
				&tele.SendOptions{
					ParseMode:   tele.ModeHTML,
					ReplyMarkup: markup,
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
				return nil
			}
			requestType <- &pair.NodeListRequest{}
			responsePairType := <-responsePairType
			nodesList, ok := responsePairType.(*pair.NodeListResponse)
			if !ok {
				return nil
			}
			//urls := []string{"mainnet.com", "testnet.com", "stagenet.com", "keknet.com"}
			var keyboard = make([][]tele.InlineButton, 0)
			for idx, url := range nodesList.Urls {
				if idx%2 == 0 {
					keyboard = append(keyboard, []tele.InlineButton{})
				}
				keyboard[idx/2] = append(keyboard[idx/2], tele.InlineButton{Text: url})
			}

			markup := &tele.ReplyMarkup{
				InlineKeyboard:  keyboard,
				OneTimeKeyboard: true,
			}
			return c.Send(
				"New list of nodes being monitored",
				&tele.SendOptions{
					ParseMode:   tele.ModeHTML,
					ReplyMarkup: markup,
				},
			)
		}

		return nil

	})
}
