package common

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strconv"
	"strings"
	"sync"

	"nodemon/cmd/bots/internal/common/messaging"
	"nodemon/pkg/entities"
	generalMessaging "nodemon/pkg/messaging"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/storing/events"

	"codnect.io/chrono"
	"github.com/bwmarrin/discordgo"
	"github.com/nats-io/nats.go"
	"github.com/pkg/errors"
	"github.com/wavesplatform/gowaves/pkg/crypto"
	"go.uber.org/zap"
	"gopkg.in/telebot.v3"
)

const (
	scheduledTimeExpression = "0 0 9 * * *" // 12:00 UTC+3
)

var (
	//go:embed templates
	templateFiles embed.FS
)

type ExpectedExtension string

const (
	HTML     ExpectedExtension = ".html"
	Markdown ExpectedExtension = ".md"
)

var errUnknownAlertType = errors.New("received unknown alert type")

type AlertSubscription struct {
	alertName    entities.AlertName
	subscription *nats.Subscription
}

type subscriptions struct {
	mu   *sync.RWMutex
	subs map[entities.AlertType]AlertSubscription
}

func (s *subscriptions) Add(alertType entities.AlertType, alertName entities.AlertName,
	subscription *nats.Subscription) {
	s.mu.Lock()
	s.subs[alertType] = AlertSubscription{alertName, subscription}
	s.mu.Unlock()
}

// Read returns alert name.
func (s *subscriptions) Read(alertType entities.AlertType) (AlertSubscription, bool) {
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
	ChatID                 string
	Bot                    *discordgo.Session
	Subscriptions          subscriptions
	zap                    *zap.Logger
	requestType            chan<- pair.Request
	responsePairType       <-chan pair.Response
	unhandledAlertMessages unhandledAlertMessages
	scheme                 string
	nc                     *nats.Conn
	alertHandlerFunc       func(msg *nats.Msg)
}

func NewDiscordBotEnvironment(
	bot *discordgo.Session,
	chatID string,
	zap *zap.Logger,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
	scheme string,
) *DiscordBotEnvironment {
	return &DiscordBotEnvironment{
		Bot:    bot,
		ChatID: chatID,
		Subscriptions: subscriptions{
			subs: make(map[entities.AlertType]AlertSubscription),
			mu:   new(sync.RWMutex),
		},
		zap:                    zap,
		requestType:            requestType,
		responsePairType:       responsePairType,
		unhandledAlertMessages: newUnhandledAlertMessages(),
		scheme:                 scheme,
	}
}

func (dscBot *DiscordBotEnvironment) TemplatesExtension() ExpectedExtension { return Markdown }

func (dscBot *DiscordBotEnvironment) Start() error {
	dscBot.zap.Info("Discord bot started")
	err := dscBot.Bot.Open()
	if err != nil {
		dscBot.zap.Error("failed to open discord bot", zap.Error(err))
		return err
	}
	return nil
}

func (dscBot *DiscordBotEnvironment) SendMessage(msg string) {
	_, err := dscBot.Bot.ChannelMessageSend(dscBot.ChatID, msg)
	if err != nil {
		dscBot.zap.Error("failed to send a message to discord", zap.Error(err))
	}
}

