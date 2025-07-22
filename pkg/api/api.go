package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	stderrs "errors"
	"fmt"
	"io"
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

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/pkg/errors"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"go.uber.org/zap"
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
	zap                *zap.Logger
	privateNodesEvents specific.PrivateNodesEventsWriter
	atom               *zap.AtomicLevel
}

type mwLog struct{ *zap.Logger }

func (m mwLog) Print(v ...interface{}) { m.Sugar().Info(v...) }

func (m mwLog) Println(v ...interface{}) { m.Sugar().Infoln(v...) }

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
			a.zap.Sugar().Fatalf("[API] Failed to serve REST API at '%s': %v", a.srv.Addr, err)
		}
	}()
	return nil
}

func (a *API) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), apiShutdownTimeout)
	defer cancel()

	if err := a.srv.Shutdown(ctx); err != nil {
		a.zap.Error("[API] Failed to shutdown REST API", zap.Error(err))
	}
}

func (a *API) routes(logger *zap.Logger) chi.Router {
	r := chi.NewRouter()
	r.Get("/nodes/all", a.nodes)
	r.Get("/nodes/enabled", a.enabled)
	r.Post("/nodes/specific/statements", a.specificNodesHandler)
	r.Get("/health", a.health)
	r.Handle("/log/level", a.atom)
	r.Handle("/metrics", tools.PrometheusHTTPMetricsHandler(mwLog{logger}))
	r.Get("/version", internal.VersionHTTPHandler)
	return r
}

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	enabledNodes, enErr := a.nodesStorage.EnabledNodes()
	if enErr != nil {
		a.zap.Error("[API] Healthcheck failed: failed get non specific nodes",
			zap.Error(enErr), zap.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to get non specific nodes: %v", enErr), http.StatusInternalServerError)
	}
	for _, node := range enabledNodes {
		_, lhErr := a.eventsStorage.LatestHeight(node.URL)
		if lhErr != nil && !errors.Is(lhErr, events.ErrNoFullStatement) {
			a.zap.Error("[API] Healthcheck failed: failed to get latest height for node", zap.String("node", node.URL),
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
	logger *zap.Logger,
) (*nodeShortStatement, *proto.StateHash, bool) {
	statementReader := io.Reader(bytes.NewBuffer(body))
	// TODO remove these readers after implementing proper statehash structure
	stateHashReader := io.Reader(bytes.NewBuffer(body))

	statement := &nodeShortStatement{}
	err := json.NewDecoder(statementReader).Decode(&statement)
	if err != nil {
		logger.Error("[API] Failed to decode a specific nodes statement",
			zap.Error(err),
			zap.String("request-id", middleware.GetReqID(r.Context())),
			zap.Stringer("hex-body", hexBodyStringer(body)),
		)
		http.Error(w, fmt.Sprintf("Failed to decode statements: %v", err), http.StatusBadRequest)
		return nil, nil, false
	}
	statehash := &proto.StateHash{}
	err = json.NewDecoder(stateHashReader).Decode(statehash)
	if err != nil {
		logger.Error("[API] Failed to decode statehash",
			zap.Error(err),
			zap.String("request-id", middleware.GetReqID(r.Context())),
			zap.Stringer("hex-body", hexBodyStringer(body)),
		)
		http.Error(w, fmt.Sprintf("Failed to decode statehash: %v", err), http.StatusBadRequest)
		return statement, nil, false
	}

	escapedNodeName := cleanCRLF(r.Header.Get("node-name"))
	updatedURL, err := entities.CheckAndUpdateURL(escapedNodeName)
	if err != nil {
		logger.Error("[API] Failed to check and update node's url",
			zap.Error(err),
			zap.String("request-id", middleware.GetReqID(r.Context())),
			zap.Stringer("hex-body", hexBodyStringer(body)),
		)
		http.Error(w, fmt.Sprintf("Failed to check node name: %v", err), http.StatusBadRequest)
		return statement, statehash, false
	}
	statement.Node = updatedURL
	logger.Debug("[API] Received statement",
		zap.Stringer("statement", (*statementLogWrapper)(statement)),
		zap.String("request-id", middleware.GetReqID(r.Context())),
	)
	return statement, statehash, true
}

func (a *API) specificNodesHandler(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, specificNodeRequestLimit))
	if err != nil {
		a.zap.Error("[API] Failed to read request body", zap.Error(err))
		http.Error(w, fmt.Sprintf("Failed to read body: %v", err), http.StatusInternalServerError)
		return
	}
	if len(body) == 0 {
		a.zap.Error("[API] Empty request body", zap.String("request-id", middleware.GetReqID(r.Context())))
		http.Error(w, "Empty request body", http.StatusBadRequest)
		return
	}
	statement, statehash, ok := parseStatementAndStateHash(w, r, body, a.zap)
	if !ok {
		return // error is already written
	}
	enabledSpecificNodes, err := a.nodesStorage.EnabledSpecificNodes()
	if err != nil {
		a.zap.Error("[API] Failed to fetch specific nodes from storage",
			zap.Error(err),
			zap.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to fetch specific nodes from storage: %v", err), http.StatusInternalServerError)
		return
	}

	if !specificNodeFoundInStorage(enabledSpecificNodes, statement) {
		a.zap.Info("[API] Received a statements from the private node but it's not being monitored by the nodemon",
			zap.String("node", statement.Node),
			zap.String("request-id", middleware.GetReqID(r.Context())),
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
		a.zap.Error("Failed to marshal statements",
			zap.Error(err),
			zap.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to marshal statements to JSON: %v", err), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)

	sumhash := cleanCRLF(statehash.SumHash.Hex())
	a.zap.Sugar().Infof("Statement for node %s has been received, height %d, statehash %s\n",
		statement.Node, statement.Height, sumhash)
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
		a.zap.Error("[API] Failed to fetch regular nodes from storage",
			zap.Error(err),
			zap.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to complete request: %v", err), http.StatusInternalServerError)
		return
	}
	specificNodes, err := a.nodesStorage.Nodes(true)
	if err != nil {
		a.zap.Error("[API] Failed to fetch specific nodes from storage",
			zap.Error(err),
			zap.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to complete request: %v", err), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(nodesResponse{Regular: regularNodes, Specific: specificNodes})
	if err != nil {
		a.zap.Error("[API] Failed to marshal nodes",
			zap.Error(err),
			zap.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to marshal nodes to JSON: %v", err), http.StatusInternalServerError)
		return
	}
}

func (a *API) enabled(w http.ResponseWriter, r *http.Request) {
	enabledRegularNodes, err := a.nodesStorage.EnabledNodes()
	if err != nil {
		a.zap.Error("[API] Failed to fetch enabled regular nodes from storage",
			zap.Error(err),
			zap.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to complete request: %v", err), http.StatusInternalServerError)
		return
	}
	enabledSpecificNodes, err := a.nodesStorage.EnabledSpecificNodes()
	if err != nil {
		a.zap.Error("[API] Failed to fetch enabled specific nodes from storage",
			zap.Error(err),
			zap.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to complete request: %v", err), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(nodesResponse{Regular: enabledRegularNodes, Specific: enabledSpecificNodes})
	if err != nil {
		a.zap.Error("[API] Failed to marshal nodes",
			zap.Error(err),
			zap.String("request-id", middleware.GetReqID(r.Context())),
		)
		http.Error(w, fmt.Sprintf("Failed to marshal nodes to JSON: %v", err), http.StatusInternalServerError)
		return
	}
}
