export GO111MODULE=on

VERSION=$(shell git describe --tags --always --dirty)
SOURCE_DIRS = cmd pkg internal

.PHONY: vendor vetcheck fmtcheck clean build gotest mod-clean

all: vendor vetcheck fmtcheck build gotest mod-clean

vendor:
	go mod vendor

vetcheck:
	go vet ./...
	golangci-lint run -c .golangci.yml

fmtcheck:
	@gofmt -l -s $(SOURCE_DIRS) | grep ".*\.go"; if [ "$$?" = "0" ]; then exit 1; fi

clean:
	rm -r build/

build:
	@go build -o build/native/nodemon -ldflags="-X 'nodemon/internal.version=$(VERSION)'" ./cmd/nodemon
	@go build -o build/native/nodemon-telegram -ldflags="-X 'nodemon/internal.version=$(VERSION)'" ./cmd/bots/telegram
	@go build -o build/native/nodemon-discord -ldflags="-X 'nodemon/internal.version=$(VERSION)'" ./cmd/bots/discord

gotest:
	go test -cover -race -covermode=atomic ./...

mod-clean:
	go mod tidy

build-bots-linux-amd64: build-nodemon-telegram-linux-amd64 build-nodemon-discord-linux-amd64

build-nodemon-telegram-linux-amd64:
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/linux-amd64/nodemon-telegram -ldflags="-X 'nodemon/internal.version=$(VERSION)'" ./cmd/bots/telegram

build-nodemon-discord-linux-amd64:
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/linux-amd64/nodemon-discord -ldflags="-X 'nodemon/internal.version=$(VERSION)'" ./cmd/bots/discord

build-bots-linux-arm64: build-nodemon-telegram-linux-arm64 build-nodemon-discord-linux-arm64

build-nodemon-telegram-linux-arm64:
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o build/linux-arm64/nodemon-telegram -ldflags="-X 'nodemon/internal.version=$(VERSION)'" ./cmd/bots/telegram

build-nodemon-discord-linux-arm64:
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o build/linux-arm64/nodemon-discord -ldflags="-X 'nodemon/internal.version=$(VERSION)'" ./cmd/bots/discord

build-nodemon-linux-arm64:
	@CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o build/linux-arm64/nodemon -ldflags="-X 'nodemon/internal.version=$(VERSION)'" ./cmd/nodemon

build-nodemon-linux-amd64:
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/linux-amd64/nodemon -ldflags="-X 'nodemon/internal.version=$(VERSION)'" ./cmd/nodemon
