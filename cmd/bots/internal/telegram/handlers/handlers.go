package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"nodemon/cmd/bots/internal/common"
	commonMessages "nodemon/cmd/bots/internal/common/messages"
	"nodemon/cmd/bots/internal/common/messaging"
	"nodemon/cmd/bots/internal/telegram/buttons"
	"nodemon/cmd/bots/internal/telegram/messages"
	"nodemon/pkg/entities"
	"nodemon/pkg/messaging/pair"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
)

func InitTgHandlers(
	env *common.TelegramBotEnvironment,
	zapLogger *zap.Logger,
	requestCh chan<- pair.Request,
	responseCh <-chan pair.Response,
) {
	env.Bot.Handle("/chat", func(c tele.Context) error {
		return c.Send(fmt.Sprintf("I am sending alerts through %d chat id", env.ChatID))
	})

	env.Bot.Handle("/ping", pingCmd(env))

	env.Bot.Handle("/start", startCmd(env))

	env.Bot.Handle("/mute", muteCmd(env))

	env.Bot.Handle("/help", func(c tele.Context) error {
		return c.Send(messages.HelpInfoText, &tele.SendOptions{ParseMode: tele.ModeHTML})
	})

	env.Bot.Handle("\f"+buttons.AddNewNode, func(c tele.Context) error {
		return c.Send(messages.AddNewNodeMsg, &tele.SendOptions{ParseMode: tele.ModeDefault})
	})
	env.Bot.Handle("\f"+buttons.RemoveNode, func(c tele.Context) error {
		return c.Send(messages.RemoveNode, &tele.SendOptions{ParseMode: tele.ModeDefault})
	})
	env.Bot.Handle("\f"+buttons.SubscribeTo, func(c tele.Context) error {
		return c.Send(messages.SubscribeTo, &tele.SendOptions{ParseMode: tele.ModeDefault})
	})
	env.Bot.Handle("\f"+buttons.UnsubscribeFrom, func(c tele.Context) error {
		return c.Send(messages.UnsubscribeFrom, &tele.SendOptions{ParseMode: tele.ModeDefault})
	})

	env.Bot.Handle("/pool", func(c tele.Context) error {
		return EditPool(c, env, requestCh, responseCh)
	})
	env.Bot.Handle("/subscriptions", func(c tele.Context) error {
		return EditSubscriptions(c, env)
	})

	env.Bot.Handle("/add", addCmd(env, requestCh, responseCh))

	env.Bot.Handle("/add_specific", addSpecificCmd(env, requestCh, responseCh))

	env.Bot.Handle("/remove", removeCmd(env, requestCh, responseCh))

	env.Bot.Handle("/add_alias", addAliasCmd(env, requestCh))

	env.Bot.Handle("/aliases", aliasesCmd(requestCh, responseCh, zapLogger))

	env.Bot.Handle("/subscribe", subscribeCmd(env))

	env.Bot.Handle("/unsubscribe", unsubscribeCmd(env))

	env.Bot.Handle("/statement", statementCmd(requestCh, responseCh, env.TemplatesExtension(), zapLogger))

	env.Bot.Handle(tele.OnText, onTextMsgHandler(env, requestCh, responseCh))

	env.Bot.Handle("/status", statusCmd(requestCh, responseCh, env.TemplatesExtension(), zapLogger))
}

func removeCmd(
	env *common.TelegramBotEnvironment,
	requestChan chan<- pair.Request,
	responseChan <-chan pair.Response,
) func(c tele.Context) error {
	return func(c tele.Context) error {
		args := c.Args()
		if len(args) != 1 {
			return c.Send(messages.RemovedDoesNotEqualOne, &tele.SendOptions{ParseMode: tele.ModeDefault})
		}
		url := args[0]
		return RemoveNodeHandler(c, env, requestChan, responseChan, url)
	}
}

func addAliasCmd(env *common.TelegramBotEnvironment, requestType chan<- pair.Request) func(c tele.Context) error {
	return func(c tele.Context) error {
		const requiredArgsCount = 2
		args := c.Args()
		if len(args) != requiredArgsCount {
			return c.Send(messages.AliasWrongFormat, &tele.SendOptions{ParseMode: tele.ModeDefault})
		}
		url, alias := args[0], args[1]
		return UpdateAliasHandler(c, env, requestType, url, alias)
	}
}

