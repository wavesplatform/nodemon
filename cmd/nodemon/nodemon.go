package main

import (
	"context"
	stderrs "errors"
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"nodemon/pkg/messaging"

	"go.uber.org/zap"

	"nodemon/internal"
	"nodemon/pkg/analysis"
	"nodemon/pkg/analysis/criteria"
	"nodemon/pkg/analysis/l2"
	"nodemon/pkg/api"
	"nodemon/pkg/clients"
	"nodemon/pkg/entities"
	"nodemon/pkg/messaging/pair"
	"nodemon/pkg/messaging/pubsub"
	"nodemon/pkg/scraping"
	"nodemon/pkg/storing/events"
	"nodemon/pkg/storing/nodes"
	"nodemon/pkg/storing/specific"
	"nodemon/pkg/tools"

	"github.com/pkg/errors"
)

const (
	defaultNetworkTimeout    = 15 * time.Second
	defaultPollingInterval   = 60 * time.Second
	defaultRetentionDuration = 12 * time.Hour
	defaultAPIReadTimeout    = 30 * time.Second

	natsMaxPayloadSize            int32 = 1024 * 1024 // 1 MB
	natsConnectionsTimeoutDefault       = 5 * time.Second
)

var (
	errInvalidParameters = stderrs.New("invalid parameters")
)

func main() {
	const (
		contextCanceledExitCode   = 130
		invalidParametersExitCode = 2
	)
	if err := run(); err != nil {
		switch {
		case stderrs.Is(err, context.Canceled):
			os.Exit(contextCanceledExitCode)
		case stderrs.Is(err, errInvalidParameters):
			os.Exit(invalidParametersExitCode)
		default:
			log.Fatal(err)
		}
	}
}

type nodemonL2Config struct {
	L2nodeURLs  string
	L2nodeNames string
}

func newNodemonL2Config() *nodemonL2Config {
	c := new(nodemonL2Config)
	tools.StringVarFlagWithEnv(&c.L2nodeURLs, "l2-urls", "",
		"List of Waves L2 Blockchain nodes URLs to monitor. Provide space separated list of REST API URLs here.")
	tools.StringVarFlagWithEnv(&c.L2nodeNames, "l2-names", "",
		"List of Waves L2 Blockchain nodes names to monitor. Provide space separated list of nodes names here. "+
			"If not provided, URLs will be used as names.")
	return c
}

func (c *nodemonL2Config) present() bool {
	return c.L2nodeURLs != ""
}

func (c *nodemonL2Config) Nodes() []l2.Node {
	if !c.present() {
		return nil
	}
	urls := strings.Fields(c.L2nodeURLs)
	names := strings.Fields(c.L2nodeNames)
	out := make([]l2.Node, 0, len(urls))
	for i, nodeURL := range urls {
		name := nodeURL
		if len(names) == len(urls) {
			name = names[i]
		}
		out = append(out, l2.Node{URL: nodeURL, Name: name})
	}
	return out
}

func validateURLs(urls []string) error {
	var errs []error
	for i, nodeURL := range urls {
		if _, err := url.ParseRequestURI(nodeURL); err != nil {
			errs = append(errs, errors.Wrapf(err, "%d-th URL '%s' is invalid", i+1, nodeURL))
		}
	}
	return stderrs.Join(errs...)
}

func (c *nodemonL2Config) validate(logger *zap.Logger) error {
	if !c.present() {
		return nil
	}
	var (
		urls  = strings.Fields(c.L2nodeURLs)
		names = strings.Fields(c.L2nodeNames)
	)
	if len(names) != 0 && len(urls) != len(names) {
		logger.Sugar().Errorf("L2 node URLs and names should have the same length: names=%d, URLs=%d",
			len(names), len(urls),
		)
		return errInvalidParameters
	}
	if err := validateURLs(urls); err != nil {
		logger.Error("Invalid L2 node URL", zap.Error(err))
		return stderrs.Join(errInvalidParameters, err)
	}
	for i, nodeName := range names {
		if nodeName == "" {
			logger.Sugar().Errorf("%d-th node name is empty", i+1)
			return errInvalidParameters
		}
	}
	return nil
}

type nodemonVaultConfig struct {
	address    string
	user       string
	password   string
	mountPath  string
	secretPath string
}

type natsOptionalConfig struct {
	enable                     bool
	serverAddress              string
	maxPayload                 uint64
	readyForConnectionsTimeout time.Duration
}

func newNodemonVaultConfig() *nodemonVaultConfig {
	c := new(nodemonVaultConfig)
	tools.StringVarFlagWithEnv(&c.address, "vault-address", "", "Vault server address.")
	tools.StringVarFlagWithEnv(&c.user, "vault-user", "", "Vault user.")
	tools.StringVarFlagWithEnv(&c.password, "vault-password", "", "Vault user's password.")
	tools.StringVarFlagWithEnv(&c.mountPath, "vault-mount-path", "gonodemonitoring",
		"Vault mount path for nodemon nodes storage.")
	tools.StringVarFlagWithEnv(&c.secretPath, "vault-secret-path", "",
		"Vault secret where nodemon nodes will be saved")
	return c
}

