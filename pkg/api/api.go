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

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/pkg/errors"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"go.uber.org/atomic"
	"go.uber.org/zap"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"
)

type API struct {
	srv                   *http.Server
	nodesStorage          *nodes.Storage
	eventsStorage         *events.Storage
	specificNodesSettings specificNodesSettings
	zap                   *zap.Logger
}

type specificNodesSettings struct {
	currentTimestamp *atomic.Int64
}

func NewAPI(bind string, nodesStorage *nodes.Storage, eventsStorage *events.Storage, specificNodesTs *atomic.Int64, apiReadTimeout time.Duration, logger *zap.Logger) (*API, error) {
	a := &API{nodesStorage: nodesStorage, eventsStorage: eventsStorage, specificNodesSettings: specificNodesSettings{currentTimestamp: specificNodesTs}, zap: logger}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))
	r.Mount("/", a.routes())
	a.srv = &http.Server{Addr: bind, Handler: r, ReadHeaderTimeout: apiReadTimeout, ReadTimeout: apiReadTimeout}
	return a, nil
}

func (a *API) Start() error {
	l, err := net.Listen("tcp", a.srv.Addr)
	if err != nil {
		return errors.Errorf("Failed to start REST API at '%s': %v", a.srv.Addr, err)
	}
	go func() {
		err := a.srv.Serve(l)
		if err != nil && err != http.ErrServerClosed {
			a.zap.Fatal(fmt.Sprintf("Failed to serve REST API at '%s': %v", a.srv.Addr, err))
		}
	}()
	return nil
}

func (a *API) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
	return r
}

type NodeShortStatement struct {
	Node    string `json:"node,omitempty"`
	Version string `json:"version,omitempty"`
	Height  int    `json:"height,omitempty"`
}

func DecodeValueFromBody(body io.ReadCloser, v interface{}) error {
	if err := json.NewDecoder(body).Decode(v); err != nil {
		return err
	}
	return nil
}

func (a *API) specificNodesHandler(w http.ResponseWriter, r *http.Request) {
	buf, err := io.ReadAll(r.Body)
	if err != nil {
		a.zap.Error("Failed to read request body", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to read body: %v", err), http.StatusInternalServerError)
		return
	}
	// TODO remove these readers after implementing proper statehash structure
	stateHashReader := io.NopCloser(bytes.NewBuffer(buf))
	statementReader := io.NopCloser(bytes.NewBuffer(buf))
	statehash := &proto.StateHash{}

	err = DecodeValueFromBody(stateHashReader, statehash)
	if err != nil {
		a.zap.Error("Failed to decode statehash", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to decode statehash: %v", err), http.StatusInternalServerError)
		return
	}

	nodeName := r.Header.Get("node-name")
	escapedNodeName := strings.Replace(nodeName, "\n", "", -1)
	escapedNodeName = strings.Replace(escapedNodeName, "\r", "", -1)

	statement := NodeShortStatement{}
	err = json.NewDecoder(statementReader).Decode(&statement)
	if err != nil {
		a.zap.Error("Failed to decode a specific nodes statement", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to decode statements: %v", err), http.StatusInternalServerError)
		return
	}
	updatedUrl, err := entities.CheckAndUpdateURL(escapedNodeName)
	if err != nil {
		a.zap.Error("Failed to check and update node's url", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to check node name: %v", err), http.StatusInternalServerError)
		return
	}
	statement.Node = updatedUrl

	if a.specificNodesSettings.currentTimestamp == nil {
		a.zap.Error("current timestamp of analyzed nodes is nil yet")
		w.WriteHeader(http.StatusOK)
		return
	}
	currentTs := a.specificNodesSettings.currentTimestamp.Load()

	var events []entities.Event

	if statement.Height < 2 {
		invalidHeightEvent := entities.NewInvalidHeightEvent(statement.Node, currentTs, statement.Version, statement.Height)
		err := a.eventsStorage.PutEvent(invalidHeightEvent)
		if err != nil {
			a.zap.Error("Failed to put invalid height event", zap.Error(err))
		}
		return
	}

	versionEvent := entities.NewVersionEvent(statement.Node, currentTs, statement.Version)
	events = append(events, versionEvent)

	heightEvent := entities.NewHeightEvent(statement.Node, currentTs, statement.Version, statement.Height)
	events = append(events, heightEvent)

	stateHashEvent := entities.NewStateHashEvent(statement.Node, currentTs, statement.Version, statement.Height, statehash)
	events = append(events, stateHashEvent)

	for _, event := range events {
		err := a.eventsStorage.PutEvent(event)
		if err != nil {
			a.zap.Error("Failed to put event", zap.Error(err))
		}
	}

	err = json.NewEncoder(w).Encode(statement)
	if err != nil {
		a.zap.Error("Failed to marshal statements", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to marshal statements to JSON: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)

	sumhash := strings.Replace(statehash.SumHash.Hex(), "\n", "", -1)
	sumhash = strings.Replace(sumhash, "\r", "", -1)
	a.zap.Info(fmt.Sprintf("Statement for node %s has been put into the storage, height %d, statehash %s\n", escapedNodeName, statement.Height, sumhash))
}

func (a *API) nodes(w http.ResponseWriter, _ *http.Request) {
	nodes, err := a.nodesStorage.Nodes(false)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to complete request: %v", err), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(nodes)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to marshal nodes to JSON: %v", err), http.StatusInternalServerError)
		return
	}
}

func (a *API) enabled(w http.ResponseWriter, _ *http.Request) {
	nodes, err := a.nodesStorage.EnabledNodes()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to complete request: %v", err), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(nodes)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to marshal nodes to JSON: %v", err), http.StatusInternalServerError)
		return
	}
}
