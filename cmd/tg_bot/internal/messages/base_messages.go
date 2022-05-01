package messages

const (
	MonitoringMsg = "📡"
	SleepingMsg   = "💤"
	PongMsg       = "🏓"

	HelpInfoText = InfoMsg + " This is a bot for monitoring Waves nodes. The next commands are available:\n\n" +
		"/ping -  the command to check whether the bot is available and what his current state is\n" +
		"/hello - the command to make the bot <b>save this chat for alerts</b>. Needs to be done first time\n" +
		"/start - the command to make the bot <b>start getting alerts</b>\n" +
		"/mute -  the command to make the bot <b>stop listening to alerts</b>" +
		"/help -  the command to see <b>information about bot</b> and available commands"

	MuteText  = "Say no more..." + SleepingMsg
	PongText  = "Pong!" + PongMsg
	StartText = "Started monitoring..." + MonitoringMsg
)
