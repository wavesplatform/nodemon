package api

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/pkg/errors"

	"nodemon/cmd/bots/internal/bots/messaging"
	"nodemon/internal"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/tools"
	"nodemon/pkg/tools/logging/attrs"
)

const (
	defaultRequestNodesTimeout = 5 * time.Second
	botAPIShutdownTimeout      = 5 * time.Second
)

type mwLog struct{ l *log.Logger }

func newMWLog(logger *slog.Logger) mwLog {
	return mwLog{l: slog.NewLogLogger(logger.Handler(), slog.LevelInfo)}
}

func (m mwLog) Print(v ...any) { m.l.Print(v...) }

func (m mwLog) Println(v ...any) { m.l.Println(v...) }

type BotAPI struct {
	srv          *http.Server
	requestChan  chan<- pair.Request
	responseChan <-chan pair.Response
	logger       *slog.Logger
}

func NewBotAPI(
	bind string,
	requestChan chan<- pair.Request,
	responseChan <-chan pair.Response,
	apiReadTimeout time.Duration,
	logger *slog.Logger,
	development bool,
) (*BotAPI, error) {
	a := &BotAPI{
		logger:       logger,
		requestChan:  requestChan,
		responseChan: responseChan,
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestLogger(&middleware.DefaultLogFormatter{
		Logger:  newMWLog(logger),
		NoColor: !development,
	}))
	r.Use(middleware.Recoverer)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))
	r.Mount("/", a.routes(logger))
	a.srv = &http.Server{Addr: bind, Handler: r, ReadHeaderTimeout: apiReadTimeout, ReadTimeout: apiReadTimeout}
	return a, nil
}

func (a *BotAPI) routes(logger *slog.Logger) chi.Router {
	r := chi.NewRouter()
	r.Get("/health", a.health)
	r.Handle("/metrics", tools.PrometheusHTTPMetricsHandler(newMWLog(logger)))
	r.Get("/version", internal.VersionHTTPHandler)
	return r
}

func (a *BotAPI) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultRequestNodesTimeout)
	defer cancel()
	if _, err := messaging.RequestNodesWithCtx(ctx, a.requestChan, a.responseChan, false); err != nil {
		a.logger.Error("Healthcheck failed: failed to reach nodemon service and get non specific nodes",
			attrs.Error(err), slog.String("request-id", middleware.GetReqID(ctx)),
		)
		http.Error(w, fmt.Sprintf("Failed to get non specific nodes: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *BotAPI) StartCtx(ctx context.Context) error {
	var cfg net.ListenConfig
	l, listenErr := cfg.Listen(ctx, "tcp", a.srv.Addr)
	if listenErr != nil {
		return errors.Errorf("Failed to start REST API at '%s': %v", a.srv.Addr, listenErr)
	}
	go func() {
		err := a.srv.Serve(l)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logger.Error("Failed to serve REST API", slog.String("address", a.srv.Addr), attrs.Error(err))
			panic(err)
		}
	}()
	return nil
}

func (a *BotAPI) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), botAPIShutdownTimeout)
	defer cancel()

	if err := a.srv.Shutdown(ctx); err != nil {
		a.logger.Error("Failed to shutdown REST API", attrs.Error(err))
	}
}
