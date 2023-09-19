# Nodemon - main monitoring service

## Options / Configuration parameters

Any option can be set in CLI parameter form and in environment variable form. CLI form has higher priority than
environment variable form.
To set an option as CLI parameter use _**kebab-case**_ option name.
To do the same as environment variable form use _**UPPER_SNAKE_CASE**_ option name.

### List of supported options in kebab-case form

- _-api-read-timeout_ (duration) — HTTP API read timeout. Default value is 30s. (default 30s)
- _-base-target-threshold_ (int) — Base target threshold. Must be specified.
- _-bind_ (string) — Local network address to bind the HTTP API of the service on. Default value is ":8080". (default ":
  8080")
- _-interval_ (duration) — Polling interval, seconds. Default value is 60 (default 1m0s)
- _-log-level_ (string) — Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level
  INFO. (default "INFO")
- _-nano-msg-pair-discord-url_ (string) — Nanomsg IPC URL for pair socket
- _-nano-msg-pair-telegram-url_ (string) — Nanomsg IPC URL for pair socket
- _-nano-msg-pubsub-url_ (string) — Nanomsg IPC URL for pubsub socket (default "ipc:///tmp/nano-msg-pubsub.ipc")
- _-nodes_ (string) — Initial list of Waves Blockchain nodes to monitor. Provide comma separated list of REST API URLs
  here.
- _-retention_ (duration) — Events retention duration. Default value is 12h (default 12h0m0s)
- _-storage_ (string) — Path to nodes storage. Will be **ignored** if _-vault-address_ is set. (default ".nodes.json")
- _-timeout_ (duration) — Network timeout, seconds. Default value is 15 (default 15s)
- _-vault-address_ (string) — Vault server address.
- _-vault-mount-path_ (string) — Vault mount path for nodemon nodes storage. (default "gonodemonitoring")
- _-vault-password_ (string) — Vault user's password.
- _-vault-secret-path_ (string) — Vault secret where nodemon nodes will be saved
- _-vault-user_ (string) — Vault user.

## Build requirements

- `Make` utility
- `Golang` toolchain

## Docker

To build docker image for this service execute these commands from **the root** of **the project**:

```shell
  docker build -t nodemon -f ./Dockerfile-nodemon .
```
