package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/pkg/errors"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"
)

type API struct {
	srv                   *http.Server
	nodesStorage          *nodes.Storage
	eventsStorage         *events.Storage
	specificNodesSettings SpecificNodesSettings
}

type SpecificNodesSettings struct {
	currentTimestamp *int64
}

func NewAPI(bind string, nodesStorage *nodes.Storage, eventsStorage *events.Storage, specificNodesTs *int64) (*API, error) {
	a := &API{nodesStorage: nodesStorage, eventsStorage: eventsStorage, specificNodesSettings: SpecificNodesSettings{currentTimestamp: specificNodesTs}}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))
	r.Mount("/", a.routes())
	a.srv = &http.Server{Addr: bind, Handler: r}
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
			log.Printf("Failed to serve REST API at '%s': %v", a.srv.Addr, err)
		}
	}()
	return nil
}

func (a *API) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.srv.Shutdown(ctx); err != nil {
		log.Printf("Failed to shutdown REST API properly: %v", err)
	}
}

func (a *API) routes() chi.Router {
	r := chi.NewRouter()
	r.Get("/nodes/all", a.nodes)
	r.Get("/nodes/enabled", a.enabled)
	r.Post("/nodes/ping", a.ping)
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
	buf, _ := ioutil.ReadAll(r.Body)
	reader1 := ioutil.NopCloser(bytes.NewBuffer(buf))
	reader2 := ioutil.NopCloser(bytes.NewBuffer(buf))
	statehash := &proto.StateHash{}

	err := DecodeValueFromBody(reader1, statehash)
	if err != nil {
		log.Printf("Failed to decode statehash: %v", err)
		http.Error(w, fmt.Sprintf("Failed to decode statehash: %v", err), http.StatusInternalServerError)
		return
	}

	nodeName := r.Header.Get("node-name")
	statement := NodeShortStatement{}
	err = json.NewDecoder(reader2).Decode(&statement)
	if err != nil {
		log.Printf("Failed to decode specific nodes statements: %v", err)
		http.Error(w, fmt.Sprintf("Failed to decode statements: %v", err), http.StatusInternalServerError)
		return
	}
	statement.Node = nodeName

	if a.specificNodesSettings.currentTimestamp == nil {
		log.Print("current timestamp of analyzed nodes is nil yet")
		w.WriteHeader(http.StatusOK)
		return
	}
	currentTs := *a.specificNodesSettings.currentTimestamp

	var events []entities.Event

	if statement.Height < 2 {
		invalidHeihtEvent := entities.NewInvalidHeightEvent(statement.Node, currentTs, statement.Version, statement.Height)
		err := a.eventsStorage.PutEvent(invalidHeihtEvent)
		if err != nil {
			log.Printf("Failed to put event: %v", err)
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
			log.Printf("Failed to put event: %v", err)
		}
	}

	err = json.NewEncoder(w).Encode(statement)
	if err != nil {
		log.Printf("Failed to marshal statements: %v", err)
		http.Error(w, fmt.Sprintf("Failed to marshal statements to JSON: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)

	log.Printf("Statement for node %s has been put into the storage, height %d, statehash %s\n", nodeName, h, statehash.SumHash.String())
}

func (a *API) ping(w http.ResponseWriter, _ *http.Request) {
	_, err := w.Write([]byte("PONG!!"))
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to marshal pong to JSON: %v", err), http.StatusInternalServerError)
		return
	}
}

func (a *API) nodes(w http.ResponseWriter, _ *http.Request) {
	nodes, err := a.nodesStorage.Nodes()
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
