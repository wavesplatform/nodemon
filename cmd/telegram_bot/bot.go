package main

import (
	"context"
	"fmt"
	"github.com/asaskevich/govalidator"
	//"github.com/joho/godotenv"
	"go.uber.org/zap"
	tele "gopkg.in/telebot.v3"
	"nodemon/pkg/bots/telegram/config"
	"os"
)

const(
	botToken = "5054144081:AAFeGslb-lhF3ujPAA2Ogtn1U5DbcC1U36U"
)
//func init() {
func in() {
	//err := godotenv.Load()
	if err := config.InitLog(); err != nil {
		panic(err)
	}

	//if err != nil {
	//	zap.S().Infof("No .env file found: %v", err)
	//}
	govalidator.SetFieldsRequiredByDefault(true)
}

func main() {
	err := run()
		switch err {
		case context.Canceled:
			os.Exit(130)
		case config.InvalidParameters:
			os.Exit(2)
		default:
			os.Exit(1)
		}
}

func run() error {
	//appConfig, err := config.LoadAppConfig()
	//if err != nil {
	//	return config.InvalidParameters
	//}
	in()
	webhook := &tele.Webhook{
		Listen:   ":8081",
		Endpoint: &tele.WebhookEndpoint{PublicURL: "https://mainnet-go-htz-fsn1-1.wavesnodes.com/bot"}, // , Cert: "webhook_cert.pem"
		//TLS:&tb.WebhookTLS{Key: "webhook_pkey.pem", Cert: "webhook_cert.pem"},
	}

	botSettings := tele.Settings{
		Token:  botToken,
		Poller: webhook,
	}

	bot, err := tele.NewBot(botSettings)
	if err != nil {
		zap.S().Fatalf("failed to start bot, %v", err)
	}

	bot.Handle("/hello", func(c tele.Context) error {
		return c.Send("Hello!")
	})

	/**/

	/**/
	fmt.Println(bot.URL)
	fmt.Print(bot.Token)
	bot.Start()

	zap.S().Info("finished")
	return nil
}

//curl -F "url=https://mainnet-go-htz-fsn1-1.wavesnodes.com/bot" https://api.telegram.org/bot5054144081:AAFeGslb-lhF3ujPAA2Ogtn1U5DbcC1U36U/setWebhookq
