package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"
	"nodemon/pkg/storing/specific"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/pkg/errors"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"go.uber.org/zap"
)

const (
	megabyte           = 1 << 20
	apiShutdownTimeout = 5 * time.Second
)

type API struct {
	srv                *http.Server
	nodesStorage       nodes.Storage
	eventsStorage      *events.Storage
	zap                *zap.Logger
	privateNodesEvents specific.PrivateNodesEventsWriter
	atom               *zap.AtomicLevel
}

type mwLog struct{ *zap.Logger }

func (m mwLog) Print(v ...interface{}) { m.Sugar().Info(v...) }

func NewAPI(
	bind string,
	nodesStorage nodes.Storage,
	eventsStorage *events.Storage,
	apiReadTimeout time.Duration,
	logger *zap.Logger,
	privateNodesEvents specific.PrivateNodesEventsWriter,
	atom *zap.AtomicLevel,
	development bool,
) (*API, error) {
	a := &API{
		nodesStorage:       nodesStorage,
		eventsStorage:      eventsStorage,
		zap:                logger,
		privateNodesEvents: privateNodesEvents,
		atom:               atom,
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
	r.Mount("/", a.routes())
	a.srv = &http.Server{Addr: bind, Handler: r, ReadHeaderTimeout: apiReadTimeout, ReadTimeout: apiReadTimeout}
	return a, nil
}

func (a *API) Start() error {
	l, listenErr := net.Listen("tcp", a.srv.Addr)
	if listenErr != nil {
		return errors.Errorf("Failed to start REST API at '%s': %v", a.srv.Addr, listenErr)
	}
	go func() {
		err := a.srv.Serve(l)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.zap.Sugar().Fatalf("Failed to serve REST API at '%s': %v", a.srv.Addr, err)
		}
	}()
	return nil
}

func (a *API) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), apiShutdownTimeout)
	defer cancel()

	if err := a.srv.Shutdown(ctx); err != nil {
		a.zap.Error("Failed to shutdown REST API", zap.Error(err))
	}
}

func (a *API) routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/nodes/all", a.nodes)
	r.Get("/nodes/enabled", a.enabled)
	r.Post("/nodes/specific/statements", a.specificNodesHandler)
	r.Get("/health", a.health)
	r.Handle("/log/level", a.atom)
	return r
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	enabledNodes, enErr := a.nodesStorage.EnabledNodes()
	if enErr != nil {
		a.zap.Error("Healthcheck failed: failed get non specific nodes",
			zap.Error(enErr), zap.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to get non specific nodes: %v", enErr), http.StatusInternalServerError)
	}
	for _, node := range enabledNodes {
		_, lhErr := a.eventsStorage.LatestHeight(node.URL)
		if lhErr != nil && !errors.Is(lhErr, events.ErrNoFullStatement) {
			a.zap.Error("Healthcheck failed: failed to get latest height for node", zap.String("node", node.URL),
				zap.Error(lhErr), zap.String("request-id", middleware.GetReqID(r.Context())),
			)
			http.Error(w, fmt.Sprintf("Failed to get non specific nodes: %v", lhErr), http.StatusInternalServerError)
		}
	}
	w.WriteHeader(http.StatusOK)
}

type nodeShortStatement struct {
	Node    string `json:"node,omitempty"`
	Version string `json:"version,omitempty"`
	Height  int    `json:"height,omitempty"`
}

func (a *API) specificNodesHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, megabyte))
	if err != nil {
		a.zap.Error("Failed to read request body", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to read body: %v", err), http.StatusInternalServerError)
		return
	}
	// TODO remove these readers after implementing proper statehash structure
	stateHashReader := io.NopCloser(bytes.NewBuffer(body))
	statementReader := io.NopCloser(bytes.NewBuffer(body))

	statehash := &proto.StateHash{}
	err = json.NewDecoder(stateHashReader).Decode(statehash)
	if err != nil {
		a.zap.Error("Failed to decode statehash", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to decode statehash: %v", err), http.StatusInternalServerError)
		return
	}

	statement := nodeShortStatement{}
	err = json.NewDecoder(statementReader).Decode(&statement)
	if err != nil {
		a.zap.Error("Failed to decode a specific nodes statement", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to decode statements: %v", err), http.StatusInternalServerError)
		return
	}
	escapedNodeName := cleanCRLF(r.Header.Get("node-name"))
	updatedURL, err := entities.CheckAndUpdateURL(escapedNodeName)
	if err != nil {
		a.zap.Error("Failed to check and update node's url", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to check node name: %v", err), http.StatusInternalServerError)
		return
	}
	statement.Node = updatedURL

	enabledSpecificNodes, err := a.nodesStorage.EnabledSpecificNodes()
	if err != nil {
		a.zap.Error("Failed to fetch specific nodes from storage", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to fetch specific nodes from storage: %v", err), http.StatusInternalServerError)
		return
	}

	if !specificNodeFoundInStorage(enabledSpecificNodes, statement) {
		a.zap.Info("Received a statements from the private node but it's not being monitored by the nodemon",
			zap.String("node", statement.Node),
		)
		w.WriteHeader(http.StatusForbidden)
		return
	}
	const (
		zeroTS         = 0 // timestamp stub
		zeroBT         = 0 // base target stub
		minValidHeight = 2
	)
	if statement.Height < minValidHeight {
		invalidHeightEvent := entities.NewInvalidHeightEvent(
			statement.Node,
			zeroTS,
			statement.Version,
			statement.Height,
		)
		a.privateNodesEvents.Write(invalidHeightEvent)
		return
	}
	// TODO: these nodes don't send base target value at the moment
	// TODO: these nodes don't send a generator at the moment
	stateHashEvent := entities.NewStateHashEvent(
		statement.Node,
		zeroTS,
		statement.Version,
		statement.Height,
		statehash,
		zeroBT,
		&statehash.BlockID,
		nil,
	)
	a.privateNodesEvents.Write(stateHashEvent)

	if err = json.NewEncoder(w).Encode(statement); err != nil {
		a.zap.Error("Failed to marshal statements", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to marshal statements to JSON: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)

	sumhash := cleanCRLF(statehash.SumHash.Hex())
	a.zap.Sugar().Infof("Statement for node %s has been received, height %d, statehash %s\n",
		escapedNodeName, statement.Height, sumhash)
}

func specificNodeFoundInStorage(enabledSpecificNodes []entities.Node, statement nodeShortStatement) bool {
	foundInStorage := false
	for _, enabled := range enabledSpecificNodes {
		if enabled.URL == statement.Node {
			foundInStorage = true
		}
	}
	return foundInStorage
}

func cleanCRLF(s string) string {
	cleaned := strings.ReplaceAll(s, "\n", "")
	cleaned = strings.ReplaceAll(cleaned, "\r", "")
	return cleaned
}

func (a *API) nodes(w http.ResponseWriter, _ *http.Request) {
	regularNodes, err := a.nodesStorage.Nodes(false)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to complete request: %v", err), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(regularNodes)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to marshal nodes to JSON: %v", err), http.StatusInternalServerError)
		return
	}
}

func (a *API) enabled(w http.ResponseWriter, _ *http.Request) {
	enabledNodes, err := a.nodesStorage.EnabledNodes()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to complete request: %v", err), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(enabledNodes)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to marshal nodes to JSON: %v", err), http.StatusInternalServerError)
		return
	}
}
