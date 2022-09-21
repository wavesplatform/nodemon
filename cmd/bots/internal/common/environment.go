package common

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	htmlTemplate "html/template"
	"log"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	textTemplate "text/template"

	"github.com/bwmarrin/discordgo"
	"github.com/pkg/errors"
	"github.com/procyon-projects/chrono"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol"
	"gopkg.in/telebot.v3"
	"nodemon/cmd/bots/internal/common/messaging"
	"nodemon/cmd/bots/internal/telegram/messages"
	"nodemon/pkg/entities"
	generalMessaging "nodemon/pkg/messaging"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/storing/events"
)

const (
	scheduledTimeExpression = "0 0 9 * * *" // 12:00 UTC+3
)

var (
	//go:embed templates
	templateFiles embed.FS
)

type expectedExtension string

const (
	Html     expectedExtension = ".html"
	Markdown expectedExtension = ".md"
)

var errUnknownAlertType = errors.New("received unknown alert type")

type subscriptions struct {
	mu   *sync.RWMutex
	subs map[entities.AlertType]string
}

func (s *subscriptions) Add(alertType entities.AlertType, alertName string) {
	s.mu.Lock()
	s.subs[alertType] = alertName
	s.mu.Unlock()
}

// Read returns alert name
func (s *subscriptions) Read(alertType entities.AlertType) (string, bool) {
	s.mu.RLock()
	elem, ok := s.subs[alertType]
	s.mu.RUnlock()
	return elem, ok
}

func (s *subscriptions) Delete(alertType entities.AlertType) {
	s.mu.Lock()
	delete(s.subs, alertType)
	s.mu.Unlock()
}

func (s *subscriptions) MapR(f func()) {
	s.mu.RLock()
	f()
	s.mu.RUnlock()
}

type DiscordBotEnvironment struct {
	ChatID        string
	Bot           *discordgo.Session
	subSocket     protocol.Socket
	Subscriptions subscriptions
}

func NewDiscordBotEnvironment(bot *discordgo.Session, chatID string) *DiscordBotEnvironment {
	return &DiscordBotEnvironment{Bot: bot, ChatID: chatID, Subscriptions: subscriptions{subs: make(map[entities.AlertType]string), mu: new(sync.RWMutex)}}
}

func (dscBot *DiscordBotEnvironment) Start() error {
	log.Println("Discord bot started")
	err := dscBot.Bot.Open()
	if err != nil {
		fmt.Println("failed to open connection to discord", err)
		return err
	}
	return nil
}

func (dscBot *DiscordBotEnvironment) SetSubSocket(subSocket protocol.Socket) {
	dscBot.subSocket = subSocket
}

func (dscBot *DiscordBotEnvironment) SendMessage(msg string) {

	_, err := dscBot.Bot.ChannelMessageSend(dscBot.ChatID, msg)

	// TODO change to error type of log
	if err != nil {
		log.Printf("failed to send a message to telegram, %v", err)
	}
}

func (dscBot *DiscordBotEnvironment) SendAlertMessage(msg []byte) {
	if len(msg) == 0 {
		log.Println("received an empty message")
		return
	}
	alertType := entities.AlertType(msg[0])
	_, ok := entities.AlertTypes[alertType]
	if !ok {
		log.Printf("failed to construct message, unknown alert type %c, %v", byte(alertType), errUnknownAlertType)

		_, err := dscBot.Bot.ChannelMessageSend(dscBot.ChatID, errUnknownAlertType.Error())

		if err != nil {
			log.Printf("failed to send a message to discord, %v", err)
		}
		return
	}

	messageToBot, err := constructMessage(alertType, msg[1:], Markdown)
	if err != nil {
		log.Printf("failed to construct message, %v\n", err)
		return
	}
	_, err = dscBot.Bot.ChannelMessageSend(dscBot.ChatID, messageToBot)

	if err != nil {
		log.Printf("failed to send a message to discord, %v", err)
	}
}

func (dscBot *DiscordBotEnvironment) SubscribeToAllAlerts() error {

	for alertType, alertName := range entities.AlertTypes {
		if dscBot.IsAlreadySubscribed(alertType) {
			return errors.Errorf("failed to subscribe to %s, already subscribed to it", alertName)
		}
		err := dscBot.subSocket.SetOption(mangos.OptionSubscribe, []byte{byte(alertType)})
		if err != nil {
			return err
		}
		dscBot.Subscriptions.Add(alertType, alertName)
		log.Printf("discord bot subscribed to %s", alertName)
	}

	return nil
}

