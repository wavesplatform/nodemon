# Nodemon-discord - discord bot for `nodemon` monitoring service

## Options / Configuration parameters

Any option can be set in CLI parameter form and in environment variable form. CLI form has higher priority than
environment variable form.
To set an option as CLI parameter use _**kebab-case**_ option name.
To do the same as environment variable form use _**UPPER_SNAKE_CASE**_ option name.

### List of supported options in kebab-case form

- _-bind_ (string) — Local network address to bind the HTTP API of the service on.
- _-development_ (bool) — Development mode.
- _-discord-bot-token_ (string) — The secret token used to authenticate the bot
- _-discord-chat-id_ (string) — discord chat ID to send alerts through
- _-log-level_ (string) — Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level
  INFO. (default "INFO")
- _-nano-msg-pair-discord-url_ (string) — Nanomsg IPC URL for pair socket (default
  "ipc:///tmp/nano-msg-nodemon-pair.ipc")
- _-nano-msg-pubsub-url_ (string) — Nanomsg IPC URL for pubsub socket (default
  "ipc:///tmp/discord/nano-msg-nodemon-pubsub.ipc")

## Build requirements

- `Make` utility
- `Golang` toolchain

## Docker

To build docker image for this service execute these commands from **the root** of **the project**:

```shell
  docker build -t nodemon-discord -f ./Dockerfile-nodemon-discord .
```
