build-bot-linux-amd64:
	@CGO_ENABLE=0 GOOS=linux GOARCH=amd64 go build -o bin/bot ./cmd/telegram_bot/bot.go
build-nodemon-linux-amd64:
	@CGO_ENABLE=0 GOOS=linux GOARCH=amd64 go build -o bin/nodemon ./cmd/nodemon/nodemon.go

