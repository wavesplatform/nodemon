package handlers

import (
	"fmt"
	"log"

	"github.com/bwmarrin/discordgo"
	"nodemon/cmd/bots/internal/common"
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
				log.Printf("failed to send a message to discord, %v", err)
			}
		}

		if m.Content == "/status" {
			urls, err := common.RequestNodesList(requestType, responsePairType, false)
			if err != nil {
				log.Printf("failed to request list of nodes, %v", err)
			}
			additionalUrls, err := common.RequestNodesList(requestType, responsePairType, true)
			if err != nil {
				log.Printf("failed to request list of specific nodes, %v", err)
			}
			urls = append(urls, additionalUrls...)

			nodesStatus, err := common.RequestNodesStatus(requestType, responsePairType, urls)
			if err != nil {
				log.Printf("failed to request status of nodes, %v", err)
			}
			msg, statusCondition, err := common.HandleNodesStatus(nodesStatus, common.Markdown)
			if err != nil {
				log.Printf("failed to handle status of nodes, %v", err)
			}
			if statusCondition.AllNodesAreOk {
				msg = fmt.Sprintf("%d %s", statusCondition.NodesNumber, msg)
			}

			msg = fmt.Sprintf("```yaml\n%s\n```", msg)
			_, err = s.ChannelMessageSend(environment.ChatID, msg)
			if err != nil {
				log.Printf("failed to send a message to discord, %v", err)
			}
		}
	})
}
