package internal

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"

	"github.com/pkg/errors"
	"gopkg.in/telebot.v3"
	"nodemon/cmd/tg_bot/internal/messages"
	"nodemon/pkg/entities"
	"nodemon/pkg/messaging"
)

var (
	//go:embed templates
	templateFiles embed.FS
)

var errUnknownAlertType = errors.New("received unknown alert type")

type TelegramBotEnvironment struct {
	ChatID int64
	Bot    *telebot.Bot
	Mute   bool
}

func NewTelegramBotEnvironment(bot *telebot.Bot, chatID int64, mute bool) *TelegramBotEnvironment {
	return &TelegramBotEnvironment{Bot: bot, ChatID: chatID, Mute: mute}
}

func (tgEnv *TelegramBotEnvironment) Start() {
	log.Println("Telegram bot started")
	tgEnv.Bot.Start()
	log.Println("Telegram bot finished")
}

func (tgEnv *TelegramBotEnvironment) makeMessagePretty(alertType entities.AlertType, alert messaging.Alert) messaging.Alert {
	// simple alert is skipped because it needs to be deleted
	switch alertType {
	case entities.UnreachableAlertType, entities.InvalidHeightAlertType, entities.StateHashAlertType, entities.HeightAlertType:
		alert.AlertDescription += fmt.Sprintf(" %s", messages.ErrorMsg)
	case entities.IncompleteAlertType:
		alert.AlertDescription += fmt.Sprintf(" %s", messages.QuestionMsg)
	case entities.AlertFixedType:
		alert.AlertDescription += fmt.Sprintf(" %s", messages.OkMsg)
	default:

	}

	if alert.Severity == entities.InfoLevel {
		alert.Severity += fmt.Sprintf(" %s", messages.InfoMsg)
	}
	if alert.Severity == entities.ErrorLevel {
		alert.Severity += fmt.Sprintf(" %s", messages.ErrorMsg)
	}

	return alert
}

func (tgEnv *TelegramBotEnvironment) constructMessage(alertType entities.AlertType, alertJson []byte) (string, error) {
	alert := messaging.Alert{}
	err := json.Unmarshal(alertJson, &alert)
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal json")
	}

	prettyAlert := tgEnv.makeMessagePretty(alertType, alert)

	tmpl, err := template.ParseFS(templateFiles, "templates/alert.html")

	if err != nil {
		log.Printf("failed to construct a message, %v", err)
		return "", err
	}

	w := &bytes.Buffer{}
	err = tmpl.Execute(w, prettyAlert)
	if err != nil {
		log.Printf("failed to construct a message, %v", err)
		return "", err
	}
	return w.String(), nil
}

func (tgEnv *TelegramBotEnvironment) SendMessage(msg []byte) {
	if tgEnv.Mute {
		log.Printf("received an alert, but asleep now")
		return
	}

	chat := &telebot.Chat{ID: tgEnv.ChatID}

	alertType := entities.AlertType(msg[0])
	_, ok := entities.AlertTypes[alertType]
	if !ok {
		log.Printf("failed to construct message, unknown alert type %c, %v", byte(alertType), errUnknownAlertType)
		_, err := tgEnv.Bot.Send(
			chat,
			errUnknownAlertType.Error(),
			&telebot.SendOptions{ParseMode: telebot.ModeHTML},
		)
		if err != nil {
			log.Printf("failed to send a message to telegram, %v", err)
		}
		return
	}

	messageToBot, err := tgEnv.constructMessage(alertType, msg[1:])
	if err != nil {
		log.Printf("failed to construct message, %v\n", err)
		return
	}
	_, err = tgEnv.Bot.Send(
		chat,
		messageToBot,
		&telebot.SendOptions{ParseMode: telebot.ModeHTML},
	)

	if err != nil {
		log.Printf("failed to send a message to telegram, %v", err)
	}
}

func (tgEnv *TelegramBotEnvironment) IsEligibleForAction(chatID int64) bool {
	return chatID == tgEnv.ChatID
}

type Node struct {
	Url string
}

func (tgEnv *TelegramBotEnvironment) NodesListMessage(urls []string) (string, error) {
	tmpl, err := template.ParseFS(templateFiles, "templates/nodes_list.html")

	if err != nil {
		log.Printf("failed to construct a message, %v", err)
		return "", err
	}
	var nodes []entities.Node
	for _, url := range urls {
		node := entities.Node{URL: url + "\n\n"}
		nodes = append(nodes, node)
	}

	w := &bytes.Buffer{}
	err = tmpl.Execute(w, nodes)
	if err != nil {
		log.Printf("failed to construct a message, %v", err)
		return "", err
	}
	return w.String(), nil
}
