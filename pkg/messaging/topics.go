package messaging

import (
	"fmt"

	"nodemon/pkg/entities"
)

const pubSubTopicPrefix = "alerts"

func PubSubMsgTopic(scheme string, alertType entities.AlertType) string {
	alertName, ok := alertType.AlertName()
	if !ok {
		return fmt.Sprintf("%s_%s_alert-%d", pubSubTopicPrefix, scheme, uint(alertType))
	}
	return fmt.Sprintf("%s_%s_%s", pubSubTopicPrefix, scheme, alertName.String())
}

const botRequestsTopicPrefix = "bot_requests"

func TelegramBotRequestsTopic(scheme string) string {
	return fmt.Sprintf("%s_%s_telegram", botRequestsTopicPrefix, scheme)
}

func DiscordBotRequestsTopic(scheme string) string {
	return fmt.Sprintf("%s_%s_discord", botRequestsTopicPrefix, scheme)
}