func (dscBot *DiscordBotEnvironment) IsAlreadySubscribed(alertType entities.AlertType) bool {
	_, ok := dscBot.Subscriptions.Read(alertType)
	return ok
}

func (dscBot *DiscordBotEnvironment) IsEligibleForAction(chatID string) bool {
	return chatID == dscBot.ChatID
}

type TelegramBotEnvironment struct {
	ChatID        int64
	Bot           *telebot.Bot
	Mute          bool // If it used elsewhere, should be protected by mutex
	subSocket     protocol.Socket
	subscriptions subscriptions
}

func NewTelegramBotEnvironment(bot *telebot.Bot, chatID int64, mute bool) *TelegramBotEnvironment {
	return &TelegramBotEnvironment{Bot: bot, ChatID: chatID, Mute: mute, subscriptions: subscriptions{subs: make(map[entities.AlertType]string), mu: new(sync.RWMutex)}}
}

func (tgEnv *TelegramBotEnvironment) Start() error {
	log.Println("Telegram bot started")
	tgEnv.Bot.Start()
	log.Println("Telegram bot finished")
	return nil
}

func (tgEnv *TelegramBotEnvironment) SetSubSocket(subSocket protocol.Socket) {
	tgEnv.subSocket = subSocket
}

func (tgEnv *TelegramBotEnvironment) SendAlertMessage(msg []byte) {
	if tgEnv.Mute {
		log.Printf("received an alert, but asleep now")
		return
	}

	if len(msg) == 0 {
		log.Println("received an empty message")
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

	messageToBot, err := constructMessage(alertType, msg[1:], Html)
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

func (tgEnv *TelegramBotEnvironment) SendMessage(msg string) {
	if tgEnv.Mute {
		log.Printf("received an alert, but asleep now")
		return
	}

	chat := &telebot.Chat{ID: tgEnv.ChatID}

	_, err := tgEnv.Bot.Send(
		chat,
		msg,
		&telebot.SendOptions{ParseMode: telebot.ModeHTML},
	)

	if err != nil {
		log.Printf("failed to send a message to telegram, %v", err)
	}
}

func (tgEnv *TelegramBotEnvironment) IsEligibleForAction(chatID string) bool {
	return chatID == strconv.FormatInt(tgEnv.ChatID, 10)
}

func (tgEnv *TelegramBotEnvironment) NodesListMessage(urls []string) (string, error) {
	tmpl, err := htmlTemplate.ParseFS(templateFiles, "templates/nodes_list.html")

	if err != nil {
		log.Printf("failed to construct a message, %v", err)
		return "", err
	}
	var nodes []entities.Node
	for _, u := range urls {
		host, err := RemoveSchemePrefix(u)
		if err != nil {
			return "", err
		}
		node := entities.Node{URL: host + "\n\n"}
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
	for alertType, alertName := range entities.AlertTypes {
		if tgEnv.IsAlreadySubscribed(alertType) {
			return errors.Errorf("failed to subscribe to %s, already subscribed to it", alertName)
		}
		err := tgEnv.subSocket.SetOption(mangos.OptionSubscribe, []byte{byte(alertType)})
		if err != nil {
			return err
		}
		tgEnv.subscriptions.Add(alertType, alertName)
		log.Printf("telegram bot subscribed to %s", alertName)
	}

	return nil
}

func (tgEnv *TelegramBotEnvironment) SubscribeToAlert(alertType entities.AlertType) error {
	alertName, ok := entities.AlertTypes[alertType] // check if such an alert exists
	if !ok {
		return errors.New("failed to subscribe to alert, unknown alert type")
	}

	if tgEnv.IsAlreadySubscribed(alertType) {
		return errors.Errorf("failed to subscribe to %s, already subscribed to it", alertName)
	}

	err := tgEnv.subSocket.SetOption(mangos.OptionSubscribe, []byte{byte(alertType)})
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to alert")
	}
	tgEnv.subscriptions.Add(alertType, alertName)
	log.Printf("Subscribed to %s", alertName)
	return nil
}

func (tgEnv *TelegramBotEnvironment) UnsubscribeFromAlert(alertType entities.AlertType) error {
	alertName, ok := entities.AlertTypes[alertType] // check if such an alert exists
	if !ok {
		return errors.New("failed to unsubscribe from alert, unknown alert type")
	}

	if !tgEnv.IsAlreadySubscribed(alertType) {
		return errors.Errorf("failed to unsubscribe from %s, was not subscribed to it", alertName)
	}

	err := tgEnv.subSocket.SetOption(mangos.OptionUnsubscribe, []byte{byte(alertType)})
	if err != nil {
		return errors.Wrap(err, "failed to unsubscribe from alert")
	}
	ok = tgEnv.IsAlreadySubscribed(alertType)
	if !ok {
		return errors.New("failed to unsubscribe from alert: was not subscribed to it")
	}
	tgEnv.subscriptions.Delete(alertType)
	log.Printf("Unsubscribed from %s", alertName)
	return nil
}

type Subscribed struct {
	AlertName string
}

type Unsubscribed struct {
	AlertName string
}

func (tgEnv *TelegramBotEnvironment) SubscriptionsList() (string, error) {
	tmpl, err := htmlTemplate.ParseFS(templateFiles, "templates/subscriptions.html")

	if err != nil {
		log.Printf("failed to construct a message, %v", err)
		return "", err
	}
	var subscribedTo []Subscribed
	tgEnv.subscriptions.MapR(func() {
		for _, alertName := range tgEnv.subscriptions.subs {
			s := Subscribed{AlertName: alertName + "\n\n"}
			subscribedTo = append(subscribedTo, s)
		}
	})

	var unsubscribedFrom []Unsubscribed
	for alertType, alertName := range entities.AlertTypes {
		ok := tgEnv.IsAlreadySubscribed(alertType)
		if !ok { // find those alerts that are not in the subscriptions list
			u := Unsubscribed{AlertName: alertName + "\n\n"}
			unsubscribedFrom = append(unsubscribedFrom, u)
		}
	}
	type subscriptionsList struct {
		SubscribedTo     []Subscribed
		UnsubscribedFrom []Unsubscribed
	}
	subscriptions := subscriptionsList{SubscribedTo: subscribedTo, UnsubscribedFrom: unsubscribedFrom}

	w := &bytes.Buffer{}
	err = tmpl.Execute(w, subscriptions)
	if err != nil {
		log.Printf("failed to construct a message, %v", err)
		return "", err
	}
	return w.String(), nil
}

func (tgEnv *TelegramBotEnvironment) IsAlreadySubscribed(alertType entities.AlertType) bool {
	_, ok := tgEnv.subscriptions.Read(alertType)
	return ok
}

func ScheduleNodesStatus(
	taskScheduler chrono.TaskScheduler,
	requestType chan<- pair.RequestPair,
	responsePairType <-chan pair.ResponsePair, bot messaging.Bot) error {

	_, err := taskScheduler.ScheduleWithCron(func(ctx context.Context) {
		urls, err := messaging.RequestNodesList(requestType, responsePairType, false)
		if err != nil {
			log.Printf("failed to request list of nodes, %v", err)
		}
		additionalUrls, err := messaging.RequestNodesList(requestType, responsePairType, true)
		if err != nil {
			log.Printf("failed to request list of additional nodes, %v", err)
		}
		urls = append(urls, additionalUrls...)

		nodesStatus, err := messaging.RequestNodesStatus(requestType, responsePairType, urls)
		if err != nil {
			log.Printf("failed to request status of nodes, %v", err)
		}

		var handledNodesStatus string
		statusCondition := StatusCondition{AllNodesAreOk: false, NodesNumber: 0, Height: ""}
		switch bot.(type) {
		case *TelegramBotEnvironment:
			handledNodesStatus, statusCondition, err = HandleNodesStatus(nodesStatus, Html)
			if err != nil {
				log.Printf("failed to handle status of nodes, %v", err)
			}
		case *DiscordBotEnvironment:
			handledNodesStatus, statusCondition, err = HandleNodesStatus(nodesStatus, Markdown)
			if err != nil {
				log.Printf("failed to handle status of nodes, %v", err)
			}
		default:
			// TODO make it an error log
			log.Println("failed to schedule nodes status, unknown bot type")
			return
		}

		if statusCondition.AllNodesAreOk {
			var msg string
			shortOkNodes := struct {
				TimeEmoji   string
				NodesNumber int
				Height      string
			}{
				TimeEmoji:   messages.TimerMsg,
				NodesNumber: statusCondition.NodesNumber,
				Height:      statusCondition.Height,
			}
			switch bot.(type) {
			case *TelegramBotEnvironment:
				msg, err = executeTemplate("templates/nodes_status_ok_short", shortOkNodes, Html)
				if err != nil {
					log.Printf("failed to construct a message, %v", err)
				}
			case *DiscordBotEnvironment:
				msg, err = executeTemplate("templates/nodes_status_ok_short", shortOkNodes, Markdown)
				if err != nil {
					log.Printf("failed to construct a message, %v", err)
				}
			default:
				log.Println("failed to schedule nodes status, unknown bot type")
				return
			}
			bot.SendMessage(msg)
			return
		}
		var msg string
		switch bot.(type) {
		case *TelegramBotEnvironment:
			msg = fmt.Sprintf("Status %s\n\n%s", messages.TimerMsg, handledNodesStatus)
		case *DiscordBotEnvironment:
			msg = fmt.Sprintf("```yaml\nStatus %s\n\n%s\n```", messages.TimerMsg, handledNodesStatus)
		default:
			log.Println("failed to schedule nodes status, unknown bot type")
			return
		}
		bot.SendMessage(msg)

	}, scheduledTimeExpression)

	if err != nil {
		return err
	}
	return nil
}

type NodeStatus struct {
	URL     string
	Sumhash string
	Status  string
	Height  string
}

func sortNodesStatuses(statuses []NodeStatus) {
	sort.Slice(statuses, func(i, j int) bool {
		return strings.Compare(statuses[i].URL, statuses[j].URL) < 0
	})
}

func executeTemplate(template string, data any, extension expectedExtension) (string, error) {
	switch extension {
	case Html:
		tmpl, err := htmlTemplate.ParseFS(templateFiles, template+string(extension))
		if err != nil {
			log.Printf("failed to parse template file, %v", err)
			return "", err
		}
		buffer := &bytes.Buffer{}
		err = tmpl.Execute(buffer, data)
		if err != nil {
			log.Printf("failed to execute a template, %v", err)
			return "", err
		}
		return buffer.String(), nil
	case Markdown:
		tmpl, err := textTemplate.ParseFS(templateFiles, template+string(extension))
		if err != nil {
			log.Printf("failed to parse template file, %v", err)
			return "", err
		}
		buffer := &bytes.Buffer{}
		err = tmpl.Execute(buffer, data)
		if err != nil {
			log.Printf("failed to execute a template, %v", err)
			return "", err
		}
		return buffer.String(), nil
	default:
		return "", errors.New("unknown message type to execute a template")
	}

}

type StatusCondition struct {
	AllNodesAreOk bool
	NodesNumber   int
	Height        string
}

func HandleNodesStatusError(nodesStatusResp *pair.NodesStatusResponse, extension expectedExtension) (string, StatusCondition, error) {
	statusCondition := StatusCondition{AllNodesAreOk: false, NodesNumber: 0, Height: ""}

	var differentHeightsNodes []NodeStatus
	var unavailableNodes []NodeStatus

	if errors.Is(nodesStatusResp.Err, events.BigHeightDifference) {
		for _, stat := range nodesStatusResp.NodesStatus {
			s := NodeStatus{}
			if stat.Status != entities.OK {
				s.URL = stat.Url
				unavailableNodes = append(unavailableNodes, s)
				continue
			}
			height := strconv.Itoa(stat.Height)
			s.Height = height
			s.URL = stat.Url

			differentHeightsNodes = append(differentHeightsNodes, s)
		}
		var msg string
		if len(unavailableNodes) != 0 {
			sortNodesStatuses(unavailableNodes)
			unavailableNodes, err := executeTemplate("templates/nodes_status_unavailable", unavailableNodes, extension)
			if err != nil {
				log.Printf("failed to construct a message, %v", err)
				return "", statusCondition, err
			}
			msg = fmt.Sprintf(unavailableNodes + "\n")
		}
		sortNodesStatuses(differentHeightsNodes)
		differentHeights, err := executeTemplate("templates/nodes_status_different_heights", differentHeightsNodes, extension)
		if err != nil {
			log.Printf("failed to construct a message, %v", err)
			return "", statusCondition, err
		}
		msg += fmt.Sprint(differentHeights)
		return fmt.Sprintf("%s\n\n%s", nodesStatusResp.Err.Error(), msg), statusCondition, nil
	}
	return nodesStatusResp.Err.Error(), statusCondition, nil
}

func HandleNodesStatus(nodesStatusResp *pair.NodesStatusResponse, extension expectedExtension) (string, StatusCondition, error) {
	statusCondition := StatusCondition{AllNodesAreOk: false, NodesNumber: 0, Height: ""}

	if nodesStatusResp.Err != nil {
		return HandleNodesStatusError(nodesStatusResp, extension)
	}

	var msg string
	var unavailableNodes []NodeStatus
	var okNodes []NodeStatus
	var height string
	for _, stat := range nodesStatusResp.NodesStatus {
		s := NodeStatus{}
		if stat.Status != entities.OK {
			s.URL = stat.Url
			unavailableNodes = append(unavailableNodes, s)
			continue
		}
		height = strconv.Itoa(stat.Height)
		s.Sumhash = stat.StateHash.SumHash.String()
		s.URL = stat.Url
		s.Status = string(stat.Status)
		okNodes = append(okNodes, s)
	}
	if len(unavailableNodes) != 0 {
		sortNodesStatuses(unavailableNodes)
		unavailableNodes, err := executeTemplate("templates/nodes_status_unavailable", unavailableNodes, extension)
		if err != nil {
			log.Printf("failed to construct a message, %v", err)
			return "", statusCondition, err
		}
		msg = fmt.Sprintf(unavailableNodes + "\n")
	}
	areHashesEqual := true
	previousHash := okNodes[0].Sumhash
	for _, node := range okNodes {
		if node.Sumhash != previousHash {
			areHashesEqual = false
		}
		previousHash = node.Sumhash
	}

	sortNodesStatuses(okNodes)
	if !areHashesEqual {
		differentHashes, err := executeTemplate("templates/nodes_status_different_hashes", okNodes, extension)
		if err != nil {
			log.Printf("failed to construct a message, %v", err)
			return "", statusCondition, err
		}
		msg += fmt.Sprintf("%s %s", differentHashes, height)
		return msg, statusCondition, nil
	}
	equalHashes, err := executeTemplate("templates/nodes_status_ok", okNodes, extension)
	if err != nil {
		log.Printf("failed to construct a message, %v", err)
		return "", statusCondition, err
	}

	if len(unavailableNodes) == 0 && len(okNodes) != 0 && areHashesEqual {
		statusCondition.AllNodesAreOk = true
		statusCondition.NodesNumber = len(okNodes)
		statusCondition.Height = height
	}

	msg += fmt.Sprintf("%s %s", equalHashes, height)
	return msg, statusCondition, nil
}

func constructMessage(alertType entities.AlertType, alertJson []byte, extension expectedExtension) (string, error) {
	alert := generalMessaging.Alert{}
	err := json.Unmarshal(alertJson, &alert)
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal json")
	}

	prettyAlert := makeMessagePretty(alertType, alert)

	msg, err := executeTemplate("templates/alert", prettyAlert, extension)
	if err != nil {
		log.Printf("failed to construct a message, %v", err)
		return "", err
	}
	return msg, nil
}

func makeMessagePretty(alertType entities.AlertType, alert generalMessaging.Alert) generalMessaging.Alert {
	alert.Details = strings.ReplaceAll(alert.Details, entities.HttpScheme+"://", "")
	alert.Details = strings.ReplaceAll(alert.Details, entities.HttpsScheme+"://", "")
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
	switch alert.Level {
	case entities.InfoLevel:
		alert.Level += fmt.Sprintf(" %s", messages.InfoMsg)
	case entities.ErrorLevel:
		alert.Level += fmt.Sprintf(" %s", messages.ErrorOrDeleteMsg)
	default:
	}

	return alert
}

func FindAlertTypeByName(alertName string) (entities.AlertType, bool) {
	for key, val := range entities.AlertTypes {
		if val == alertName {
			return key, true
		}
	}
	return 0, false

}

func RemoveSchemePrefix(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", errors.Wrapf(err, "failed to parse URL %s", s)
	}
	return u.Host, nil
}
