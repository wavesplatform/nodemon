package api

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"nodemon/cmd/bots/internal/common/messaging"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/tools"
)

const (
	defaultRequestNodesTimeout = 5 * time.Second
	botAPIShutdownTimeout      = 5 * time.Second
)

type mwLog struct{ *zap.Logger }

func (m mwLog) Print(v ...interface{}) { m.Sugar().Info(v...) }

func (m mwLog) Println(v ...interface{}) { m.Sugar().Infoln(v...) }

type BotAPI struct {
	srv          *http.Server
	requestChan  chan<- pair.Request
	responseChan <-chan pair.Response
	zap          *zap.Logger
	atom         *zap.AtomicLevel
}

func NewBotAPI(
	bind string,
	requestChan chan<- pair.Request,
	responseChan <-chan pair.Response,
	apiReadTimeout time.Duration,
	logger *zap.Logger,
	atom *zap.AtomicLevel,
	development bool,
) (*BotAPI, error) {
	a := &BotAPI{
		zap:          logger,
		requestChan:  requestChan,
		responseChan: responseChan,
		atom:         atom,
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestLogger(&middleware.DefaultLogFormatter{
		Logger:  mwLog{logger},
		NoColor: !development,
	}))
	r.Use(middleware.Recoverer)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))
	r.Mount("/", a.routes(logger))
	a.srv = &http.Server{Addr: bind, Handler: r, ReadHeaderTimeout: apiReadTimeout, ReadTimeout: apiReadTimeout}
	return a, nil
}

func (a *BotAPI) routes(logger *zap.Logger) chi.Router {
	r := chi.NewRouter()
	r.Get("/health", a.health)
	r.Handle("/log/level", a.atom)
	r.Handle("/metrics", tools.PrometheusHTTPMetricsHandler(mwLog{logger}))
	return r
}

func (a *BotAPI) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), defaultRequestNodesTimeout)
	defer cancel()
	if _, err := messaging.RequestNodesWithCtx(ctx, a.requestChan, a.responseChan, false); err != nil {
		a.zap.Error("Healthcheck failed: failed to reach nodemon service and get non specific nodes",
			zap.Error(err), zap.String("request-id", middleware.GetReqID(ctx)),
		)
		http.Error(w, fmt.Sprintf("Failed to get non specific nodes: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *BotAPI) Start() error {
	l, listenErr := net.Listen("tcp", a.srv.Addr)
	if listenErr != nil {
		return errors.Errorf("Failed to start REST API at '%s': %v", a.srv.Addr, listenErr)
	}
	go func() {
		err := a.srv.Serve(l)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.zap.Fatal("Failed to serve REST API", zap.String("address", a.srv.Addr), zap.Error(err))
		}
	}()
	return nil
}

func (a *BotAPI) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), botAPIShutdownTimeout)
	defer cancel()

	if err := a.srv.Shutdown(ctx); err != nil {
		a.zap.Error("Failed to shutdown REST API", zap.Error(err))
	}
}
