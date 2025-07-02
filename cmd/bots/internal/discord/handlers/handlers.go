package handlers

import (
	"fmt"
	"strings"

	"nodemon/cmd/bots/internal/bots"
	"nodemon/cmd/bots/internal/bots/messaging"
	"nodemon/cmd/bots/internal/discord/messages"
	"nodemon/pkg/messaging/pair"

	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func InitDscHandlers(
	environment *bots.DiscordBotEnvironment,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
	logger *zap.Logger,
) {
	isEligibleForAction := func(m *discordgo.MessageCreate) bool {
		if environment.IsEligibleForAction(m.ChannelID) {
			return true
		}
		_, err := environment.Bot.ChannelMessageSend(m.ChannelID, "Sorry, you have no right to use this command")
		if err != nil {
			logger.Error("Failed to send a message to discord", zap.Error(err), zap.String("channelID", m.ChannelID))
		}
		return false
	}
	environment.Bot.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		switch {
		case m.Author.ID == s.State.User.ID: // ignore self messages
			return
		case m.Content == "/ping":
			handlePingCmd(s, environment, logger)
		case m.Content == "/help":
			handleHelpCmd(s, environment, logger)
		case m.Content == "/status":
			handleStatusCmd(s, requestType, responsePairType, logger, environment)
		case strings.Contains(m.Content, "/add"):
			if isEligibleForAction(m) {
				handleAddCmd(s, m, environment, logger, requestType)
			}
		case strings.Contains(m.Content, "/remove"):
			if isEligibleForAction(m) {
				handleRemoveCmd(s, m, environment, logger, requestType)
			}
		}
	})
}

func handleRemoveCmd(
	s *discordgo.Session,
	m *discordgo.MessageCreate,
	environment *bots.DiscordBotEnvironment,
	logger *zap.Logger,
	requestType chan<- pair.Request,
) {
	url := strings.Replace(m.Content, "/remove ", "", 1)
	if url == "" {
		_, err := s.ChannelMessageSend(environment.ChatID, "Please provide a URL to remove")
		if err != nil {
			logger.Error("failed to send a message to discord", zap.Error(err))
		}
		return
	}
	err := RequestRemoveNode(s, environment, m.ChannelID, requestType, url)
	if err != nil {
		_, sendErr := s.ChannelMessageSend(environment.ChatID, "Failed to remove a node, "+err.Error())
		if sendErr != nil {
			logger.Error("failed to send a message to discord", zap.Error(sendErr))
		}
	}
}

func handleAddCmd(
	s *discordgo.Session,
	m *discordgo.MessageCreate,
	environment *bots.DiscordBotEnvironment,
	logger *zap.Logger,
	requestType chan<- pair.Request,
) {
	url := strings.Replace(m.Content, "/add ", "", 1)
	if url == "" {
		_, err := s.ChannelMessageSend(environment.ChatID, "Please provide a URL to add")
		if err != nil {
			logger.Error("failed to send a message to discord", zap.Error(err))
		}
		return
	}
	err := RequestAddNode(s, environment, m.ChannelID, requestType, url, false)
	if err != nil {
		_, sendErr := s.ChannelMessageSend(environment.ChatID, "Failed to add a node, "+err.Error())
		if sendErr != nil {
			logger.Error("failed to send a message to discord", zap.Error(sendErr))
		}
	}
}

func handleStatusCmd(
	s *discordgo.Session,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
	logger *zap.Logger,
	env *bots.DiscordBotEnvironment,
) {
	nodes, err := messaging.RequestAllNodes(requestType, responsePairType)
	if err != nil {
		logger.Error("failed to get nodes list", zap.Error(err))
	}
	urls := messaging.NodesToUrls(nodes)

	nodesStatus, err := messaging.RequestNodesStatements(requestType, responsePairType, urls)
	if err != nil {
		logger.Error("failed to request nodes status", zap.Error(err))
	}
	msg, statusCondition, err := bots.HandleNodesStatus(nodesStatus, env.TemplatesExtension(), nodes)
	if err != nil {
		logger.Error("failed to handle nodes status", zap.Error(err))
	}
	if statusCondition.AllNodesAreOk {
		msg = fmt.Sprintf("%d %s", statusCondition.NodesNumber, msg)
	}

	msg = fmt.Sprintf("```yaml\n%s\n```", msg)
	_, err = s.ChannelMessageSend(env.ChatID, msg)
	if err != nil {
		logger.Error("failed to send a message to discord", zap.Error(err))
	}
}

func handleHelpCmd(s *discordgo.Session, environment *bots.DiscordBotEnvironment, logger *zap.Logger) {
	_, err := s.ChannelMessageSend(environment.ChatID, messages.HelpInfoText)
	if err != nil {
		logger.Error("failed to send a message to discord", zap.Error(err))
	}
}

func handlePingCmd(s *discordgo.Session, environment *bots.DiscordBotEnvironment, logger *zap.Logger) {
	_, err := s.ChannelMessageSend(environment.ChatID, "Pong!")
	if err != nil {
		logger.Error("failed to send a message to discord", zap.Error(err))
	}
}

func RequestAddNode(
	s *discordgo.Session,
	bot messaging.Bot,
	chatID string,
	requestType chan<- pair.Request,
	url string,
	specific bool,
) error {
	response, err := messaging.AddNewNodeHandler(chatID, bot, requestType, url, specific)
	if err != nil {
		if errors.Is(err, messaging.ErrIncorrectURL) || errors.Is(err, messaging.ErrInsufficientPermissions) {
			_, err = s.ChannelMessageSend(chatID, response)
			if err != nil {
				return errors.Wrap(err, "failed to send a message to discord")
			}
		}
		return errors.Wrap(err, "failed to add a new node")
	}
	_, err = s.ChannelMessageSend(chatID, response)
	if err != nil {
		return errors.Wrap(err, "failed to send a message to discord")
	}
	return nil
}

func RequestRemoveNode(
	s *discordgo.Session,
	bot messaging.Bot,
	chatID string,
	requestType chan<- pair.Request,
	url string,
) error {
	response, err := messaging.RemoveNodeHandler(chatID, bot, requestType, url)
	if err != nil {
		if errors.Is(err, messaging.ErrIncorrectURL) || errors.Is(err, messaging.ErrInsufficientPermissions) {
			_, err = s.ChannelMessageSend(chatID, response)
			if err != nil {
				return errors.Wrap(err, "failed to send a message to discord")
			}
		}
		return errors.Wrap(err, "failed to remove a node")
	}
	_, err = s.ChannelMessageSend(chatID, response)
	if err != nil {
		return errors.Wrap(err, "failed to send a message to discord")
	}
	return nil
}
