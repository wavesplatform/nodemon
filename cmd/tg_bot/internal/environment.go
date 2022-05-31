package internal

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"

	"github.com/pkg/errors"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol"
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
	ChatID        int64
	Bot           *telebot.Bot
	Mute          bool
	pubSubSocket  protocol.Socket
	subscriptions map[entities.AlertType]string
}

func NewTelegramBotEnvironment(bot *telebot.Bot, chatID int64, mute bool) *TelegramBotEnvironment {
	return &TelegramBotEnvironment{Bot: bot, ChatID: chatID, Mute: mute, subscriptions: make(map[entities.AlertType]string)}
}

func (tgEnv *TelegramBotEnvironment) Start() {
	log.Println("Telegram bot started")
	tgEnv.Bot.Start()
	log.Println("Telegram bot finished")
}

func (tgEnv *TelegramBotEnvironment) SetPubSubSocket(pubSubSocket protocol.Socket) {
	tgEnv.pubSubSocket = pubSubSocket
}

func (tgEnv *TelegramBotEnvironment) makeMessagePretty(alertType entities.AlertType, alert messaging.Alert) messaging.Alert {
	// simple alert is skipped because it needs to be deleted
	switch alertType {
	case entities.UnreachableAlertType, entities.InvalidHeightAlertType, entities.StateHashAlertType, entities.HeightAlertType:
		alert.AlertDescription += fmt.Sprintf(" %s", messages.ErrorOrDeleteMsg)
	case entities.IncompleteAlertType:
		alert.AlertDescription += fmt.Sprintf(" %s", messages.QuestionMsg)
	case entities.AlertFixedType:
		alert.AlertDescription += fmt.Sprintf(" %s", messages.OkMsg)
	default:

	}

	if alert.Level == entities.InfoLevel {
		alert.Level += fmt.Sprintf(" %s", messages.InfoMsg)
	}
	if alert.Level == entities.ErrorLevel {
		alert.Level += fmt.Sprintf(" %s", messages.ErrorOrDeleteMsg)
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

func (tgEnv *TelegramBotEnvironment) SubscribeToAllAlerts() error {
	for alertType := range entities.AlertTypes {
		alertName, ok := entities.AlertTypes[alertType] // check if such an alert exists
		if !ok {
			return errors.New("failed to subscribe to alerts, unknown alert type")
		}

		err := tgEnv.pubSubSocket.SetOption(mangos.OptionSubscribe, []byte{byte(alertType)})
		if err != nil {
			return err
		}
		tgEnv.subscriptions[alertType] = alertName
		log.Printf("Subscribed to %s", alertName)
	}

	return nil
}

func (tgEnv *TelegramBotEnvironment) SubscribeToAlert(alertType entities.AlertType) error {
	alertName, ok := entities.AlertTypes[alertType] // check if such an alert exists
	if !ok {
		return errors.New("failed to subscribe to alert, unkown alert type")
	}

	err := tgEnv.pubSubSocket.SetOption(mangos.OptionSubscribe, []byte{byte(alertType)})
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to alert")
	}

	tgEnv.subscriptions[alertType] = alertName
	log.Printf("Subscribed to %s", alertName)
	return nil
}

func (tgEnv *TelegramBotEnvironment) UnubscribeFromAlert(alertType entities.AlertType) error {
	alertName, ok := entities.AlertTypes[alertType] // check if such an alert exists
	if !ok {
		return errors.New("failed to unsubscribe from alert, unkown alert type")
	}
	err := tgEnv.pubSubSocket.SetOption(mangos.OptionUnsubscribe, []byte{byte(alertType)})
	if err != nil {
		return errors.Wrap(err, "failed to unsubscribe from alert")
	}

	if _, ok := tgEnv.subscriptions[alertType]; !ok {
		return errors.New("failed to unsubscribe from alert: was not subscribed to it")
	}
	delete(tgEnv.subscriptions, alertType)
	log.Printf("Unsubscribed from %s", alertName)
	return nil
}

type Subscribed struct {
	AlertName string
}

type Unsubscribed struct {
	AlertName string
}

type Subscriptions struct {
	SubscribedTo     []Subscribed
	UnsubscribedFrom []Unsubscribed
}

func (tgEnv *TelegramBotEnvironment) SubscriptionsList() (string, error) {
	tmpl, err := template.ParseFS(templateFiles, "templates/subscriptions.html")

	if err != nil {
		log.Printf("failed to construct a message, %v", err)
		return "", err
	}
	var subcribedTo []Subscribed
	for _, alertName := range tgEnv.subscriptions {
		s := Subscribed{AlertName: alertName + "\n\n"}
		subcribedTo = append(subcribedTo, s)
	}

	var unsusbcribedFrom []Unsubscribed
	for alertType, alertName := range entities.AlertTypes {
		if _, ok := tgEnv.subscriptions[alertType]; !ok { // find those alerts that are not in the subscriptions list
			u := Unsubscribed{AlertName: alertName + "\n\n"}
			unsusbcribedFrom = append(unsusbcribedFrom, u)
		}
	}

	subscriptions := Subscriptions{SubscribedTo: subcribedTo, UnsubscribedFrom: unsusbcribedFrom}

	w := &bytes.Buffer{}
	err = tmpl.Execute(w, subscriptions)
	if err != nil {
		log.Printf("failed to construct a message, %v", err)
		return "", err
	}
	return w.String(), nil
}

func (tgEnv *TelegramBotEnvironment) IsAlreadySubscribed(alertType entities.AlertType) bool {
	if _, ok := tgEnv.subscriptions[alertType]; ok {
		return true
	}
	return false
}

func (tgEnv *TelegramBotEnvironment) AlertExists(alertType entities.AlertType) bool {
	if _, ok := entities.AlertTypes[alertType]; ok {
		return true
	}
	return false
}

func (tgEnv *TelegramBotEnvironment) FindAlertTypeByName(alertName string) (entities.AlertType, bool) {
	for key, val := range entities.AlertTypes {
		if val == alertName {
			return key, true
		}
	}
	return 0, false

}
