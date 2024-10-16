export GO111MODULE=on

VERSION=$(shell git describe --tags --always --dirty)
SOURCE_DIRS = cmd pkg

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
	@go build -o build/nodemon -ldflags="-X 'nodemon/cmd/internal.version=$(VERSION)'" ./cmd/nodemon

gotest:
	go test -cover -race -covermode=atomic ./...

mod-clean:
	go mod tidy

build-bots-linux-amd64:
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/linux-amd64/nodemon-telegram -ldflags="-X 'nodemon/cmd/internal.version=$(VERSION)'" ./cmd/bots/telegram
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/linux-amd64/nodemon-discord -ldflags="-X 'nodemon/cmd/internal.version=$(VERSION)'" ./cmd/bots/discord

build-nodemon-linux-amd64:
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o build/linux-amd64/nodemon -ldflags="-X 'nodemon/cmd/internal.version=$(VERSION)'" ./cmd/nodemon
