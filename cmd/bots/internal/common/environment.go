package common

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	htmlTemplate "html/template"
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
	"go.uber.org/zap"
	"gopkg.in/telebot.v3"
	commonMessages "nodemon/cmd/bots/internal/common/messages"
	"nodemon/cmd/bots/internal/common/messaging"
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
	ChatID           string
	Bot              *discordgo.Session
	subSocket        protocol.Socket
	Subscriptions    subscriptions
	zap              *zap.Logger
	requestType      chan<- pair.RequestPair
	responsePairType <-chan pair.ResponsePair
}

func NewDiscordBotEnvironment(bot *discordgo.Session, chatID string, zap *zap.Logger, requestType chan<- pair.RequestPair,
	responsePairType <-chan pair.ResponsePair) *DiscordBotEnvironment {
	return &DiscordBotEnvironment{Bot: bot, ChatID: chatID, Subscriptions: subscriptions{subs: make(map[entities.AlertType]string), mu: new(sync.RWMutex)}, zap: zap, requestType: requestType, responsePairType: responsePairType}
}

func (dscBot *DiscordBotEnvironment) Start() error {
	dscBot.zap.Info("Discord bot started")
	err := dscBot.Bot.Open()
	if err != nil {
		dscBot.zap.Error("failed to open discord bot", zap.Error(err))
		return err
	}
	return nil
}

func (dscBot *DiscordBotEnvironment) SetSubSocket(subSocket protocol.Socket) {
	dscBot.subSocket = subSocket
}

func (dscBot *DiscordBotEnvironment) SendMessage(msg string) {
	_, err := dscBot.Bot.ChannelMessageSend(dscBot.ChatID, msg)
	if err != nil {
		dscBot.zap.Error("failed to send a message to discord", zap.Error(err))
	}
}

