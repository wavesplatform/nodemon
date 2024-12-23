# Nodemon - main monitoring service

## Options / Configuration parameters

Any option can be set in a CLI parameter or environment variable form. The CLI form has higher priority than
the environment variable form.
To set an option as a CLI parameter use _**kebab-case**_ option name.
To do the same as environment variable form use _**UPPER_SNAKE_CASE**_ option name.

### List of supported options in kebab-case form

- _-api-read-timeout_ (duration) — HTTP API read timeout used by the monitoring API server.
  Default value is 30s. (default 30s)
- _-base-target-threshold_ (int) — Base target threshold used for base target alerts. Must be specified.
- _-bind_ (string) — Local network address to bind the HTTP API of the service on. Default value is ":8080". (default ":
  8080")
- _-interval_ (duration) — Polling interval, seconds. Used for polling nodes for the analysis.
  Default value is 60 (default 1m0s)
- _-development_ (bool) — Development mode. It is used for zap logger.
- _-log-level_ (string) — Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level
  INFO. (default "INFO")
- _-nats-msg-url_ (string) — Nats URL for messaging (default "nats://127.0.0.1:4222").
  Used for communication with the discord bot, telegram bot and sending events for subscribers.
- _-nats-connection-timeout_ (string) — NATS connection to server timeout (default 5s).
- _-nodes_ (string) — Initial list of Waves Blockchain nodes to monitor. Provide comma separated list of REST API URLs
  here.
- _-retention_ (duration) — Events retention duration. Default value is 12h (default 12h0m0s)
- _-storage_ (string) — Path to nodes storage. Will be **ignored** if _-vault-address_ is set. (default ".nodes.json")
- _-timeout_ (duration) — Network timeout, seconds. Used by the poller to poll nodes with the timeout.
  Default value is 15 (default 15s)

#### Optional parameters

- _-vault-address_ (string) — Vault server address.
- _-vault-mount-path_ (string) — Vault mount path for nodemon nodes storage. (default "gonodemonitoring")
- _-vault-password_ (string) — Vault user's password.
- _-vault-secret-path_ (string) — Vault secret where nodemon nodes will be saved
- _-vault-user_ (string) — Vault user.

- _-nats-server-enable_ (bool) — Enable NATS embedded server (default _false_)
- _-nats-server-address_ (string) — NATS embedded server address in form 'host:port' (default "127.0.0.1:4222")
- _-nats-server-max-payload_ (uint64) — NATS embedded server URL (default 1MB)
- _-nats-server-ready-timeout_ (duration) — NATS server 'ready for connections' timeout (default 10s)

## Build requirements

- `Make` utility
- `Golang` toolchain

## Docker

To build docker image for this service execute these commands from **the root** of **the project**:

```shell
  docker build -t nodemon -f ./Dockerfile-nodemon .
```
