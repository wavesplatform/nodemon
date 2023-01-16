package handlers

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"nodemon/cmd/bots/internal/common"
	"nodemon/cmd/bots/internal/common/messaging"
	"nodemon/cmd/bots/internal/discord/messages"
	"nodemon/pkg/messaging/pair"
)

func InitDscHandlers(environment *common.DiscordBotEnvironment, requestType chan<- pair.RequestPair, responsePairType <-chan pair.ResponsePair) {
	environment.Bot.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		if m.Author.ID == s.State.User.ID {
			return
		}
		if m.Content == "/ping" {
			_, err := s.ChannelMessageSend(environment.ChatID, "Pong!")
			if err != nil {
				environment.Zap.Error("failed to send a message to discord", zap.Error(err))
			}
		}

		if m.Content == "/help" {
			_, err := s.ChannelMessageSend(environment.ChatID, messages.HelpInfoText)
			if err != nil {
				environment.Zap.Error("failed to send a message to discord", zap.Error(err))
			}
		}

		if m.Content == "/status" {
			urls, err := messaging.RequestNodesList(requestType, responsePairType, false)
			if err != nil {
				environment.Zap.Error("failed to get a list of nodes", zap.Error(err))
			}
			additionalUrls, err := messaging.RequestNodesList(requestType, responsePairType, true)
			if err != nil {
				environment.Zap.Error("failed to get a list of nodes", zap.Error(err))
			}
			urls = append(urls, additionalUrls...)

			nodesStatus, err := messaging.RequestNodesStatus(requestType, responsePairType, urls)
			if err != nil {
				environment.Zap.Error("failed to request nodes status", zap.Error(err))
			}
			msg, statusCondition, err := common.HandleNodesStatus(nodesStatus, common.Markdown)
			if err != nil {
				environment.Zap.Error("failed to handle nodes status", zap.Error(err))
			}
			if statusCondition.AllNodesAreOk {
				msg = fmt.Sprintf("%d %s", statusCondition.NodesNumber, msg)
			}

			msg, err = common.ReplaceNodesWithAliases(requestType, responsePairType, msg)
			if err != nil {
				environment.Zap.Error("failed to replaces nodes with aliases", zap.Error(err))
			}

			msg = fmt.Sprintf("```yaml\n%s\n```", msg)
			_, err = s.ChannelMessageSend(environment.ChatID, msg)
			if err != nil {
				environment.Zap.Error("failed to send a message to discord", zap.Error(err))
			}
		}

		if strings.Contains(m.Content, "/add") {
			url := strings.Replace(m.Content, "/add ", "", 1)
			if url == "" {
				_, err := s.ChannelMessageSend(environment.ChatID, "Please provide a URL to add")
				if err != nil {
					environment.Zap.Error("failed to send a message to discord", zap.Error(err))
				}
				return
			}
			err := RequestAddNode(s, environment, m.ChannelID, requestType, url, false)
			if err != nil {
				_, err := s.ChannelMessageSend(environment.ChatID, "Failed to add a node, "+err.Error())
				if err != nil {
					environment.Zap.Error("failed to send a message to discord", zap.Error(err))
				}
			}
		}

		if strings.Contains(m.Content, "/remove") {
			url := strings.Replace(m.Content, "/remove ", "", 1)
			if url == "" {
				_, err := s.ChannelMessageSend(environment.ChatID, "Please provide a URL to remove")
				if err != nil {
					environment.Zap.Error("failed to send a message to discord", zap.Error(err))
				}
				return
			}
			err := RequestRemoveNode(s, environment, m.ChannelID, requestType, url)
			if err != nil {
				_, err := s.ChannelMessageSend(environment.ChatID, "Failed to remove a node, "+err.Error())
				if err != nil {
					environment.Zap.Error("failed to send a message to discord", zap.Error(err))
				}
			}
		}
	})
}

func RequestAddNode(s *discordgo.Session, bot messaging.Bot, chatID string, requestType chan<- pair.RequestPair, url string, specific bool) error {
	response, err := messaging.AddNewNodeHandler(chatID, bot, requestType, url, specific)
	if err != nil {
		if err == messaging.IncorrectUrlError || err == messaging.InsufficientPermissionsError {
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

func RequestRemoveNode(s *discordgo.Session, bot messaging.Bot, chatID string, requestType chan<- pair.RequestPair, url string) error {
	response, err := messaging.RemoveNodeHandler(chatID, bot, requestType, url)
	if err != nil {
		if err == messaging.IncorrectUrlError || err == messaging.InsufficientPermissionsError {
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
