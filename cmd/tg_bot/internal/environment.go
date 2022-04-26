package internal

import (
	"bytes"
	"embed"
	"html/template"
	"log"

	"github.com/pkg/errors"
	"gopkg.in/telebot.v3"
	"nodemon/pkg/entities"
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

type Alert struct {
	AlertType string
	Severity  string
	Details   string
}

func NewTelegramBotEnvironment(bot *telebot.Bot, chatID int64, mute bool) *TelegramBotEnvironment {
	return &TelegramBotEnvironment{Bot: bot, ChatID: chatID, Mute: mute}
}

func (tgEnv *TelegramBotEnvironment) Start() {
	log.Println("Telegram bot started")
	tgEnv.Bot.Start()
	log.Println("Telegram bot finished")
}

func (tgEnv *TelegramBotEnvironment) constructMessage(alert string, msg []byte) (string, error) {
	a := Alert{
		AlertType: alert,
		Severity:  "Error",
		Details:   string(msg),
	}

	tmpl, err := template.ParseFS(templateFiles, "templates/alert.html")

	if err != nil {
		log.Printf("failed to construct a message, %v", err)
		return "", err
	}

	w := &bytes.Buffer{}
	err = tmpl.Execute(w, a)
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

	alertDescription, ok := entities.AlertTypes[entities.AlertType(msg[0])]
	if !ok {
		log.Printf("failed to construct message, %v", errUnknownAlertType)
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

	messageToBot, err := tgEnv.constructMessage(alertDescription, msg[1:])
	if err != nil {
		log.Printf("failed to construct message, %v", err)
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