func aliasesCmd(
	requestChan chan<- pair.Request,
	responseChan <-chan pair.Response,
	zapLogger *zap.Logger,
) func(c tele.Context) error {
	return func(c tele.Context) error {
		nodes, err := messaging.RequestAllNodes(requestChan, responseChan)
		if err != nil {
			zapLogger.Error("failed to request nodes list", zap.Error(err))
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
		return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeHTML})
	}
}

func onTextMsgHandler(
	environment *common.TelegramBotEnvironment,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
) func(c tele.Context) error {
	return func(c tele.Context) error {
		command := strings.ToLower(c.Text())
		switch {
		case strings.HasPrefix(command, "add specific"):
			u := strings.TrimPrefix(command, "add specific ")
			return AddNewNodeHandler(c, environment, requestType, responsePairType, u, true)
		case strings.HasPrefix(command, "add"):
			u := strings.TrimPrefix(command, "add ")
			return AddNewNodeHandler(c, environment, requestType, responsePairType, u, false)
		case strings.HasPrefix(command, "remove"):
			u := strings.TrimPrefix(command, "remove ")
			return RemoveNodeHandler(c, environment, requestType, responsePairType, u)
		case strings.HasPrefix(command, "subscribe to"):
			alertName := strings.TrimSpace(strings.TrimPrefix(command, "subscribe to "))
			return SubscribeHandler(c, environment, entities.AlertName(alertName))
		case strings.HasPrefix(command, "unsubscribe from"):
			alertName := strings.TrimSpace(strings.TrimPrefix(command, "unsubscribe from "))
			return UnsubscribeHandler(c, environment, entities.AlertName(alertName))
		default:
			return nil // do nothing
		}
	}
}

func statusCmd(
	requestChan chan<- pair.Request,
	responsePairType <-chan pair.Response,
	ext common.ExpectedExtension,
	zapLogger *zap.Logger,
) func(c tele.Context) error {
	return func(c tele.Context) error {
		nodes, err := messaging.RequestAllNodes(requestChan, responsePairType)
		if err != nil {
			zapLogger.Error("failed to get nodes list", zap.Error(err))
		}
		urls := messaging.NodesToUrls(nodes)

		nodesStatus, err := messaging.RequestNodesStatus(requestChan, responsePairType, urls)
		if err != nil {
			zapLogger.Error("failed to request status of nodes", zap.Error(err))
			return err
		}

		msg, statusCondition, err := common.HandleNodesStatus(nodesStatus, ext, nodes)
		if err != nil {
			zapLogger.Error("failed to handle status of nodes", zap.Error(err))
			return err
		}

		if statusCondition.AllNodesAreOk {
			msg = fmt.Sprintf("<b>%d</b> %s", statusCondition.NodesNumber, msg)
		}

		return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeHTML})
	}
}

func statementCmd(
	requestChan chan<- pair.Request,
	responseChan <-chan pair.Response,
	ext common.ExpectedExtension,
	zapLogger *zap.Logger,
) func(c tele.Context) error {
	return func(c tele.Context) error {
		args := c.Args()
		if len(args) > 2 || len(args) < 1 {
			return c.Send(messages.StatementWrongFormat, &tele.SendOptions{ParseMode: tele.ModeDefault})
		}
		url := args[0]

		updatedURL, err := entities.CheckAndUpdateURL(url)
		if err != nil {
			return c.Send(messages.InvalidURL, &tele.SendOptions{ParseMode: tele.ModeDefault})
		}
		height, err := strconv.Atoi(args[1])
		if err != nil {
			return c.Send(fmt.Sprintf("failed to parse height: %s", err.Error()),
				&tele.SendOptions{ParseMode: tele.ModeDefault},
			)
		}
		statement, err := messaging.RequestNodeStatement(requestChan, responseChan, updatedURL, height)
		if err != nil {
			zapLogger.Error("failed to request nodes list buttons", zap.Error(err))
			return err
		}

		msg, err := common.HandleNodeStatement(statement, ext)
		if err != nil {
			zapLogger.Error("failed to handle status of nodes", zap.Error(err))
			return err
		}

		return c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeHTML})
	}
}

func unsubscribeCmd(environment *common.TelegramBotEnvironment) func(c tele.Context) error {
	return func(c tele.Context) error {
		args := c.Args()
		if len(args) != 1 {
			return c.Send(messages.SubscribeWrongNumberOfNodes, &tele.SendOptions{ParseMode: tele.ModeDefault})
		}
		alertName := entities.AlertName(args[0])
		return UnsubscribeHandler(c, environment, alertName)
	}
}

