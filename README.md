<h1 align="center">🔷 📊 Waves Nodes Monitoring</h1>

## Description
Incident reports, nodes' status, fork detection and many other features are available. Discord and Telegram bots are supported.

## Available commands
`/status` command shows the current state and heights of monitored nodes.

`/add <node_url>` adds a new node to the list of those which are monitored

`/remove <node_url>` removes a node from the list

`/pool` sends the list of monitored nodes

`/subscribe` makes a bot subscribe to specific alerts and notify users about them

`/unsubscribe` removes an alert from the list of alerts to be sent

`/mute` to stop monitoring

`/start` to start monitoring




## Alerts

All monitored nodes are being polled constantly. Thus, it is possible to be notified of any accident.
Main alerts:

  - `Height alert` is sent if the current height of one (or more) of the nodes is noticeably lower than other's
  
  - `State Hash alerts`is sent if one (or more) of the nodes handled a transaction differently
  
  - `Unreachable alert` is sent if one (or more) of the nodes are unavailable due to network disconnection or an internal error
  
  
  Telegram bot, Discord bot and Monitoring services are different services, however the bots depend on the information which Monitoring service provides
