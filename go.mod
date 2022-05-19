module nodemon

go 1.18

// exclude vulnerable dependency: github.com/wavesplatform/gowaves ->
// -> github.com/prometheus/client_golang -> github.com/prometheus/common@v0.4.1 -> vulnerable
exclude github.com/gogo/protobuf v1.1.1

require (
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/jameycribbs/hare v0.6.0
	github.com/pkg/errors v0.9.1
	github.com/stretchr/testify v1.7.1
	github.com/tidwall/buntdb v1.2.9
	github.com/wavesplatform/gowaves v0.9.1-0.20220303094153-0e3aec83b7a6
	go.nanomsg.org/mangos/v3 v3.4.1
	gopkg.in/telebot.v3 v3.0.0
)

require (
	go.uber.org/goleak v1.1.12 // indirect
	golang.org/x/crypto v0.0.0-20220112180741-5e0467b6c7ce // indirect
	golang.org/x/sys v0.0.0-20220209214540-3681064d5158 // indirect
)
