package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	stderrs "errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	"nodemon/internal"
	"nodemon/pkg/entities"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"
	"nodemon/pkg/storing/specific"
	"nodemon/pkg/tools"
	"nodemon/pkg/tools/logging"
	"nodemon/pkg/tools/logging/attrs"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/pkg/errors"
	"github.com/wavesplatform/gowaves/pkg/proto"
)

const (
	kb                       = 1 << 10
	specificNodeRequestLimit = 10 * kb
	apiShutdownTimeout       = 5 * time.Second
)

type API struct {
	srv                *http.Server
	nodesStorage       nodes.Storage
	eventsStorage      *events.Storage
	logger             *slog.Logger
	privateNodesEvents specific.PrivateNodesEventsWriter
}

type mwLog struct{ l *log.Logger }

func newMWLog(logger *slog.Logger) mwLog {
	return mwLog{l: slog.NewLogLogger(logger.Handler(), slog.LevelInfo)}
}

func (m mwLog) Print(v ...any) { m.l.Print(v...) }

func (m mwLog) Println(v ...any) { m.l.Println(v...) }

const namespace = "API"

func NewAPI(
	bind string,
	nodesStorage nodes.Storage,
	eventsStorage *events.Storage,
	apiReadTimeout time.Duration,
	logger *slog.Logger,
	privateNodesEvents specific.PrivateNodesEventsWriter,
	development bool,
) (*API, error) {
	logger = logger.With(
		slog.String(logging.NamespaceKey, namespace),
	)
	a := &API{
		nodesStorage:       nodesStorage,
		eventsStorage:      eventsStorage,
		logger:             logger,
		privateNodesEvents: privateNodesEvents,
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

func (a *API) Start() error { // left for compatibility
	return a.StartCtx(context.Background())
}

func (a *API) StartCtx(ctx context.Context) error {
	var cfg net.ListenConfig
	l, listenErr := cfg.Listen(ctx, "tcp", a.srv.Addr)
	if listenErr != nil {
		return errors.Errorf("Failed to start REST API at '%s': %v", a.srv.Addr, listenErr)
	}
	go func() {
		err := a.srv.Serve(l)
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.logger.Error("Failed to serve REST API",
				slog.String("address", a.srv.Addr), attrs.Error(err),
			)
			panic(err)
		}
	}()
	return nil
}

func (a *API) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), apiShutdownTimeout)
	defer cancel()

	if err := a.srv.Shutdown(ctx); err != nil {
		a.logger.Error("Failed to shutdown REST API", attrs.Error(err))
	}
}