func newNatsOptionalConfig() *natsOptionalConfig {
	c := new(natsOptionalConfig)
	tools.BoolVarFlagWithEnv(&c.enable, "nats-server-enable", false, "Enable NATS embedded server")
	tools.StringVarFlagWithEnv(&c.serverAddress, "nats-server-address",
		"127.0.0.1:4222", "NATS embedded server address in form 'host:port'")
	tools.Uint64VarFlagWithEnv(&c.maxPayload, "nats-server-max-payload", uint64(natsMaxPayloadSize),
		"Max server payload size in bytes")
	tools.DurationVarFlagWithEnv(&c.readyForConnectionsTimeout, "nats-server-ready-timeout",
		natsConnectionsTimeoutDefault, "NATS server 'ready for connections' timeout")
	return c
}

func (n *nodemonVaultConfig) present() bool {
	return n.address != ""
}

func (n *nodemonVaultConfig) validate(logger *zap.Logger) error {
	if n.address == "" { // skip further validation
		return nil
	}
	if n.user == "" {
		logger.Error("Empty vault user.")
		return errInvalidParameters
	}
	if n.password == "" {
		logger.Error("Empty vault password.")
		return errInvalidParameters
	}
	if len(n.mountPath) == 0 {
		logger.Error("Empty vault mount path")
		return errInvalidParameters
	}
	if len(n.secretPath) == 0 {
		logger.Error("Empty vault secret path")
		return errInvalidParameters
	}
	return nil
}

type nodemonConfig struct {
	storage             string
	nodes               string
	L2nodeName          string
	L2nodeURL           string
	bindAddress         string
	interval            time.Duration
	timeout             time.Duration
	natsMessagingURL    string
	natsPairTelegram    bool
	natsPairDiscord     bool
	natsTimeout         time.Duration
	retention           time.Duration
	apiReadTimeout      time.Duration
	baseTargetThreshold uint64
	logLevel            string
	development         bool
	vault               *nodemonVaultConfig
	l2                  *nodemonL2Config
	scheme              string
	natsOptionalConfig  *natsOptionalConfig
}

