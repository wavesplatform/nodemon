package internal

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/procyon-projects/chrono"
	"go.nanomsg.org/mangos/v3"
	"go.nanomsg.org/mangos/v3/protocol"
	"gopkg.in/telebot.v3"
	"nodemon/cmd/tg_bot/internal/messages"
	"nodemon/pkg/entities"
	"nodemon/pkg/messaging"
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

var errUnknownAlertType = errors.New("received unknown alert type")

type subsciptions struct {
	mu   *sync.RWMutex
	subs map[entities.AlertType]string
}

func (s *subsciptions) Add(alertType entities.AlertType, alertName string) {
	s.mu.Lock()
	s.subs[alertType] = alertName
	s.mu.Unlock()
}

// Read returns alert name
func (s *subsciptions) Read(alertType entities.AlertType) (string, bool) {
	s.mu.RLock()
	elem, ok := s.subs[alertType]
	s.mu.RUnlock()
	return elem, ok
}

func (s *subsciptions) Delete(alertType entities.AlertType) {
	s.mu.Lock()
	delete(s.subs, alertType)
	s.mu.Unlock()
}

func (s *subsciptions) MapR(f func()) {
	s.mu.RLock()
	f()
	s.mu.RUnlock()
}

type TelegramBotEnvironment struct {
	ChatID        int64
	Bot           *telebot.Bot
	Mute          bool // If it used elsewhere, should be protected by mutex
	pubSubSocket  protocol.Socket
	subscriptions subsciptions
}