func subscribeCmd(environment *common.TelegramBotEnvironment) func(c tele.Context) error {
	return func(c tele.Context) error {
		args := c.Args()
		if len(args) != 1 {
			return c.Send(messages.SubscribeWrongNumberOfNodes, &tele.SendOptions{ParseMode: tele.ModeDefault})
		}
		alertName := entities.AlertName(args[0])
		return SubscribeHandler(c, environment, alertName)
	}
}

func addSpecificCmd(
	environment *common.TelegramBotEnvironment,
	requestChan chan<- pair.Request,
	responseChan <-chan pair.Response,
) func(c tele.Context) error {
	return func(c tele.Context) error {
		args := c.Args()
		if len(args) != 1 {
			return c.Send(messages.AddWrongNumberOfNodes, &tele.SendOptions{ParseMode: tele.ModeDefault})
		}
		url := args[0]
		response := AddNewNodeHandler(c, environment, requestChan, responseChan, url, true)
		return c.Send(response, &tele.SendOptions{ParseMode: tele.ModeHTML})
	}
}

func addCmd(
	environment *common.TelegramBotEnvironment,
	requestChan chan<- pair.Request,
	responseChan <-chan pair.Response,
) func(c tele.Context) error {
	return func(c tele.Context) error {
		args := c.Args()
		if len(args) != 1 {
			return c.Send(messages.AddWrongNumberOfNodes, &tele.SendOptions{ParseMode: tele.ModeDefault})
		}
		url := args[0]

		response := AddNewNodeHandler(c, environment, requestChan, responseChan, url, false)
		err := c.Send(response, &tele.SendOptions{ParseMode: tele.ModeHTML})
		if err != nil {
			return err
		}
		urls, err := messaging.RequestNodes(requestChan, responseChan, false)
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
}

func muteCmd(environment *common.TelegramBotEnvironment) func(c tele.Context) error {
	return func(c tele.Context) error {
		if !environment.IsEligibleForAction(strconv.FormatInt(c.Chat().ID, 10)) {
			return c.Send("Sorry, you have no right to mute me")
		}
		if environment.Mute {
			return c.Send("I had already been sleeping, continue sleeping.." + commonMessages.SleepingMsg)
		}
		environment.Mute = true
		return c.Send("I had been monitoring, but going to sleep now.." + commonMessages.SleepingMsg)
	}
}

func startCmd(environment *common.TelegramBotEnvironment) func(c tele.Context) error {
	return func(c tele.Context) error {
		if !environment.IsEligibleForAction(strconv.FormatInt(c.Chat().ID, 10)) {
			return c.Send("Sorry, you have no right to start me")
		}
		if environment.Mute {
			environment.Mute = false
			return c.Send("I had been asleep, but started monitoring now... " + commonMessages.MonitoringMsg)
		}
		return c.Send("I had already been monitoring" + commonMessages.MonitoringMsg)
	}
}

func pingCmd(environment *common.TelegramBotEnvironment) func(c tele.Context) error {
	return func(c tele.Context) error {
		if environment.Mute {
			return c.Send(messages.PongText + " I am currently sleeping" + commonMessages.SleepingMsg)
		}
		return c.Send(messages.PongText + " I am monitoring" + commonMessages.MonitoringMsg)
	}
}

func EditPool(
	c tele.Context,
	environment *common.TelegramBotEnvironment,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
) error {
	nodes, err := messaging.RequestAllNodes(requestType, responsePairType)
	if err != nil {
		return errors.Wrap(err, "failed to request nodes list buttons")
	}
	message, err := environment.NodesListMessage(nodes)
	if err != nil {
		return errors.Wrap(err, "failed to construct nodes list message")
	}
	err = c.Send(message, &tele.SendOptions{ParseMode: tele.ModeHTML})
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

	return c.Send("Please choose",
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
	err = c.Send(msg, &tele.SendOptions{ParseMode: tele.ModeHTML})
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

	return c.Send("Please choose",
		&tele.SendOptions{
			ParseMode: tele.ModeHTML,
			ReplyMarkup: &tele.ReplyMarkup{
				InlineKeyboard:  keyboardSubUnsub,
				ResizeKeyboard:  true,
				OneTimeKeyboard: true},
		},
	)
}