func (dscBot *DiscordBotEnvironment) SendAlertMessage(msg generalMessaging.AlertMessage) {
	alertType := msg.AlertType()
	if !alertType.Exist() {
		dscBot.zap.Sugar().Errorf("failed to construct message, unknown alert type %c, %v",
			byte(alertType), errUnknownAlertType,
		)
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

	alertJSON := msg.Data()
	messageToBot, err := constructMessage(alertType, alertJSON, dscBot.TemplatesExtension(), nodes)
	if err != nil {
		dscBot.zap.Error("failed to construct message", zap.Error(err))
		return
	}
	alertID := msg.ReferenceID()

	if alertType == entities.AlertFixedType {
		messageID, ok := dscBot.unhandledAlertMessages.GetMessageIDByAlertID(alertID)
		if !ok {
			dscBot.zap.Error("failed to get message ID by the given alertID: alertID hasn't been found",
				zap.Stringer("alertID", alertID),
			)
			return
		}
		msgRef := &discordgo.MessageReference{MessageID: strconv.Itoa(messageID)}
		_, err = dscBot.Bot.ChannelMessageSendReply(dscBot.ChatID, messageToBot, msgRef)
		if err != nil {
			dscBot.zap.Error("failed to send a message about fixed alert to discord", zap.Error(err))
		}
		dscBot.unhandledAlertMessages.Delete(alertID)
		return
	}
	sentMessage, err := dscBot.Bot.ChannelMessageSend(dscBot.ChatID, messageToBot)
	if err != nil {
		dscBot.zap.Error("failed to send a message to discord", zap.Error(err))
		return
	}

	messageID, err := strconv.Atoi(sentMessage.ID)
	if err != nil {
		dscBot.zap.Error("failed to parse messageID from a send message on discord", zap.Error(err))
		return
	}
	dscBot.unhandledAlertMessages.Add(alertID, messageID)
}

func (dscBot *DiscordBotEnvironment) SetNatsConnection(nc *nats.Conn) {
	dscBot.nc = nc
}

func (dscBot *DiscordBotEnvironment) SetAlertHandlerFunc(alertHandlerFunc func(msg *nats.Msg)) {
	dscBot.alertHandlerFunc = alertHandlerFunc
}

func (dscBot *DiscordBotEnvironment) SubscribeToAllAlerts() error {
	for alertType, alertName := range entities.GetAllAlertTypesAndNames() {
		if dscBot.IsAlreadySubscribed(alertType) {
			return errors.Errorf("failed to subscribe to %s, already subscribed to it", alertName)
		}
		topic := generalMessaging.PubSubMsgTopic(dscBot.scheme, alertType)
		subscription, err := dscBot.nc.Subscribe(topic, dscBot.alertHandlerFunc)
		if err != nil {
			return errors.Wrap(err, "failed to subscribe to alert")
		}
		dscBot.Subscriptions.Add(alertType, alertName, subscription)
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

type unhandledAlertMessages struct {
	mu            *sync.RWMutex
	alertMessages map[crypto.Digest]int // map[AlertID]MessageID
}

func newUnhandledAlertMessages() unhandledAlertMessages {
	return unhandledAlertMessages{mu: new(sync.RWMutex), alertMessages: make(map[crypto.Digest]int)}
}

func (m unhandledAlertMessages) Add(alertID crypto.Digest, messageID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertMessages[alertID] = messageID
}

func (m unhandledAlertMessages) GetMessageIDByAlertID(alertID crypto.Digest) (int, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	messageID, ok := m.alertMessages[alertID]
	return messageID, ok
}

func (m unhandledAlertMessages) Delete(alertID crypto.Digest) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.alertMessages, alertID)
}

type TelegramBotEnvironment struct {
	ChatID                 int64
	Bot                    *telebot.Bot
	Mute                   bool // If it used elsewhere, should be protected by mutex.
	subscriptions          subscriptions
	zap                    *zap.Logger
	requestType            chan<- pair.Request
	responsePairType       <-chan pair.Response
	unhandledAlertMessages unhandledAlertMessages
	scheme                 string
	nc                     *nats.Conn
	alertHandlerFunc       func(msg *nats.Msg)
}

func NewTelegramBotEnvironment(
	bot *telebot.Bot,
	chatID int64,
	mute bool,
	zap *zap.Logger,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
	scheme string,
) *TelegramBotEnvironment {
	return &TelegramBotEnvironment{
		Bot:    bot,
		ChatID: chatID,
		Mute:   mute,
		subscriptions: subscriptions{
			subs: make(map[entities.AlertType]AlertSubscription),
			mu:   new(sync.RWMutex),
		},
		zap:                    zap,
		requestType:            requestType,
		responsePairType:       responsePairType,
		unhandledAlertMessages: newUnhandledAlertMessages(),
		scheme:                 scheme,
	}
}

func (tgEnv *TelegramBotEnvironment) TemplatesExtension() ExpectedExtension { return HTML }

func (tgEnv *TelegramBotEnvironment) Start(ctx context.Context) error {
	tgEnv.zap.Info("Telegram bot started")
	go func() {
		<-ctx.Done()
		tgEnv.Bot.Stop()
	}()
	tgEnv.Bot.Start() // the call is blocking
	tgEnv.zap.Info("Telegram bot finished")
	return nil
}

func (tgEnv *TelegramBotEnvironment) SendAlertMessage(msg generalMessaging.AlertMessage) {
	if tgEnv.Mute {
		tgEnv.zap.Info("received an alert, but asleep now")
		return
	}

	chat := &telebot.Chat{ID: tgEnv.ChatID}

	alertType := msg.AlertType()
	if !alertType.Exist() {
		tgEnv.zap.Sugar().Errorf("failed to construct message, unknown alert type %c, %v",
			byte(alertType), errUnknownAlertType,
		)
		_, err := tgEnv.Bot.Send(
			chat,
			errUnknownAlertType.Error(),
			&telebot.SendOptions{ParseMode: telebot.ModeHTML},
		)
		if err != nil {
			tgEnv.zap.Error("failed to send a message to telegram", zap.Error(err))
		}
		return
	}

	nodes, err := messaging.RequestAllNodes(tgEnv.requestType, tgEnv.responsePairType)
	if err != nil {
		tgEnv.zap.Error("failed to get nodes list", zap.Error(err))
	}

	alertJSON := msg.Data()
	messageToBot, err := constructMessage(alertType, alertJSON, tgEnv.TemplatesExtension(), nodes)
	if err != nil {
		tgEnv.zap.Error("failed to construct message", zap.Error(err))
		return
	}
	alertID := msg.ReferenceID()

	if alertType == entities.AlertFixedType {
		messageID, ok := tgEnv.unhandledAlertMessages.GetMessageIDByAlertID(alertID)
		if !ok {
			tgEnv.zap.Error("failed to get message ID by the given alertID: alertID hasn't been found",
				zap.Stringer("alertID", alertID),
			)
			return
		}
		opts := &telebot.SendOptions{ReplyTo: &telebot.Message{ID: messageID}, ParseMode: telebot.ModeHTML}
		_, sendErr := tgEnv.Bot.Send(chat, messageToBot, opts)
		if sendErr != nil {
			tgEnv.zap.Error("failed to send a message about fixed alert to telegram", zap.Error(sendErr))
		}
		tgEnv.unhandledAlertMessages.Delete(alertID)
		return
	}

	sentMessage, err := tgEnv.Bot.Send(
		chat,
		messageToBot,
		&telebot.SendOptions{ParseMode: telebot.ModeHTML},
	)
	if err != nil {
		tgEnv.zap.Error("failed to send a message to telegram", zap.Error(err))
	}

	tgEnv.unhandledAlertMessages.Add(alertID, sentMessage.ID)
}

func (tgEnv *TelegramBotEnvironment) SendMessage(msg string) {
	if tgEnv.Mute {
		tgEnv.zap.Info("received a message, but asleep now")
		return
	}

	chat := &telebot.Chat{ID: tgEnv.ChatID}

	_, err := tgEnv.Bot.Send(
		chat,
		msg,
		&telebot.SendOptions{ParseMode: telebot.ModeHTML},
	)

	if err != nil {
		tgEnv.zap.Error("failed to send a message to telegram", zap.Error(err))
	}
}

func (tgEnv *TelegramBotEnvironment) IsEligibleForAction(chatID string) bool {
	return chatID == strconv.FormatInt(tgEnv.ChatID, 10)
}

func removeHTTPOrHTTPSScheme(s string) string {
	s = strings.ReplaceAll(s, entities.HTTPSScheme+"://", "")
	return strings.ReplaceAll(s, entities.HTTPScheme+"://", "")
}

func nodesToUrls(nodes []entities.Node) []string {
	urls := make([]string, 0, len(nodes))
	for _, n := range nodes {
		var host string
		if n.Alias != "" {
			host = n.Alias
		} else {
			host = removeHTTPOrHTTPSScheme(n.URL)
		}
		urls = append(urls, host)
	}
	return urls
}

func (tgEnv *TelegramBotEnvironment) NodesListMessage(nodes []entities.Node) (string, error) {
	urls := nodesToUrls(nodes)
	tmpl, err := template.ParseFS(templateFiles, "templates/nodes_list.html")
	if err != nil {
		tgEnv.zap.Error("failed to parse nodes list template", zap.Error(err))
		return "", err
	}

	sort.Strings(urls)

	w := &bytes.Buffer{}
	err = tmpl.Execute(w, urls)
	if err != nil {
		tgEnv.zap.Error("failed to construct a message", zap.Error(err))
		return "", err
	}
	return w.String(), nil
}

func (tgEnv *TelegramBotEnvironment) SubscribeToAllAlerts() error {
	for alertType, alertName := range entities.GetAllAlertTypesAndNames() {
		if tgEnv.IsAlreadySubscribed(alertType) {
			return errors.Errorf("failed to subscribe to %s, already subscribed to it", alertName)
		}
		topic := generalMessaging.PubSubMsgTopic(tgEnv.scheme, alertType)
		subscription, err := tgEnv.nc.Subscribe(topic, tgEnv.alertHandlerFunc)
		if err != nil {
			return errors.Wrap(err, "failed to subscribe to alert")
		}
		tgEnv.subscriptions.Add(alertType, alertName, subscription)
		tgEnv.zap.Sugar().Infof("Telegram bot subscribed to %s", alertName)
	}

	return nil
}

func (tgEnv *TelegramBotEnvironment) SubscribeToAlert(alertType entities.AlertType) error {
	alertName, ok := alertType.AlertName() // check if such an alert exists
	if !ok {
		return errors.New("failed to subscribe to alert, unknown alert type")
	}

	if tgEnv.IsAlreadySubscribed(alertType) {
		return errors.Errorf("failed to subscribe to %s, already subscribed to it", alertName)
	}

	topic := generalMessaging.PubSubMsgTopic(tgEnv.scheme, alertType)
	subscription, err := tgEnv.nc.Subscribe(topic, tgEnv.alertHandlerFunc)
	if err != nil {
		return errors.Wrap(err, "failed to subscribe to alert")
	}
	tgEnv.subscriptions.Add(alertType, alertName, subscription)
	tgEnv.zap.Sugar().Infof("Telegram bot subscribed to %s", alertName)

	return nil
}

func (tgEnv *TelegramBotEnvironment) UnsubscribeFromAlert(alertType entities.AlertType) error {
	alertName, ok := alertType.AlertName() // check if such an alert exists
	if !ok {
		return errors.New("failed to unsubscribe from alert, unknown alert type")
	}

	if !tgEnv.IsAlreadySubscribed(alertType) {
		return errors.Errorf("failed to unsubscribe from %s, was not subscribed to it", alertName)
	}
	alertSub, ok := tgEnv.subscriptions.Read(alertType)
	if !ok {
		return errors.Errorf("subscription didn't exist even though I was subscribed to it")
	}
	err := alertSub.subscription.Unsubscribe()
	if err != nil {
		return errors.New("failed to unsubscribe from alert")
	}
	ok = tgEnv.IsAlreadySubscribed(alertType)
	if !ok {
		return errors.New("tried to unsubscribe from alert, but still subscribed to it")
	}
	tgEnv.subscriptions.Delete(alertType)
	tgEnv.zap.Sugar().Infof("Telegram bot unsubscribed from %s", alertName)
	return nil
}

func (tgEnv *TelegramBotEnvironment) SetNatsConnection(nc *nats.Conn) {
	tgEnv.nc = nc
}

func (tgEnv *TelegramBotEnvironment) SetAlertHandlerFunc(alertHandlerFunc func(msg *nats.Msg)) {
	tgEnv.alertHandlerFunc = alertHandlerFunc
}

type subscribed struct {
	AlertName string
}

type unsubscribed struct {
	AlertName string
}

type subscriptionsList struct {
	SubscribedTo     []subscribed
	UnsubscribedFrom []unsubscribed
}

func (tgEnv *TelegramBotEnvironment) SubscriptionsList() (string, error) {
	tmpl, err := template.ParseFS(templateFiles, "templates/subscriptions.html")

	if err != nil {
		tgEnv.zap.Error("failed to parse subscriptions template", zap.Error(err))
		return "", err
	}
	var subscribedTo []subscribed
	tgEnv.subscriptions.MapR(func() {
		for _, alertName := range tgEnv.subscriptions.subs {
			s := subscribed{AlertName: string(alertName.alertName) + "\n\n"}
			subscribedTo = append(subscribedTo, s)
		}
	})

	var unsubscribedFrom []unsubscribed
	for alertType, alertName := range entities.GetAllAlertTypesAndNames() {
		ok := tgEnv.IsAlreadySubscribed(alertType)
		if !ok { // find those alerts that are not in the subscriptions list
			u := unsubscribed{AlertName: string(alertName) + "\n\n"}
			unsubscribedFrom = append(unsubscribedFrom, u)
		}
	}

	subsList := subscriptionsList{SubscribedTo: subscribedTo, UnsubscribedFrom: unsubscribedFrom}

	w := &bytes.Buffer{}
	err = tmpl.Execute(w, subsList)
	if err != nil {
		tgEnv.zap.Error("failed to construct a message", zap.Error(err))
		return "", err
	}
	return w.String(), nil
}

func (tgEnv *TelegramBotEnvironment) IsAlreadySubscribed(alertType entities.AlertType) bool {
	_, ok := tgEnv.subscriptions.Read(alertType)
	return ok
}

type shortOkNodes struct {
	TimeEmoji   string
	NodesNumber int
	Height      string
}

type bot interface {
	messaging.Bot
	TemplatesExtension() ExpectedExtension
}

func ScheduleNodesStatus(
	taskScheduler chrono.TaskScheduler,
	requestType chan<- pair.Request,
	responsePairType <-chan pair.Response,
	bot bot,
	zapLogger *zap.Logger,
) error {
	_, err := taskScheduler.ScheduleWithCron(func(_ context.Context) {
		nodes, err := messaging.RequestAllNodes(requestType, responsePairType)
		if err != nil {
			zapLogger.Error("failed to get nodes list", zap.Error(err))
		}
		urls := messaging.NodesToUrls(nodes)

		nodesStatus, err := messaging.RequestNodesStatements(requestType, responsePairType, urls)
		if err != nil {
			zapLogger.Error("failed to get nodes status", zap.Error(err))
		}
		handledNodesStatus, statusCondition, err := HandleNodesStatus(nodesStatus, bot.TemplatesExtension(), nodes)
		if err != nil {
			zapLogger.Error("failed to handle nodes status", zap.Error(err))
		}

		if statusCondition.AllNodesAreOk {
			var msg string
			okNodes := shortOkNodes{
				TimeEmoji:   messaging.TimerMsg,
				NodesNumber: statusCondition.NodesNumber,
				Height:      statusCondition.Height,
			}
			msg, err = executeTemplate("templates/nodes_status_ok_short", okNodes, bot.TemplatesExtension())
			if err != nil {
				zapLogger.Error("failed to construct a message", zap.Error(err))
			}
			bot.SendMessage(msg)
			return
		}
		var msg string
		switch bot.(type) {
		case *TelegramBotEnvironment:
			msg = fmt.Sprintf("Status %s\n\n%s", messaging.TimerMsg, handledNodesStatus)
		case *DiscordBotEnvironment:
			msg = fmt.Sprintf("```yaml\nStatus %s\n\n%s\n```", messaging.TimerMsg, handledNodesStatus)
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
	SumHash string
	Status  string
	Height  string
	BlockID string
}

func sortNodesStatuses(statuses []NodeStatus) {
	sort.Slice(statuses, func(i, j int) bool {
		return strings.Compare(statuses[i].URL, statuses[j].URL) < 0
	})
}

func executeTemplate(templateName string, data any, extension ExpectedExtension) (string, error) {
	switch extension {
	case HTML, Markdown:
		tmpl, err := template.ParseFS(templateFiles, templateName+string(extension))
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
	return removeHTTPOrHTTPSScheme(node)
}

func GetNodeURLByAlias(alias string, nodes []entities.Node) string {
	for _, n := range nodes {
		if n.Alias == alias {
			return n.URL
		}
	}
	return alias
}

type heightStatementGroup struct {
	Nodes  []string
	Height uint64
}

type heightStatement struct {
	HeightDifference uint64
	FirstGroup       heightStatementGroup
	SecondGroup      heightStatementGroup
}

type stateHashStatementGroup struct {
	BlockID   string
	Nodes     []string
	StateHash string
}

type stateHashStatement struct {
	SameHeight               uint64
	LastCommonStateHashExist bool
	ForkHeight               uint64
	ForkBlockID              string
	ForkStateHash            string
	FirstGroup               stateHashStatementGroup
	SecondGroup              stateHashStatementGroup
}

type fixedStatement struct {
	PreviousAlert string
}

func executeAlertTemplate(
	alertType entities.AlertType,
	alertJSON []byte,
	extension ExpectedExtension,
	allNodes []entities.Node,
) (string, error) {
	nodesAliases := make(map[string]string)
	for _, n := range allNodes {
		if n.Alias != "" {
			nodesAliases[n.URL] = n.Alias
		}
	}
	var (
		msg string
		err error
	)
	switch alertType {
	case entities.SimpleAlertType:
		msg, err = executeSimpleAlertTemplate(alertJSON, extension)
	case entities.UnreachableAlertType:
		msg, err = executeUnreachableTemplate(alertJSON, nodesAliases, extension)
	case entities.IncompleteAlertType:
		msg, err = executeIncompleteTemplate(alertJSON, nodesAliases, extension)
	case entities.InvalidHeightAlertType:
		msg, err = executeInvalidHeightTemplate(alertJSON, nodesAliases, extension)
	case entities.HeightAlertType:
		msg, err = executeHeightTemplate(alertJSON, nodesAliases, extension)
	case entities.StateHashAlertType:
		msg, err = executeStateHashTemplate(alertJSON, nodesAliases, extension)
	case entities.AlertFixedType:
		msg, err = executeAlertFixed(alertJSON, extension)
	case entities.BaseTargetAlertType:
		msg, err = executeBaseTargetTemplate(alertJSON, nodesAliases, extension)
	case entities.InternalErrorAlertType:
		msg, err = executeInternalErrorTemplate(alertJSON, extension)
	case entities.ChallengedBlockAlertType:
		msg, err = executeChallengedBlockTemplate(alertJSON, extension)
	case entities.L2StuckAlertType:
		msg, err = executeL2StuckAlertTemplate(alertJSON, extension)
	default:
		return "", errors.Errorf("unknown alert type (%d)", alertType)
	}
	if err != nil {
		return "", err
	}

	return msg, nil
}

func executeSimpleAlertTemplate(alertJSON []byte, extension ExpectedExtension) (string, error) {
	var simpleAlert entities.SimpleAlert
	err := json.Unmarshal(alertJSON, &simpleAlert)
	if err != nil {
		return "", err
	}
	msg, err := executeTemplate("templates/alerts/simple_alert", simpleAlert, extension)
	if err != nil {
		return "", err
	}
	return msg, nil
}

func executeChallengedBlockTemplate(alertJSON []byte, extension ExpectedExtension) (string, error) {
	var challengedBlockAlert entities.ChallengedBlockAlert
	err := json.Unmarshal(alertJSON, &challengedBlockAlert)
	if err != nil {
		return "", err
	}
	_ = fmt.Stringer(challengedBlockAlert.BlockID) // to check if it implements Stringer
	msg, err := executeTemplate("templates/alerts/challenged_block_alert", challengedBlockAlert, extension)
	if err != nil {
		return "", err
	}
	return msg, nil
}

func executeInternalErrorTemplate(alertJSON []byte, extension ExpectedExtension) (string, error) {
	var internalErrorAlert entities.InternalErrorAlert
	err := json.Unmarshal(alertJSON, &internalErrorAlert)
	if err != nil {
		return "", err
	}
	msg, err := executeTemplate("templates/alerts/internal_error_alert", internalErrorAlert, extension)
	if err != nil {
		return "", err
	}
	return msg, nil
}

func executeBaseTargetTemplate(
	alertJSON []byte,
	nodesAliases map[string]string,
	extension ExpectedExtension,
) (string, error) {
	var btAlert entities.BaseTargetAlert
	err := json.Unmarshal(alertJSON, &btAlert)
	if err != nil {
		return "", err
	}

	for i := range btAlert.BaseTargetValues {
		btAlert.BaseTargetValues[i].Node = replaceNodeWithAlias(btAlert.BaseTargetValues[i].Node, nodesAliases)
	}

	msg, err := executeTemplate("templates/alerts/base_target_alert", btAlert, extension)
	if err != nil {
		return "", err
	}
	return msg, nil
}

func executeAlertFixed(alertJSON []byte, extension ExpectedExtension) (string, error) {
	var alertFixed entities.AlertFixed
	err := json.Unmarshal(alertJSON, &alertFixed)
	if err != nil {
		return "", err
	}

	statement := fixedStatement{
		PreviousAlert: alertFixed.Fixed.Name().String(),
	}
	msg, err := executeTemplate("templates/alerts/alert_fixed", statement, extension)
	if err != nil {
		return "", err
	}
	return msg, nil
}

func executeStateHashTemplate(
	alertJSON []byte,
	nodesAliases map[string]string,
	extension ExpectedExtension,
) (string, error) {
	var stateHashAlert entities.StateHashAlert
	err := json.Unmarshal(alertJSON, &stateHashAlert)
	if err != nil {
		return "", err
	}

	for i := range stateHashAlert.FirstGroup.Nodes {
		stateHashAlert.FirstGroup.Nodes[i] = replaceNodeWithAlias(stateHashAlert.FirstGroup.Nodes[i], nodesAliases)
	}
	for i := range stateHashAlert.SecondGroup.Nodes {
		stateHashAlert.SecondGroup.Nodes[i] = replaceNodeWithAlias(stateHashAlert.SecondGroup.Nodes[i], nodesAliases)
	}

	statement := stateHashStatement{
		SameHeight:               stateHashAlert.CurrentGroupsBucketHeight,
		LastCommonStateHashExist: stateHashAlert.LastCommonStateHashExist,
		ForkHeight:               stateHashAlert.LastCommonStateHashHeight,
		ForkBlockID:              stateHashAlert.LastCommonStateHash.BlockID.String(),
		ForkStateHash:            stateHashAlert.LastCommonStateHash.SumHash.Hex(),

		FirstGroup: stateHashStatementGroup{
			BlockID:   stateHashAlert.FirstGroup.StateHash.BlockID.String(),
			Nodes:     stateHashAlert.FirstGroup.Nodes,
			StateHash: stateHashAlert.FirstGroup.StateHash.SumHash.Hex(),
		},
		SecondGroup: stateHashStatementGroup{
			BlockID:   stateHashAlert.SecondGroup.StateHash.BlockID.String(),
			Nodes:     stateHashAlert.SecondGroup.Nodes,
			StateHash: stateHashAlert.SecondGroup.StateHash.SumHash.Hex(),
		},
	}

	if statement.FirstGroup.BlockID != statement.SecondGroup.BlockID {
		msg, tmplErr := executeTemplate("templates/alerts/state_hash_several_chains_alert", statement, extension)
		if tmplErr != nil {
			return "", tmplErr
		}
		return msg, nil
	}

	msg, tmplErr := executeTemplate("templates/alerts/state_hash_alert", statement, extension)
	if tmplErr != nil {
		return "", tmplErr
	}
	return msg, nil
}

func executeHeightTemplate(alertJSON []byte, nodesAliases map[string]string, ext ExpectedExtension) (string, error) {
	var heightAlert entities.HeightAlert
	err := json.Unmarshal(alertJSON, &heightAlert)
	if err != nil {
		return "", err
	}

	for i := range heightAlert.MaxHeightGroup.Nodes {
		heightAlert.MaxHeightGroup.Nodes[i] = replaceNodeWithAlias(heightAlert.MaxHeightGroup.Nodes[i], nodesAliases)
	}
	for i := range heightAlert.OtherHeightGroup.Nodes {
		heightAlert.OtherHeightGroup.Nodes[i] = replaceNodeWithAlias(heightAlert.OtherHeightGroup.Nodes[i], nodesAliases)
	}

	statement := heightStatement{
		HeightDifference: heightAlert.MaxHeightGroup.Height - heightAlert.OtherHeightGroup.Height,
		FirstGroup: heightStatementGroup{
			Nodes:  heightAlert.MaxHeightGroup.Nodes,
			Height: heightAlert.MaxHeightGroup.Height,
		},
		SecondGroup: heightStatementGroup{
			Nodes:  heightAlert.OtherHeightGroup.Nodes,
			Height: heightAlert.OtherHeightGroup.Height,
		},
	}

	msg, err := executeTemplate("templates/alerts/height_alert", statement, ext)
	if err != nil {
		return "", err
	}
	return msg, nil
}

func executeInvalidHeightTemplate(
	alertJSON []byte,
	nodesAliases map[string]string,
	extension ExpectedExtension,
) (string, error) {
	var invalidHeightAlert entities.InvalidHeightAlert
	err := json.Unmarshal(alertJSON, &invalidHeightAlert)
	if err != nil {
		return "", err
	}
	statement := invalidHeightAlert.NodeStatement

	statement.Node = replaceNodeWithAlias(statement.Node, nodesAliases)

	msg, err := executeTemplate("templates/alerts/invalid_height_alert", statement, extension)
	if err != nil {
		return "", err
	}
	return msg, nil
}

func executeIncompleteTemplate(
	alertJSON []byte,
	nodesAliases map[string]string,
	extension ExpectedExtension,
) (string, error) {
	var incompleteAlert entities.IncompleteAlert
	err := json.Unmarshal(alertJSON, &incompleteAlert)
	if err != nil {
		return "", err
	}
	statement := incompleteAlert.NodeStatement
	statement.Node = replaceNodeWithAlias(statement.Node, nodesAliases)

	msg, err := executeTemplate("templates/alerts/incomplete_alert", statement, extension)
	if err != nil {
		return "", err
	}
	return msg, nil
}

func executeUnreachableTemplate(
	alertJSON []byte,
	nodesAliases map[string]string,
	extension ExpectedExtension,
) (string, error) {
	var unreachableAlert entities.UnreachableAlert
	err := json.Unmarshal(alertJSON, &unreachableAlert)
	if err != nil {
		return "", err
	}

	unreachableAlert.Node = replaceNodeWithAlias(unreachableAlert.Node, nodesAliases)

	msg, err := executeTemplate("templates/alerts/unreachable_alert", unreachableAlert, extension)
	if err != nil {
		return "", err
	}
	return msg, nil
}

func executeL2StuckAlertTemplate(alertJSON []byte, extension ExpectedExtension) (string, error) {
	var l2StuckAlert entities.L2StuckAlert
	err := json.Unmarshal(alertJSON, &l2StuckAlert)
	if err != nil {
		return "", err
	}
	msg, err := executeTemplate("templates/alerts/l2/l2_stuck_alert", l2StuckAlert, extension)
	if err != nil {
		return "", err
	}
	return msg, nil
}

type StatusCondition struct {
	AllNodesAreOk bool
	NodesNumber   int
	Height        string
}

func HandleNodesStatusError(resp *pair.NodesStatementsResponse,
	ext ExpectedExtension) (string, StatusCondition, error) {
	statusCondition := StatusCondition{AllNodesAreOk: false, NodesNumber: 0, Height: ""}

	var differentHeightsNodes []NodeStatus
	var unavailableNodes []NodeStatus

	if resp.ErrMessage != events.ErrBigHeightDifference.Error() {
		return resp.ErrMessage, statusCondition, nil
	}

	for _, stat := range resp.NodesStatements {
		if stat.Status != entities.OK {
			unavailableNodes = append(unavailableNodes, NodeStatus{URL: stat.URL})
			continue
		}
		height := strconv.FormatUint(stat.Height, 10)
		s := NodeStatus{
			URL:    stat.URL,
			Height: height,
		}
		differentHeightsNodes = append(differentHeightsNodes, s)
	}
	var msg string
	if len(unavailableNodes) != 0 {
		sortNodesStatuses(unavailableNodes)
		unavailableMsg, err := executeTemplate("templates/nodes_status_unavailable", unavailableNodes, ext)
		if err != nil {
			return "", statusCondition, err
		}
		msg = unavailableMsg + "\n"
	}
	sortNodesStatuses(differentHeightsNodes)
	differentHeights, err := executeTemplate("templates/nodes_status_different_heights", differentHeightsNodes, ext)
	if err != nil {
		return "", statusCondition, err
	}
	msg += differentHeights
	return fmt.Sprintf("%s\n\n%s", resp.ErrMessage, msg), statusCondition, nil
}

func HandleNodesStatus(
	resp *pair.NodesStatementsResponse,
	ext ExpectedExtension,
	nodes []entities.Node,
) (string, StatusCondition, error) {
	statusCondition := StatusCondition{AllNodesAreOk: false, NodesNumber: 0, Height: ""}

	nodesAliases := nodeURLToAlias(nodes)
	for i := range resp.NodesStatements {
		resp.NodesStatements[i].URL = replaceNodeWithAlias(resp.NodesStatements[i].URL, nodesAliases)
	}

	if resp.ErrMessage != "" {
		return HandleNodesStatusError(resp, ext)
	}

	var msg string
	unavailableNodes, okNodes, height := sortedNodesByStatusWithHeight(resp)

	if len(unavailableNodes) != 0 {
		unavailableMsg, err := executeTemplate("templates/nodes_status_unavailable", unavailableNodes, ext)
		if err != nil {
			return "", statusCondition, err
		}
		msg = unavailableMsg + "\n"
	}

	areHashesEqual := areStateHashesEqual(okNodes)
	if !areHashesEqual {
		differentHashes, err := executeTemplate("templates/nodes_status_different_hashes", okNodes, ext)
		if err != nil {
			return "", statusCondition, err
		}
		msg += fmt.Sprintf("%s %s", differentHashes, height)
		return msg, statusCondition, nil
	}
	equalHashes, err := executeTemplate("templates/nodes_status_ok", okNodes, ext)
	if err != nil {
		return "", statusCondition, err
	}

	if len(unavailableNodes) == 0 && len(okNodes) != 0 {
		statusCondition.AllNodesAreOk = true
		statusCondition.NodesNumber = len(okNodes)
		statusCondition.Height = height
	}

	msg += fmt.Sprintf("%s %s", equalHashes, height)
	return msg, statusCondition, nil
}

type nodesSingleChain struct {
	NodesNumber      int
	Height           string
	BlockID          string
	GeneratorAddress string
}

type nodesChains struct {
	Height string
	Chains []chain
}

type chain struct {
	BlockID          string
	GeneratorAddress string
}

func HandleNodesChains(
	resp *pair.NodesStatementsResponse,
	ext ExpectedExtension,
) (string, error) {
	if len(resp.NodesStatements) < 1 {
		return "", errors.New("failed to collect enough statements (< 1)")
	}

	sample := resp.NodesStatements[0]
	if sample.BlockID == nil {
		return "", errors.Errorf("block ID or generator are empty for node %s", sample.URL)
	}
	sampleGeneratorAddress := ""
	if sample.Generator != nil {
		sampleGeneratorAddress = sample.Generator.String()
	}
	var chains []chain
	chains = append(chains, chain{
		BlockID:          sample.BlockID.String(),
		GeneratorAddress: sampleGeneratorAddress,
	})
	height := sample.Height
	for i := 1; i < len(resp.NodesStatements); i++ {
		statement := resp.NodesStatements[i]
		if statement.BlockID == nil {
			return "", errors.Errorf("block ID is empty for node %s", statement.URL)
		}
		// can be empty for private nodes
		generatorAddress := ""
		if statement.Generator != nil {
			generatorAddress = statement.Generator.String()
		}
		if statement.BlockID.String() != sample.BlockID.String() && statement.Height == sample.Height {
			chains = append(chains, chain{
				BlockID:          statement.BlockID.String(),
				GeneratorAddress: generatorAddress,
			})
		}
	}

	/* we have more than one chain */
	if len(chains) > 1 {
		nodesChainsMsg := nodesChains{
			Height: strconv.FormatUint(height, 10),
			Chains: chains,
		}
		msg, err := executeTemplate("templates/nodes_chains_different", nodesChainsMsg, ext)
		if err != nil {
			return "", err
		}
		return msg, nil
	}

	nodeChainOk := nodesSingleChain{
		NodesNumber:      len(resp.NodesStatements),
		Height:           strconv.FormatUint(height, 10),
		BlockID:          sample.BlockID.String(),
		GeneratorAddress: sampleGeneratorAddress,
	}

	okMsg, err := executeTemplate("templates/nodes_chains_ok", nodeChainOk, ext)
	if err != nil {
		return "", err
	}

	return okMsg, nil
}

func areStateHashesEqual(okNodes []NodeStatus) bool {
	if len(okNodes) == 0 {
		return true
	}
	hashesEqual := true
	previousHash := okNodes[0].SumHash
	for _, node := range okNodes {
		if node.SumHash != previousHash {
			hashesEqual = false
		}
		previousHash = node.SumHash
	}
	return hashesEqual
}

func nodeURLToAlias(allNodes []entities.Node) map[string]string {
	nodesAliases := make(map[string]string)
	for _, n := range allNodes {
		if n.Alias != "" {
			nodesAliases[n.URL] = n.Alias
		}
	}
	return nodesAliases
}

func sortedNodesByStatusWithHeight(resp *pair.NodesStatementsResponse) ([]NodeStatus, []NodeStatus, string) {
	var (
		unavailable []NodeStatus
		okNodes     []NodeStatus
		height      string
	)
	for _, stat := range resp.NodesStatements {
		if stat.Status != entities.OK {
			unavailable = append(unavailable, NodeStatus{URL: stat.URL})
		} else {
			height = strconv.FormatUint(stat.Height, 10)
			s := NodeStatus{
				URL:     stat.URL,
				SumHash: stat.StateHash.SumHash.Hex(),
				Status:  string(stat.Status),
				Height:  height,
				BlockID: stat.StateHash.BlockID.String(),
			}
			okNodes = append(okNodes, s)
		}
	}
	sortNodesStatuses(unavailable)
	sortNodesStatuses(okNodes)
	return unavailable, okNodes, height
}

type nodeStatement struct {
	Node      string
	Height    uint64
	Timestamp int64
	StateHash string
	Version   string
}

func HandleNodeStatement(resp *pair.NodeStatementResponse, extension ExpectedExtension) (string, error) {
	resp.NodeStatement.Node = removeHTTPOrHTTPSScheme(resp.NodeStatement.Node)

	if resp.ErrMessage != "" {
		return resp.ErrMessage, nil
	}

	statement := nodeStatement{
		Node:      resp.NodeStatement.Node,
		Height:    resp.NodeStatement.Height,
		Timestamp: resp.NodeStatement.Timestamp,
		StateHash: resp.NodeStatement.StateHash.SumHash.Hex(),
		Version:   resp.NodeStatement.Version,
	}

	msg, err := executeTemplate("templates/node_statement", statement, extension)
	if err != nil {
		return "", err
	}

	return msg, nil
}

func constructMessage(
	alertType entities.AlertType,
	alertJSON []byte,
	extension ExpectedExtension,
	allNodes []entities.Node,
) (string, error) {
	msg, err := executeAlertTemplate(alertType, alertJSON, extension, allNodes)
	if err != nil {
		return "", errors.Wrap(err, "failed to execute an alert template")
	}
	return msg, nil
}