func (dscBot *DiscordBotEnvironment) SendAlertMessage(msg []byte) {
	if len(msg) == 0 {
		dscBot.zap.Error("received empty alert message")
		return
	}
	alertType := entities.AlertType(msg[0])
	_, ok := entities.AlertTypes[alertType]
	if !ok {
		dscBot.zap.Sugar().Errorf("failed to construct message, unknown alert type %c, %v", byte(alertType), errUnknownAlertType)
		_, err := dscBot.Bot.ChannelMessageSend(dscBot.ChatID, errUnknownAlertType.Error())

		if err != nil {
			dscBot.zap.Error("failed to send a message to discord", zap.Error(err))
		}
		return
	}

	nodes, err := messaging.RequestAllNodes(dscBot.requestType, dscBot.responsePairType)
	if err != nil {
		dscBot.zap.Error("failed to get nodes list", zap.Error(err))
	}

	messageToBot, err := constructMessage(alertType, msg[1:], Markdown, nodes)
	if err != nil {
		dscBot.zap.Error("failed to construct message", zap.Error(err))
		return
	}
	_, err = dscBot.Bot.ChannelMessageSend(dscBot.ChatID, messageToBot)

	if err != nil {
		dscBot.zap.Error("failed to send a message to discord", zap.Error(err))
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
		dscBot.zap.Sugar().Infof("subscribed to %s", alertName)
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
	ChatID           int64
	Bot              *telebot.Bot
	Mute             bool // If it used elsewhere, should be protected by mutex
	subSocket        protocol.Socket
	subscriptions    subscriptions
	Zap              *zap.Logger
	requestType      chan<- pair.RequestPair
	responsePairType <-chan pair.ResponsePair
}

func NewTelegramBotEnvironment(bot *telebot.Bot, chatID int64, mute bool, zap *zap.Logger, requestType chan<- pair.RequestPair, responsePairType <-chan pair.ResponsePair) *TelegramBotEnvironment {
	return &TelegramBotEnvironment{Bot: bot, ChatID: chatID, Mute: mute, subscriptions: subscriptions{subs: make(map[entities.AlertType]string), mu: new(sync.RWMutex)}, Zap: zap, requestType: requestType, responsePairType: responsePairType}
}

func (tgEnv *TelegramBotEnvironment) Start() error {
	tgEnv.Zap.Info("Telegram bot started")
	tgEnv.Bot.Start()
	tgEnv.Zap.Info("Telegram bot finished")
	return nil
}

func (tgEnv *TelegramBotEnvironment) SetSubSocket(subSocket protocol.Socket) {
	tgEnv.subSocket = subSocket
}

func (tgEnv *TelegramBotEnvironment) SendAlertMessage(msg []byte) {
	if tgEnv.Mute {
		tgEnv.Zap.Info("received an alert, but asleep now")
		return
	}

	if len(msg) == 0 {
		tgEnv.Zap.Info("received an empty message")
		return
	}

	chat := &telebot.Chat{ID: tgEnv.ChatID}

	alertType := entities.AlertType(msg[0])
	_, ok := entities.AlertTypes[alertType]
	if !ok {
		tgEnv.Zap.Sugar().Errorf("failed to construct message, unknown alert type %c, %v", byte(alertType), errUnknownAlertType)
		_, err := tgEnv.Bot.Send(
			chat,
			errUnknownAlertType.Error(),
			&telebot.SendOptions{ParseMode: telebot.ModeHTML},
		)
		if err != nil {
			tgEnv.Zap.Error("failed to send a message to telegram", zap.Error(err))
		}
		return
	}

	nodes, err := messaging.RequestAllNodes(tgEnv.requestType, tgEnv.responsePairType)
	if err != nil {
		tgEnv.Zap.Error("failed to get nodes list", zap.Error(err))
	}

	messageToBot, err := constructMessage(alertType, msg[1:], Html, nodes)
	if err != nil {
		tgEnv.Zap.Error("failed to construct message", zap.Error(err))
		return
	}
	_, err = tgEnv.Bot.Send(
		chat,
		messageToBot,
		&telebot.SendOptions{ParseMode: telebot.ModeHTML},
	)

	if err != nil {
		tgEnv.Zap.Error("failed to send a message to telegram", zap.Error(err))
	}
}

func (tgEnv *TelegramBotEnvironment) SendMessage(msg string) {
	if tgEnv.Mute {
		tgEnv.Zap.Info("received a message, but asleep now")
		return
	}

	chat := &telebot.Chat{ID: tgEnv.ChatID}

	_, err := tgEnv.Bot.Send(
		chat,
		msg,
		&telebot.SendOptions{ParseMode: telebot.ModeHTML},
	)

	if err != nil {
		tgEnv.Zap.Error("failed to send a message to telegram", zap.Error(err))
	}
}

func (tgEnv *TelegramBotEnvironment) IsEligibleForAction(chatID string) bool {
	return chatID == strconv.FormatInt(tgEnv.ChatID, 10)
}

func (tgEnv *TelegramBotEnvironment) NodesListMessage(urls []string) (string, error) {
	tmpl, err := htmlTemplate.ParseFS(templateFiles, "templates/nodes_list.html")

	if err != nil {
		tgEnv.Zap.Error("failed to parse nodes list template", zap.Error(err))
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
		tgEnv.Zap.Error("failed to construct a message", zap.Error(err))
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
		tgEnv.Zap.Sugar().Infof("Telegram bot subscribed to %s", alertName)
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
	tgEnv.Zap.Sugar().Infof("Telegram bot subscribed to %s", alertName)
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
	tgEnv.Zap.Sugar().Infof("Telegram bot unsubscribed from %s", alertName)
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
		tgEnv.Zap.Error("failed to parse subscriptions template", zap.Error(err))
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
		tgEnv.Zap.Error("failed to construct a message", zap.Error(err))
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
	responsePairType <-chan pair.ResponsePair, bot messaging.Bot, zapLogger *zap.Logger) error {

	_, err := taskScheduler.ScheduleWithCron(func(ctx context.Context) {
		nodes, err := messaging.RequestAllNodes(requestType, responsePairType)
		if err != nil {
			zapLogger.Error("failed to get nodes list", zap.Error(err))
		}
		urls := messaging.NodesToUrls(nodes)

		nodesStatus, err := messaging.RequestNodesStatus(requestType, responsePairType, urls)
		if err != nil {
			zapLogger.Error("failed to get nodes status", zap.Error(err))
		}

		var handledNodesStatus string
		statusCondition := StatusCondition{AllNodesAreOk: false, NodesNumber: 0, Height: ""}
		switch bot.(type) {
		case *TelegramBotEnvironment:
			handledNodesStatus, statusCondition, err = HandleNodesStatus(nodesStatus, Html, nodes)
			if err != nil {
				zapLogger.Error("failed to handle nodes status", zap.Error(err))
			}
		case *DiscordBotEnvironment:
			handledNodesStatus, statusCondition, err = HandleNodesStatus(nodesStatus, Markdown, nodes)
			if err != nil {
				zapLogger.Error("failed to handle nodes status", zap.Error(err))
			}
		default:
			zapLogger.Error("failed to schedule nodes status, unknown bot type")
			return
		}

		if statusCondition.AllNodesAreOk {
			var msg string
			shortOkNodes := struct {
				TimeEmoji   string
				NodesNumber int
				Height      string
			}{
				TimeEmoji:   commonMessages.TimerMsg,
				NodesNumber: statusCondition.NodesNumber,
				Height:      statusCondition.Height,
			}
			switch bot.(type) {
			case *TelegramBotEnvironment:
				msg, err = executeTemplate("templates/nodes_status_ok_short", shortOkNodes, Html)
				if err != nil {
					zapLogger.Error("failed to construct a message", zap.Error(err))
				}
			case *DiscordBotEnvironment:
				msg, err = executeTemplate("templates/nodes_status_ok_short", shortOkNodes, Markdown)
				if err != nil {
					zapLogger.Error("failed to construct a message", zap.Error(err))
				}
			default:
				zapLogger.Error("failed to schedule nodes status, unknown bot type")
				return
			}
			bot.SendMessage(msg)
			return
		}
		var msg string
		switch bot.(type) {
		case *TelegramBotEnvironment:
			msg = fmt.Sprintf("Status %s\n\n%s", commonMessages.TimerMsg, handledNodesStatus)
		case *DiscordBotEnvironment:
			msg = fmt.Sprintf("```yaml\nStatus %s\n\n%s\n```", commonMessages.TimerMsg, handledNodesStatus)
		default:
			zapLogger.Error("failed to schedule nodes status, unknown bot type")
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
	BlockID string
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
			return "", err
		}
		buffer := &bytes.Buffer{}
		err = tmpl.Execute(buffer, data)
		if err != nil {
			return "", err
		}
		return buffer.String(), nil
	case Markdown:
		tmpl, err := textTemplate.ParseFS(templateFiles, template+string(extension))
		if err != nil {
			return "", err
		}
		buffer := &bytes.Buffer{}
		err = tmpl.Execute(buffer, data)
		if err != nil {
			return "", err
		}
		return buffer.String(), nil
	default:
		return "", errors.New("unknown message type to execute a template")
	}
}

func replaceNodeWithAlias(node string, nodesAlias map[string]string) string {
	if alias, ok := nodesAlias[node]; ok {
		return alias
	}
	node = strings.ReplaceAll(node, entities.HttpsScheme+"://", "")
	node = strings.ReplaceAll(node, entities.HttpScheme+"://", "")
	return node
}

func executeAlertTemplate(alertType entities.AlertType, alertJson []byte, extension expectedExtension, allNodes []entities.Node) (string, error) {
	nodesAliases := make(map[string]string)
	for _, n := range allNodes {
		if n.Alias != "" {
			nodesAliases[n.URL] = n.Alias
		}
	}
	var msg string
	switch alertType {
	case entities.UnreachableAlertType:
		var unreachableAlert entities.UnreachableAlert
		err := json.Unmarshal(alertJson, &unreachableAlert)
		if err != nil {
			return "", err
		}

		unreachableAlert.Node = replaceNodeWithAlias(unreachableAlert.Node, nodesAliases)

		msg, err = executeTemplate("templates/alerts/unreachable_alert", unreachableAlert, extension)
		if err != nil {
			return "", err
		}
	case entities.IncompleteAlertType:
		var incompleteAlert entities.IncompleteAlert
		err := json.Unmarshal(alertJson, &incompleteAlert)
		if err != nil {
			return "", err
		}
		incompleteStatement := incompleteAlert.NodeStatement
		incompleteStatement.Node = replaceNodeWithAlias(incompleteStatement.Node, nodesAliases)

		msg, err = executeTemplate("templates/alerts/incomplete_alert", incompleteStatement, extension)
		if err != nil {
			return "", err
		}
	case entities.InvalidHeightAlertType:
		var invalidHeightAlert entities.InvalidHeightAlert
		err := json.Unmarshal(alertJson, &invalidHeightAlert)
		if err != nil {
			return "", err
		}
		invalidHeightStatement := invalidHeightAlert.NodeStatement

		invalidHeightStatement.Node = replaceNodeWithAlias(invalidHeightStatement.Node, nodesAliases)

		msg, err = executeTemplate("templates/alerts/invalid_height_alert", invalidHeightStatement, extension)
		if err != nil {
			return "", err
		}
	case entities.HeightAlertType:
		var heightAlert entities.HeightAlert
		err := json.Unmarshal(alertJson, &heightAlert)
		if err != nil {
			return "", err
		}

		for i := range heightAlert.MaxHeightGroup.Nodes {
			heightAlert.MaxHeightGroup.Nodes[i] = replaceNodeWithAlias(heightAlert.MaxHeightGroup.Nodes[i], nodesAliases)
		}
		for i := range heightAlert.OtherHeightGroup.Nodes {
			heightAlert.OtherHeightGroup.Nodes[i] = replaceNodeWithAlias(heightAlert.OtherHeightGroup.Nodes[i], nodesAliases)
		}

		type group struct {
			Nodes  []string
			Height int
		}
		heightStatement := struct {
			HeightDifference int
			FirstGroup       group
			SecondGroup      group
		}{
			HeightDifference: heightAlert.MaxHeightGroup.Height - heightAlert.OtherHeightGroup.Height,
			FirstGroup: group{
				Nodes:  heightAlert.MaxHeightGroup.Nodes,
				Height: heightAlert.MaxHeightGroup.Height,
			},
			SecondGroup: group{
				Nodes:  heightAlert.OtherHeightGroup.Nodes,
				Height: heightAlert.OtherHeightGroup.Height,
			},
		}

		msg, err = executeTemplate("templates/alerts/height_alert", heightStatement, extension)
		if err != nil {
			return "", err
		}
	case entities.StateHashAlertType:
		var stateHashAlert entities.StateHashAlert
		err := json.Unmarshal(alertJson, &stateHashAlert)
		if err != nil {
			return "", err
		}

		for i := range stateHashAlert.FirstGroup.Nodes {
			stateHashAlert.FirstGroup.Nodes[i] = replaceNodeWithAlias(stateHashAlert.FirstGroup.Nodes[i], nodesAliases)
		}
		for i := range stateHashAlert.SecondGroup.Nodes {
			stateHashAlert.SecondGroup.Nodes[i] = replaceNodeWithAlias(stateHashAlert.SecondGroup.Nodes[i], nodesAliases)
		}
		type stateHashGroup struct {
			BlockID   string
			Nodes     []string
			StateHash string
		}
		stateHashStatement := struct {
			SameHeight               int
			LastCommonStateHashExist bool
			ForkHeight               int
			ForkBlockID              string
			ForkStateHash            string

			FirstGroup  stateHashGroup
			SecondGroup stateHashGroup
		}{
			SameHeight:               stateHashAlert.CurrentGroupsBucketHeight,
			LastCommonStateHashExist: stateHashAlert.LastCommonStateHashExist,
			ForkHeight:               stateHashAlert.LastCommonStateHashHeight,
			ForkBlockID:              stateHashAlert.LastCommonStateHash.BlockID.String(),
			ForkStateHash:            stateHashAlert.LastCommonStateHash.SumHash.Hex(),

			FirstGroup: stateHashGroup{
				BlockID:   stateHashAlert.FirstGroup.StateHash.BlockID.String(),
				Nodes:     stateHashAlert.FirstGroup.Nodes,
				StateHash: stateHashAlert.FirstGroup.StateHash.SumHash.Hex(),
			},
			SecondGroup: stateHashGroup{
				BlockID:   stateHashAlert.SecondGroup.StateHash.BlockID.String(),
				Nodes:     stateHashAlert.SecondGroup.Nodes,
				StateHash: stateHashAlert.SecondGroup.StateHash.SumHash.Hex(),
			},
		}

		msg, err = executeTemplate("templates/alerts/state_hash_alert", stateHashStatement, extension)
		if err != nil {
			return "", err
		}
	case entities.AlertFixedType:
		var alertFixed entities.AlertFixed
		err := json.Unmarshal(alertJson, &alertFixed)
		if err != nil {
			return "", err
		}

		// TODO there is no alias here right now, but AlertFixed needs to be changed. Make previous alert to look like a number

		fixedStatement := struct {
			PreviousAlert string
		}{
			PreviousAlert: alertFixed.Fixed.Message(),
		}

		msg, err = executeTemplate("templates/alerts/alert_fixed", fixedStatement, extension)
		if err != nil {
			return "", err
		}
	case entities.BaseTargetAlertType:
		var baseTargetAlert entities.BaseTargetAlert
		err := json.Unmarshal(alertJson, &baseTargetAlert)

		for i := range baseTargetAlert.BaseTargetValues {
			baseTargetAlert.BaseTargetValues[i].Node = replaceNodeWithAlias(baseTargetAlert.BaseTargetValues[i].Node, nodesAliases)
		}

		if err != nil {
			return "", err
		}

		msg, err = executeTemplate("templates/alerts/base_target_alert", baseTargetAlert, extension)
		if err != nil {
			return "", err
		}
	case entities.InternalErrorAlertType:
		var internalErrorAlert entities.InternalErrorAlert
		err := json.Unmarshal(alertJson, &internalErrorAlert)
		if err != nil {
			return "", err
		}
		msg, err = executeTemplate("templates/alerts/internal_error_alert", internalErrorAlert, extension)
		if err != nil {
			return "", err
		}
	default:
		return "", errors.New("unknown alert type")
	}

	return msg, nil

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

	if nodesStatusResp.ErrMessage == events.BigHeightDifference.Error() {
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
				return "", statusCondition, err
			}
			msg = fmt.Sprintf(unavailableNodes + "\n")
		}
		sortNodesStatuses(differentHeightsNodes)
		differentHeights, err := executeTemplate("templates/nodes_status_different_heights", differentHeightsNodes, extension)
		if err != nil {
			return "", statusCondition, err
		}
		msg += fmt.Sprint(differentHeights)
		return fmt.Sprintf("%s\n\n%s", nodesStatusResp.ErrMessage, msg), statusCondition, nil
	}
	return nodesStatusResp.ErrMessage, statusCondition, nil

}

func HandleNodesStatus(nodesStatusResp *pair.NodesStatusResponse,
	extension expectedExtension, allNodes []entities.Node) (string, StatusCondition, error) {
	statusCondition := StatusCondition{AllNodesAreOk: false, NodesNumber: 0, Height: ""}

	nodesAliases := make(map[string]string)
	for _, n := range allNodes {
		if n.Alias != "" {
			nodesAliases[n.URL] = n.Alias
		}
	}
	for i := range nodesStatusResp.NodesStatus {
		nodesStatusResp.NodesStatus[i].Url = replaceNodeWithAlias(nodesStatusResp.NodesStatus[i].Url, nodesAliases)
	}

	if nodesStatusResp.ErrMessage != "" {
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
		s.Sumhash = stat.StateHash.SumHash.Hex()
		s.URL = stat.Url
		s.Status = string(stat.Status)
		s.BlockID = stat.StateHash.BlockID.String()
		okNodes = append(okNodes, s)
	}
	if len(unavailableNodes) != 0 {
		sortNodesStatuses(unavailableNodes)
		unavailableNodes, err := executeTemplate("templates/nodes_status_unavailable", unavailableNodes, extension)
		if err != nil {
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
			return "", statusCondition, err
		}
		msg += fmt.Sprintf("%s %s", differentHashes, height)
		return msg, statusCondition, nil
	}
	equalHashes, err := executeTemplate("templates/nodes_status_ok", okNodes, extension)
	if err != nil {
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

func HandleNodeStatement(nodeStatementResp *pair.NodeStatementResponse, extension expectedExtension) (string, error) {
	nodeStatementResp.NodeStatement.Node = strings.ReplaceAll(nodeStatementResp.NodeStatement.Node, entities.HttpsScheme+"://", "")
	nodeStatementResp.NodeStatement.Node = strings.ReplaceAll(nodeStatementResp.NodeStatement.Node, entities.HttpScheme+"://", "")

	if nodeStatementResp.ErrMessage != "" {
		return nodeStatementResp.ErrMessage, nil
	}

	nodeStatement := struct {
		Node      string
		Height    int
		Timestamp int64
		StateHash string
		Version   string
	}{Node: nodeStatementResp.NodeStatement.Node,
		Height:    nodeStatementResp.NodeStatement.Height,
		Timestamp: nodeStatementResp.NodeStatement.Timestamp,
		StateHash: nodeStatementResp.NodeStatement.StateHash.SumHash.Hex(),
		Version:   nodeStatementResp.NodeStatement.Version,
	}

	msg, err := executeTemplate("templates/node_statement", nodeStatement, extension)
	if err != nil {
		return "", err
	}

	return msg, nil
}

func constructMessage(alertType entities.AlertType, alertJson []byte, extension expectedExtension, allNodes []entities.Node) (string, error) {
	alert := generalMessaging.Alert{}
	err := json.Unmarshal(alertJson, &alert)
	if err != nil {
		return "", errors.Wrap(err, "failed to unmarshal json")
	}

	msg, err := executeAlertTemplate(alertType, alertJson, extension, allNodes)
	if err != nil {
		return "", errors.Errorf("failed to execute an alert template, %v", err)
	}
	return msg, nil
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
