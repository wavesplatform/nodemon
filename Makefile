export GO111MODULE=on

SOURCE_DIRS = cmd pkg

.PHONY: vendor vetcheck fmtcheck clean build gotest mod-clean

all: vendor vetcheck fmtcheck build gotest mod-clean

vendor:
	go mod vendor

vetcheck:
	go vet ./...
	golangci-lint run -c .golangci.yml
	golangci-lint run -c .golangci-strict.yml --new-from-rev=origin/main

strict-vet-check:
	golangci-lint run -c .golangci-strict.yml

fmtcheck:
	@gofmt -l -s $(SOURCE_DIRS) | grep ".*\.go"; if [ "$$?" = "0" ]; then exit 1; fi

clean:
	rm -r build/

build:
	go build -o build/nodemon ./cmd/nodemon

gotest:
	go test -cover -race -covermode=atomic ./...

mod-clean:
	go mod tidy

build-bots-linux-amd64:
	@CGO_ENABLE=0 GOOS=linux GOARCH=amd64 go build -o build/linux-amd64/nodemon-telegram ./cmd/bots/telegram/telegram.go
	@CGO_ENABLE=0 GOOS=linux GOARCH=amd64 go build -o build/linux-amd64/nodemon-discord ./cmd/bots/discord/discord.go
build-nodemon-linux-amd64:
	@CGO_ENABLE=0 GOOS=linux GOARCH=amd64 go build -o build/linux-amd64/nodemon ./cmd/nodemon/nodemon.go
