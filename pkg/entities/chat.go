package entities

type Platform byte

type ChatID int64

const (
	TelegramPlatform Platform = iota + 1
	DiscordPlatform
)

type Chat struct {
	ChatID   ChatID   `json:"chatID"`
	Platform Platform `json:"platform"`
}
