module nodemon

go 1.18

// exclude vulnerable dependency: github.com/wavesplatform/gowaves ->
// -> github.com/prometheus/client_golang -> github.com/prometheus/common@v0.4.1 -> vulnerable
exclude github.com/gogo/protobuf v1.1.1

require (
	github.com/go-chi/chi v4.1.2+incompatible
	github.com/jameycribbs/hare v0.6.0
	github.com/pkg/errors v0.9.1
	github.com/procyon-projects/chrono v1.1.0
	github.com/stretchr/testify v1.8.0
	github.com/tidwall/buntdb v1.2.10
	github.com/wavesplatform/gowaves v0.10.2-0.20220817081803-6008477aab3d
	go.nanomsg.org/mangos/v3 v3.4.2
	gopkg.in/telebot.v3 v3.0.0
)

require (
	filippo.io/edwards25519 v1.0.0 // indirect
	github.com/Microsoft/go-winio v0.5.2 // indirect
	github.com/btcsuite/btcd v0.22.1 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/golang/protobuf v1.5.2 // indirect
	github.com/gorilla/websocket v1.5.0 // indirect
	github.com/jinzhu/copier v0.3.5 // indirect
	github.com/kilic/bls12-381 v0.0.0-20200820230200-6b2c19996391 // indirect
	github.com/mr-tron/base58 v1.2.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/tidwall/btree v1.4.2 // indirect
	github.com/tidwall/gjson v1.14.3 // indirect
	github.com/tidwall/grect v0.1.4 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.0 // indirect
	github.com/tidwall/rtred v0.1.2 // indirect
	github.com/tidwall/tinyqueue v0.1.1 // indirect
	github.com/umbracle/fastrlp v0.0.0-20210128110402-41364ca56ca8 // indirect
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.uber.org/goleak v1.1.12 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.22.0 // indirect
	golang.org/x/crypto v0.0.0-20220112180741-5e0467b6c7ce // indirect
	golang.org/x/net v0.0.0-20220225172249-27dd8689420f // indirect
	golang.org/x/sys v0.0.0-20220520151302-bc2c85ada10a // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/genproto v0.0.0-20210226172003-ab064af71705 // indirect
	google.golang.org/grpc v1.48.0 // indirect
	google.golang.org/protobuf v1.28.1 // indirect
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