func newNodemonConfig() *nodemonConfig {
	c := new(nodemonConfig)
	tools.StringVarFlagWithEnv(&c.storage, "storage",
		".nodes.json", "Path to storage. Default value is \".nodes.json\"")
	tools.StringVarFlagWithEnv(&c.nodes, "nodes", "",
		"Initial list of Waves Blockchain nodes to monitor. Provide space separated list of REST API URLs here.")
	tools.StringVarFlagWithEnv(&c.bindAddress, "bind", ":8080",
		"Local network address to bind the HTTP API of the service on. Default value is \":8080\".")
	tools.DurationVarFlagWithEnv(&c.interval, "interval",
		defaultPollingInterval, "Polling interval, seconds. Default value is 60")
	tools.DurationVarFlagWithEnv(&c.timeout, "timeout",
		defaultNetworkTimeout, "Network timeout, seconds. Default value is 15")
	tools.Uint64VarFlagWithEnv(&c.baseTargetThreshold, "base-target-threshold",
		0, "Base target threshold. Must be specified")
	tools.StringVarFlagWithEnv(&c.natsMessagingURL, "nats-msg-url",
		"nats://127.0.0.1:4222", "Nats URL for messaging")
	tools.DurationVarFlagWithEnv(&c.natsTimeout, "nats-connection-timeout",
		natsConnectionsTimeoutDefault, "NATS connection to server timeout")
	tools.DurationVarFlagWithEnv(&c.retention, "retention", defaultRetentionDuration,
		"Events retention duration. Default value is 12h")
	tools.DurationVarFlagWithEnv(&c.apiReadTimeout, "api-read-timeout", defaultAPIReadTimeout,
		"HTTP API read timeout. Default value is 30s.")
	tools.BoolVarFlagWithEnv(&c.development, "development", false, "Development mode.")
	tools.BoolVarFlagWithEnv(&c.natsPairDiscord, "bot-requests-discord", false, "Should let discord bot send commands?")
	tools.BoolVarFlagWithEnv(&c.natsPairTelegram, "bot-requests-telegram", true, "Should let telegram bot send commands?")
	tools.StringVarFlagWithEnv(&c.logLevel, "log-level", "INFO",
		"Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
	tools.StringVarFlagWithEnv(&c.scheme, "scheme",
		"", "Blockchain scheme i.e. mainnet, testnet, stagenet")
	c.vault = newNodemonVaultConfig()
	c.l2 = newNodemonL2Config()
	c.natsOptionalConfig = newNatsOptionalConfig()
	return c
}

func (c *nodemonConfig) validate(logger *zap.Logger) error {
	if !c.vault.present() {
		if len(c.storage) == 0 || len(strings.Fields(c.storage)) > 1 {
			logger.Error("Invalid storage path", zap.String("path", c.storage))
			return errInvalidParameters
		}
	}
	if c.interval <= 0 {
		logger.Error("Invalid polling interval", zap.Stringer("interval", c.interval))
		return errInvalidParameters
	}
	if c.scheme == "" {
		logger.Error("Empty blockchain scheme", zap.String("scheme", c.scheme))
		return errInvalidParameters
	}
	if c.timeout <= 0 {
		logger.Error("Invalid network timeout", zap.Stringer("timeout", c.timeout))
		return errInvalidParameters
	}
	if c.retention <= 0 {
		logger.Error("Invalid retention duration", zap.Stringer("retention", c.retention))
		return errInvalidParameters
	}
	if c.baseTargetThreshold == 0 {
		logger.Error("Invalid base target threshold", zap.Uint64("threshold", c.baseTargetThreshold))
		return errInvalidParameters
	}
	return stderrs.Join(c.vault.validate(logger), c.l2.validate(logger))
}

func (c *nodemonConfig) runDiscordPairServer() bool { return c.natsPairDiscord }

func (c *nodemonConfig) runTelegramPairServer() bool { return c.natsPairTelegram }

func (c *nodemonConfig) runAnalyzers(
	ctx context.Context,
	cfg *nodemonConfig,
	es *events.Storage,
	logger *zap.Logger,
	notifications <-chan entities.NodesGatheringNotification,
) <-chan entities.Alert {
	alerts := runAnalyzer(cfg, es, logger, notifications)
	// L2 analyzer will only be run if the arguments are set
	if cfg.l2.present() {
		alertL2 := l2.RunL2Analyzers(ctx, logger, cfg.l2.Nodes())
		// merge alerts from different analyzers, wait till both are done
		mergedAlerts := tools.FanIn(alerts, alertL2)
		alerts = mergedAlerts
	}
	return alerts
}

func run() error {
	cfg := newNodemonConfig()
	flag.Parse()

	logger, atom, err := tools.SetupZapLogger(cfg.logLevel, cfg.development)
	if err != nil {
		log.Printf("Failed to setup zap logger: %v", err)
		return stderrs.Join(errInvalidParameters, err)
	}
	defer func(zap *zap.Logger) {
		if syncErr := zap.Sync(); syncErr != nil {
			log.Println(syncErr)
		}
	}(logger)

	logger.Info("Starting nodemon", zap.String("version", internal.Version()))

	if validateErr := cfg.validate(logger); validateErr != nil {
		return validateErr
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()

	ns, es, err := initializeStorages(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer closeStorages(ns, es, logger)

	scraper, err := scraping.NewScraper(ns, es, cfg.interval, cfg.timeout, logger)
	if err != nil {
		logger.Error("failed to initialize scraper", zap.Error(err))
		return err
	}

	privateNodesHandler, err := specific.NewPrivateNodesHandlerWithUnreachableInitialState(es, ns, logger)
	if err != nil {
		logger.Error("failed to create private nodes handler with unreachable initial state", zap.Error(err))
		return err
	}

	shutdownFn, serviceErr := startServices(ctx, cfg, ns, es, scraper, privateNodesHandler, atom, logger)
	if serviceErr != nil {
		return serviceErr
	}
	defer shutdownFn()

	<-ctx.Done()
	logger.Info("Shutting down")
	return nil
}

func initializeStorages(ctx context.Context, cfg *nodemonConfig,
	logger *zap.Logger) (nodes.Storage, *events.Storage, error) {
	ns, err := createNodesStorage(ctx, cfg, logger)
	if err != nil {
		logger.Error("failed to initialize nodes storage", zap.Error(err))
		return nil, nil, err
	}

	es, err := events.NewStorage(cfg.retention, logger)
	if err != nil {
		logger.Error("failed to initialize events storage", zap.Error(err))
		return nil, nil, err
	}

	return ns, es, nil
}

func closeStorages(ns nodes.Storage, es *events.Storage, logger *zap.Logger) {
	if err := ns.Close(); err != nil {
		logger.Error("failed to close nodes storage", zap.Error(err))
	}
	if err := es.Close(); err != nil {
		logger.Error("failed to close events storage", zap.Error(err))
	}
}

type shutdownFunc func()

func startServices( //nolint:nonamedreturns // needs in defer
	ctx context.Context,
	cfg *nodemonConfig,
	ns nodes.Storage,
	es *events.Storage,
	scraper *scraping.Scraper,
	privateNodesHandler *specific.PrivateNodesHandler,
	atom *zap.AtomicLevel,
	logger *zap.Logger,
) (_ shutdownFunc, runErr error) {
	notifications := scraper.Start(ctx)
	notifications = privateNodesHandler.Run(notifications) // wraps scraper's notifications

	pew := privateNodesHandler.PrivateNodesEventsWriter()
	a, err := api.NewAPI(cfg.bindAddress, ns, es, cfg.apiReadTimeout, logger, pew, atom, cfg.development)
	if err != nil {
		logger.Error("failed to initialize API", zap.Error(err))
		return nil, err
	}

	if apiErr := a.Start(); apiErr != nil {
		logger.Error("failed to start API", zap.Error(apiErr))
		return nil, apiErr
	}
	defer func() {
		if runErr != nil { // stop API if any error occurred
			a.Shutdown()
		}
	}()

	shutdownFn := a.Shutdown
	if nCfg := cfg.natsOptionalConfig; nCfg.enable {
		natsShutdown, nErr := messaging.RunNatsMessagingServer(
			nCfg.serverAddress,
			logger,
			nCfg.maxPayload,
			nCfg.readyForConnectionsTimeout,
		)
		if nErr != nil {
			logger.Error("failed to start NATS server", zap.Error(nErr))
			return nil, nErr
		}
		defer func() {
			if runErr != nil { // stop NATS server if any error occurred
				natsShutdown()
			}
		}()
		shutdownFn = chainShutdownFuncs(shutdownFn, natsShutdown) // add NATS server shutdown to the chain
	}

	alerts := cfg.runAnalyzers(ctx, cfg, es, logger, notifications)

	runMessagingServices(ctx, cfg, alerts, logger, ns, es, pew)

	return shutdownFn, err
}

func chainShutdownFuncs(fns ...shutdownFunc) shutdownFunc {
	return func() {
		for _, fn := range fns {
			fn()
		}
	}
}

func createNodesStorage(ctx context.Context, cfg *nodemonConfig, logger *zap.Logger) (nodes.Storage, error) {
	var (
		ns  nodes.Storage
		err error
	)
	if cfg.vault.present() {
		cl, clErr := clients.NewVaultSimpleClient(ctx, logger, cfg.vault.address, cfg.vault.user, cfg.vault.password)
		if clErr != nil {
			logger.Error("failed to create vault client", zap.Error(clErr))
			return nil, clErr
		}
		ns, err = nodes.NewJSONVaultStorage(
			ctx,
			cl,
			cfg.vault.mountPath,
			cfg.vault.secretPath,
			strings.Fields(cfg.nodes),
			logger,
		)
	} else {
		ns, err = nodes.NewJSONFileStorage(cfg.storage, strings.Fields(cfg.nodes), logger)
	}
	if err != nil {
		logger.Error("failed to initialize nodes storage", zap.Error(err))
		return nil, err
	}
	return ns, nil
}

func runAnalyzer(
	cfg *nodemonConfig,
	es *events.Storage,
	zap *zap.Logger,
	notifications <-chan entities.NodesGatheringNotification,
) <-chan entities.Alert {
	opts := &analysis.AnalyzerOptions{
		BaseTargetCriterionOpts: &criteria.BaseTargetCriterionOptions{Threshold: cfg.baseTargetThreshold},
	}
	analyzer := analysis.NewAnalyzer(es, opts, zap)
	alerts := analyzer.Start(notifications)
	return alerts
}

func runMessagingServices(
	ctx context.Context,
	cfg *nodemonConfig,
	alerts <-chan entities.Alert,
	logger *zap.Logger,
	ns nodes.Storage,
	es *events.Storage,
	pew specific.PrivateNodesEventsWriter,
) {
	go func() {
		pubSubErr := pubsub.StartPubMessagingServer(ctx, cfg.natsMessagingURL, alerts, logger, cfg.scheme)
		if pubSubErr != nil {
			logger.Fatal("failed to start pub messaging server", zap.Error(pubSubErr))
		}
	}()

	if cfg.runTelegramPairServer() {
		go func() {
			telegramTopic := messaging.TelegramBotRequestsTopic(cfg.scheme)
			pairErr := pair.StartPairMessagingServer(ctx, cfg.natsMessagingURL, ns, es, pew, logger, telegramTopic)
			if pairErr != nil {
				logger.Fatal("failed to start pair messaging server", zap.Error(pairErr))
			}
		}()
	}

	if cfg.runDiscordPairServer() {
		go func() {
			discordTopic := messaging.DiscordBotRequestsTopic(cfg.scheme)
			pairErr := pair.StartPairMessagingServer(ctx, cfg.natsMessagingURL, ns, es, pew, logger, discordTopic)
			if pairErr != nil {
				logger.Fatal("failed to start pair messaging server", zap.Error(pairErr))
			}
		}()
	}
}
