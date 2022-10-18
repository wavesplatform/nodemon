package entities

type Platform byte

type ChatID int64

type Chat struct {
	ChatID   ChatID   `json:"chatID"`
	Platform Platform `json:"platform"`
}
