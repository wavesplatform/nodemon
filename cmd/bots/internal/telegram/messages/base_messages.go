package messages

import (
	"nodemon/cmd/bots/internal/common/messages"
)

const (
	HelpInfoText = messages.InfoMsg + " This is a bot for monitoring Waves nodes. The next commands are available:\n\n" +
		"/ping -  the command to check whether the bot is available and what his current state is\n" +
		"/start - the command to make the bot <b>start getting alerts</b>\n" +
		"/mute -  the command to make the bot <b>stop listening to alerts</b>\n" +
		"/pool -  to see the list of nodes and edit it\n" +
		"/subscriptions - to see the list of subscriptions and edit it\n" +
		"/status - to see the status of all nodes\n" +
		"/statement <b>node</b> <b>height</b> - to see a node statement at a specific height.\n" +
		"/add <b>node</b> - to add a node to the list\n" +
		"/remove <b>node</b> - to remove a node from the list\n" +
		"/subscribe <b>alert name</b> - to subscribe to a specific alert\n" +
		"/unsubscribe <b>alert name</b> - to unsubscribe from a specific alert\n" +
		"/add_alias <b>node</b> <b>alias</b>\n" +
		"/aliases - to see the matching list with aliases"

	MuteText  = "Say no more..." + messages.SleepingMsg
	PongText  = "Pong!" + messages.PongMsg
	StartText = "Started monitoring..." + messages.MonitoringMsg

	RemoveNode = `Please type the url of the node you want to remove
Example: Remove <url>
`
	AddNewNodeMsg = `Please type the url of the node you want to add
Example: Add <url>
`
	SubscribeTo = `Please type the name of alert you want to subscribe to
Example: Subscribe to <alert>
`
	UnsubscribeFrom = `Please type the name of alert you want to unsubscribe from
Example: Unsubscribe from <alert>
`
)
