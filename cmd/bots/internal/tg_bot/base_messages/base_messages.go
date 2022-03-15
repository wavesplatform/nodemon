package base_messages

import tele "gopkg.in/telebot.v3"

const (
	HelpInfoText2 = "This is a bot for monitoring Waves nodes. The next commands are available:\n\n" +
		"/ping - a simple command to check whether the bot is available\n" +
		"/hello - the command to make the bot <b>save this chat for alerts</b>. Needs to be done first time\n" +
		"/start - the command to make the bot <b>start getting alerts</b>\n" +
		"/mute -  the command to make the bot <b>stop listening to alerts</b>"
)

func getCommands() []string {
	return []string{"/ping", "/hello", "/start", "/mute", "/help"}
}

func HelpCommandKeyboard() [][]tele.ReplyButton {
	commands := getCommands()
	var keyboard = make([][]tele.ReplyButton, 0)
	for i, command := range commands {
		if i%2 == 0 {
			keyboard = append(keyboard, []tele.ReplyButton{})
		}
		keyboard[i/2] = append(keyboard[i/2], tele.ReplyButton{Text: command})
	}

	return keyboard
}
