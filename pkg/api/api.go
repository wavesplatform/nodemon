package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/pkg/errors"
	"nodemon/pkg/storing"
)

type API struct {
	srv     *http.Server
	storage *storing.ConfigurationStorage
}

func NewAPI(bind string, storage *storing.ConfigurationStorage) (*API, error) {
	a := &API{storage: storage}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.SetHeader("Content-Type", "application/json"))
	r.Mount("/", a.routes())
	return &API{srv: &http.Server{Addr: bind, Handler: r}}, nil
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
	return r
}

func (a *API) nodes(w http.ResponseWriter, _ *http.Request) {
	nodes, err := a.storage.Nodes()
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
	nodes, err := a.storage.EnabledNodes()
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