func NewTelegramBotEnvironment(bot *telebot.Bot, chatID int64, mute bool) *TelegramBotEnvironment {
	return &TelegramBotEnvironment{Bot: bot, ChatID: chatID, Mute: mute, subscriptions: subsciptions{subs: make(map[entities.AlertType]string), mu: new(sync.RWMutex)}}
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
	alert.Details = strings.ReplaceAll(alert.Details, entities.HttpScheme+"://", "")
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

func (tgEnv *TelegramBotEnvironment) SendAlertMessage(msg []byte) {
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
		host, err := RemoveSchemePrefix(url)
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

type NodeStatus struct {
	URL     string
	Sumhash string
	Status  string
	Height  string
}

type StatusCondition struct {
	AllNodesAreOk bool
	Nodes         int
	Height        string
}

func SortNodesStatuses(statuses []NodeStatus) {
	sort.Slice(statuses, func(i, j int) bool {
		return strings.Compare(statuses[i].URL, statuses[j].URL) < 0
	})
}

func (tgEnv *TelegramBotEnvironment) NodesStatus(nodesStatusResp *pair.NodesStatusResponse) (string, StatusCondition, error) {
	statusCondition := StatusCondition{AllNodesAreOk: false, Nodes: 0, Height: ""}

	if nodesStatusResp.Err != "" {
		var differentHeightsNodes []NodeStatus
		var unavailableNodes []NodeStatus
		if nodesStatusResp.Err == events.BigHeightDifference.Error() {
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
				tmpl, err := template.ParseFS(templateFiles, "templates/nodes_status_unavailable.html")
				if err != nil {
					log.Printf("failed to construct a message, %v", err)
					return "", statusCondition, err
				}
				wUnavailable := &bytes.Buffer{}
				if err != nil {
					log.Printf("failed to construct a message, %v", err)
					return "", statusCondition, err
				}
				SortNodesStatuses(unavailableNodes)
				err = tmpl.Execute(wUnavailable, unavailableNodes)
				if err != nil {
					log.Printf("failed to construct a message, %v", err)
					return "", statusCondition, err
				}
				msg = fmt.Sprintf(wUnavailable.String() + "\n")
			}
			tmpl, err := template.ParseFS(templateFiles, "templates/nodes_status_different_heights.html")
			if err != nil {
				log.Printf("failed to construct a message, %v", err)
				return "", statusCondition, err
			}
			wDifferentHeights := &bytes.Buffer{}
			if err != nil {
				log.Printf("failed to construct a message, %v", err)
				return "", statusCondition, err
			}
			SortNodesStatuses(differentHeightsNodes)
			err = tmpl.Execute(wDifferentHeights, differentHeightsNodes)
			if err != nil {
				log.Printf("failed to construct a message, %v", err)
				return "", statusCondition, err
			}
			msg += wDifferentHeights.String()
			return fmt.Sprintf("<i>%s</i>\n\n%s", nodesStatusResp.Err, msg), statusCondition, nil
		}
		return nodesStatusResp.Err, statusCondition, nil
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
		tmpl, err := template.ParseFS(templateFiles, "templates/nodes_status_unavailable.html")
		if err != nil {
			log.Printf("failed to construct a message, %v", err)
			return "", statusCondition, err
		}
		wUnavailable := &bytes.Buffer{}
		if err != nil {
			log.Printf("failed to construct a message, %v", err)
			return "", statusCondition, err
		}
		SortNodesStatuses(unavailableNodes)
		err = tmpl.Execute(wUnavailable, unavailableNodes)
		if err != nil {
			log.Printf("failed to construct a message, %v", err)
			return "", statusCondition, err
		}
		msg = fmt.Sprintf(wUnavailable.String() + "\n")
	}
	areHashesEqual := true
	previousHash := okNodes[0].Sumhash
	for _, node := range okNodes {
		if node.Sumhash != previousHash {
			areHashesEqual = false
		}
		previousHash = node.Sumhash
	}

	if !areHashesEqual {
		tmpl, err := template.ParseFS(templateFiles, "templates/nodes_status_different_hashes.html")
		if err != nil {
			log.Printf("failed to construct a message, %v", err)
			return "", statusCondition, err
		}
		wDifferent := &bytes.Buffer{}
		if err != nil {
			log.Printf("failed to construct a message, %v", err)
			return "", statusCondition, err
		}
		SortNodesStatuses(okNodes)
		err = tmpl.Execute(wDifferent, okNodes)
		if err != nil {
			log.Printf("failed to construct a message, %v", err)
			return "", statusCondition, err
		}
		msg += fmt.Sprintf("%s <code>%s</code>", wDifferent.String(), height)
		return msg, statusCondition, nil
	}

	tmpl, err := template.ParseFS(templateFiles, "templates/nodes_status_ok.html")
	if err != nil {
		log.Printf("failed to construct a message, %v", err)
		return "", statusCondition, err
	}
	wOk := &bytes.Buffer{}
	if err != nil {
		log.Printf("failed to construct a message, %v", err)
		return "", statusCondition, err
	}
	SortNodesStatuses(okNodes)
	err = tmpl.Execute(wOk, okNodes)
	if err != nil {
		log.Printf("failed to construct a message, %v", err)
		return "", statusCondition, err
	}

	if len(unavailableNodes) == 0 && len(okNodes) != 0 && areHashesEqual {
		statusCondition.AllNodesAreOk = true
		statusCondition.Nodes = len(okNodes)
		statusCondition.Height = height
	}

	msg += fmt.Sprintf("%s <code>%s</code>", wOk.String(), height)
	return msg, statusCondition, nil
}

func (tgEnv *TelegramBotEnvironment) SubscribeToAllAlerts() error {

	for alertType, alertName := range entities.AlertTypes {
		if tgEnv.IsAlreadySubscribed(alertType) {
			return errors.Errorf("failed to subscribe to %s, already subscribed to it", alertName)
		}
		err := tgEnv.pubSubSocket.SetOption(mangos.OptionSubscribe, []byte{byte(alertType)})
		if err != nil {
			return err
		}
		tgEnv.subscriptions.Add(alertType, alertName)
		log.Printf("Subscribed to %s", alertName)
	}

	return nil
}

func (tgEnv *TelegramBotEnvironment) SubscribeToAlert(alertType entities.AlertType) error {
	alertName, ok := entities.AlertTypes[alertType] // check if such an alert exists
	if !ok {
		return errors.New("failed to subscribe to alert, unkown alert type")
	}

	if tgEnv.IsAlreadySubscribed(alertType) {
		return errors.Errorf("failed to subscribe to %s, already subscribed to it", alertName)
	}

	err := tgEnv.pubSubSocket.SetOption(mangos.OptionSubscribe, []byte{byte(alertType)})
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
		return errors.New("failed to unsubscribe from alert, unkown alert type")
	}

	if !tgEnv.IsAlreadySubscribed(alertType) {
		return errors.Errorf("failed to unsubscribe from %s, was not subscribed to it", alertName)
	}

	err := tgEnv.pubSubSocket.SetOption(mangos.OptionUnsubscribe, []byte{byte(alertType)})
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
	tgEnv.subscriptions.MapR(func() {
		for _, alertName := range tgEnv.subscriptions.subs {
			s := Subscribed{AlertName: alertName + "\n\n"}
			subcribedTo = append(subcribedTo, s)
		}
	})

	var unsusbcribedFrom []Unsubscribed
	for alertType, alertName := range entities.AlertTypes {
		ok := tgEnv.IsAlreadySubscribed(alertType)
		if !ok { // find those alerts that are not in the subscriptions list
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
	_, ok := tgEnv.subscriptions.Read(alertType)
	return ok
}

func RequestNodesList(requestType chan<- pair.RequestPair, responsePairType <-chan pair.ResponsePair, specific bool) ([]string, error) {
	requestType <- &pair.NodeListRequest{Specific: specific}
	responsePair := <-responsePairType
	nodesList, ok := responsePair.(*pair.NodesListResponse)
	if !ok {
		return nil, errors.New("failed to convert response interface to the node list type")
	}
	urls := nodesList.Urls
	sort.Strings(urls)
	return urls, nil
}

func (tgEnv *TelegramBotEnvironment) RequestNodesStatus(
	requestType chan<- pair.RequestPair,
	responsePairType <-chan pair.ResponsePair,
	urls []string) (string, StatusCondition, error) {

	requestType <- &pair.NodesStatusRequest{Urls: urls}
	responsePair := <-responsePairType
	nodesStatus, ok := responsePair.(*pair.NodesStatusResponse)
	if !ok {
		return "", StatusCondition{}, errors.New("failed to convert response interface to the nodes status type")
	}

	return tgEnv.NodesStatus(nodesStatus)

}

func (tgEnv *TelegramBotEnvironment) ScheduleNodesStatus(
	taskScheduler chrono.TaskScheduler,
	requestType chan<- pair.RequestPair,
	responsePairType <-chan pair.ResponsePair) {

	_, err := taskScheduler.ScheduleWithCron(func(ctx context.Context) {
		urls, err := RequestNodesList(requestType, responsePairType, false)
		if err != nil {
			log.Printf("failed to request list of nodes, %v", err)
		}
		additionalUrls, err := RequestNodesList(requestType, responsePairType, true)
		if err != nil {
			log.Printf("failed to request list of additional nodes, %v", err)
		}
		urls = append(urls, additionalUrls...)

		nodesStatus, statusCondition, err := tgEnv.RequestNodesStatus(requestType, responsePairType, urls)
		if err != nil {
			log.Printf("failed to send status of nodes that was scheduled, %v", err)
		}
		if statusCondition.AllNodesAreOk {
			msg := fmt.Sprintf("Status %s\n\nAll <b>%d</b> nodes have the same hashes on height <code>%s</code>", messages.TimerMsg, statusCondition.Nodes, statusCondition.Height)
			tgEnv.SendMessage(msg)
			return
		}
		msg := fmt.Sprintf("Status %s\n\n%s", messages.TimerMsg, nodesStatus)
		tgEnv.SendMessage(msg)
	}, scheduledTimeExpression)

	if err != nil {
		taskScheduler.Shutdown()
		log.Printf("failed to schdule nodes status alert, %v", err)
		return
	}
	log.Println("Nodes status alert has been scheduled successfully")
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