func (a *API) routes(logger *slog.Logger) chi.Router {
	r := chi.NewRouter()
	r.Get("/nodes/all", a.nodes)
	r.Get("/nodes/enabled", a.enabled)
	r.Post("/nodes/specific/statements", a.specificNodesHandler)
	r.Get("/health", a.health)
	r.Handle("/metrics", tools.PrometheusHTTPMetricsHandler(newMWLog(logger)))
	r.Get("/version", internal.VersionHTTPHandler)
	return r
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	enabledNodes, enErr := a.nodesStorage.EnabledNodes()
	if enErr != nil {
		a.logger.Error("Healthcheck failed: failed get non specific nodes",
			attrs.Error(enErr), slog.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to get non specific nodes: %v", enErr), http.StatusInternalServerError)
	}
	for _, node := range enabledNodes {
		_, lhErr := a.eventsStorage.LatestHeight(node.URL)
		if lhErr != nil && !errors.Is(lhErr, events.ErrNoFullStatement) {
			a.logger.Error("Healthcheck failed: failed to get latest height for node", slog.String("node", node.URL),
				attrs.Error(lhErr), slog.String("request-id", middleware.GetReqID(r.Context())),
			)
			http.Error(w, fmt.Sprintf("Failed to get non specific nodes: %v", lhErr), http.StatusInternalServerError)
		}
	}
	w.WriteHeader(http.StatusOK)
}

type nodeShortStatement struct {
	Node    string `json:"node,omitempty"`
	Version string `json:"version,omitempty"`
	Height  uint64 `json:"height,omitempty"`
}

type statementLogWrapper nodeShortStatement

func (s statementLogWrapper) String() string {
	data, err := json.Marshal(s)
	if err != nil {
		type jsonErr struct {
			Error string `json:"error"`
		}
		errData, mErr := json.Marshal(jsonErr{Error: err.Error()})
		if mErr != nil { // must never happen
			panic(stderrs.Join(err, mErr))
		}
		return string(errData)
	}
	return string(data)
}

type hexBodyStringer []byte

func (b hexBodyStringer) String() string { return hex.EncodeToString(b) }

func parseStatementAndStateHash(
	w http.ResponseWriter,
	r *http.Request,
	body []byte,
	logger *slog.Logger,
) (*nodeShortStatement, *proto.StateHash, bool) {
	statementReader := io.Reader(bytes.NewBuffer(body))
	// TODO remove these readers after implementing proper statehash structure
	stateHashReader := io.Reader(bytes.NewBuffer(body))

	statement := &nodeShortStatement{}
	err := json.NewDecoder(statementReader).Decode(&statement)
	if err != nil {
		logger.Error("Failed to decode a specific nodes statement",
			attrs.Error(err),
			slog.String("request-id", middleware.GetReqID(r.Context())),
			attrs.Stringer("hex-body", hexBodyStringer(body)),
		)
		http.Error(w, fmt.Sprintf("Failed to decode statements: %v", err), http.StatusBadRequest)
		return nil, nil, false
	}
	statehash := &proto.StateHash{}
	err = json.NewDecoder(stateHashReader).Decode(statehash)
	if err != nil {
		logger.Error("Failed to decode statehash",
			attrs.Error(err),
			slog.String("request-id", middleware.GetReqID(r.Context())),
			attrs.Stringer("hex-body", hexBodyStringer(body)),
		)
		http.Error(w, fmt.Sprintf("Failed to decode statehash: %v", err), http.StatusBadRequest)
		return statement, nil, false
	}

	escapedNodeName := cleanCRLF(r.Header.Get("node-name"))
	updatedURL, err := entities.CheckAndUpdateURL(escapedNodeName)
	if err != nil {
		logger.Error("Failed to check and update node's url",
			attrs.Error(err),
			slog.String("request-id", middleware.GetReqID(r.Context())),
			attrs.Stringer("hex-body", hexBodyStringer(body)),
		)
		http.Error(w, fmt.Sprintf("Failed to check node name: %v", err), http.StatusBadRequest)
		return statement, statehash, false
	}
	statement.Node = updatedURL
	logger.Debug("Received statement",
		attrs.Stringer("statement", (*statementLogWrapper)(statement)),
		slog.String("request-id", middleware.GetReqID(r.Context())),
	)
	return statement, statehash, true
}

func (a *API) specificNodesHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, specificNodeRequestLimit))
	if err != nil {
		a.logger.Error("Failed to read request body", attrs.Error(err))
		http.Error(w, fmt.Sprintf("Failed to read body: %v", err), http.StatusInternalServerError)
		return
	}
	if len(body) == 0 {
		a.logger.Error("Empty request body", slog.String("request-id", middleware.GetReqID(r.Context())))
		http.Error(w, "Empty request body", http.StatusBadRequest)
		return
	}
	statement, statehash, ok := parseStatementAndStateHash(w, r, body, a.logger)
	if !ok {
		return // error is already written
	}
	enabledSpecificNodes, err := a.nodesStorage.EnabledSpecificNodes()
	if err != nil {
		a.logger.Error("Failed to fetch specific nodes from storage",
			attrs.Error(err),
			slog.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to fetch specific nodes from storage: %v", err), http.StatusInternalServerError)
		return
	}

	if !specificNodeFoundInStorage(enabledSpecificNodes, statement) {
		a.logger.Info("Received a statements from the private node but it's not being monitored by the nodemon",
			slog.String("node", statement.Node),
			slog.String("request-id", middleware.GetReqID(r.Context())),
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
	// TODO: support the generator field
	stateHashEvent := entities.NewStateHashEvent(
		statement.Node,
		zeroTS,
		statement.Version,
		statement.Height,
		statehash,
		zeroBT,
		&statehash.BlockID,
		nil,
		false,
	)
	a.privateNodesEvents.Write(stateHashEvent)

	if err = json.NewEncoder(w).Encode(statement); err != nil {
		a.logger.Error("Failed to marshal statements",
			attrs.Error(err),
			slog.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to marshal statements to JSON: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)

	sumhash := cleanCRLF(statehash.SumHash.Hex())
	a.logger.Info("Statement for node has been received",
		slog.String("node", statement.Node),
		slog.Uint64("height", statement.Height),
		slog.String("statehash", sumhash),
	)
}

func specificNodeFoundInStorage(enabledSpecificNodes []entities.Node, statement *nodeShortStatement) bool {
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

type nodesResponse struct {
	Regular  proto.NonNullableSlice[entities.Node] `json:"regular"`
	Specific proto.NonNullableSlice[entities.Node] `json:"specific"`
}

func (a *API) nodes(w http.ResponseWriter, r *http.Request) {
	regularNodes, err := a.nodesStorage.Nodes(false)
	if err != nil {
		a.logger.Error("Failed to fetch regular nodes from storage",
			attrs.Error(err),
			slog.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to complete request: %v", err), http.StatusInternalServerError)
		return
	}
	specificNodes, err := a.nodesStorage.Nodes(true)
	if err != nil {
		a.logger.Error("Failed to fetch specific nodes from storage",
			attrs.Error(err),
			slog.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to complete request: %v", err), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(nodesResponse{Regular: regularNodes, Specific: specificNodes})
	if err != nil {
		a.logger.Error("Failed to marshal nodes",
			attrs.Error(err),
			slog.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to marshal nodes to JSON: %v", err), http.StatusInternalServerError)
		return
	}
}

func (a *API) enabled(w http.ResponseWriter, r *http.Request) {
	enabledRegularNodes, err := a.nodesStorage.EnabledNodes()
	if err != nil {
		a.logger.Error("Failed to fetch enabled regular nodes from storage",
			attrs.Error(err),
			slog.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to complete request: %v", err), http.StatusInternalServerError)
		return
	}
	enabledSpecificNodes, err := a.nodesStorage.EnabledSpecificNodes()
	if err != nil {
		a.logger.Error("Failed to fetch enabled specific nodes from storage",
			attrs.Error(err),
			slog.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to complete request: %v", err), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(nodesResponse{Regular: enabledRegularNodes, Specific: enabledSpecificNodes})
	if err != nil {
		a.logger.Error("Failed to marshal nodes",
			attrs.Error(err),
			slog.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to marshal nodes to JSON: %v", err), http.StatusInternalServerError)
		return
	}
}
