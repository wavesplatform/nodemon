package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"

	"nodemon/pkg/analysis"
	"nodemon/pkg/analysis/criteria"
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
)

const (
	defaultNetworkTimeout    = 15 * time.Second
	defaultPollingInterval   = 60 * time.Second
	defaultRetentionDuration = 12 * time.Hour
	defaultAPIReadTimeout    = 30 * time.Second
)

var (
	errInvalidParameters = errors.New("invalid parameters")
)

func main() {
	const (
		contextCanceledExitCode   = 130
		invalidParametersExitCode = 2
	)
	if err := run(); err != nil {
		switch {
		case errors.Is(err, context.Canceled):
			os.Exit(contextCanceledExitCode)
		case errors.Is(err, errInvalidParameters):
			os.Exit(invalidParametersExitCode)
		default:
			log.Fatal(err)
		}
	}
}

type nodemonVaultConfig struct {
	address    string
	user       string
	password   string
	mountPath  string
	secretPath string
}

func (n *nodemonVaultConfig) present() bool {
	return n.address != ""
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
	storage                string
	nodes                  string
	bindAddress            string
	interval               time.Duration
	timeout                time.Duration
	nanomsgPubSubURL       string
	nanomsgPairTelegramURL string
	nanomsgPairDiscordURL  string
	retention              time.Duration
	apiReadTimeout         time.Duration
	baseTargetThreshold    int
	logLevel               string
	development            bool
	vault                  *nodemonVaultConfig
}

func newNodemonConfig() *nodemonConfig {
	c := new(nodemonConfig)
	tools.StringVarFlagWithEnv(&c.storage, "storage",
		".nodes.json", "Path to storage. Default value is \".nodes.json\"")
	tools.StringVarFlagWithEnv(&c.nodes, "nodes", "",
		"Initial list of Waves Blockchain nodes to monitor. Provide comma separated list of REST API URLs here.")
	tools.StringVarFlagWithEnv(&c.bindAddress, "bind", ":8080",
		"Local network address to bind the HTTP API of the service on. Default value is \":8080\".")
	tools.DurationVarFlagWithEnv(&c.interval, "interval",
		defaultPollingInterval, "Polling interval, seconds. Default value is 60")
	tools.DurationVarFlagWithEnv(&c.timeout, "timeout",
		defaultNetworkTimeout, "Network timeout, seconds. Default value is 15")
	tools.IntVarFlagWithEnv(&c.baseTargetThreshold, "base-target-threshold",
		0, "Base target threshold. Must be specified")
	tools.StringVarFlagWithEnv(&c.nanomsgPubSubURL, "nano-msg-pubsub-url",
		"ipc:///tmp/nano-msg-pubsub.ipc", "Nanomsg IPC URL for pubsub socket")
	tools.StringVarFlagWithEnv(&c.nanomsgPairTelegramURL, "nano-msg-pair-telegram-url",
		"", "Nanomsg IPC URL for pair socket")
	tools.StringVarFlagWithEnv(&c.nanomsgPairDiscordURL, "nano-msg-pair-discord-url",
		"", "Nanomsg IPC URL for pair socket")
	tools.DurationVarFlagWithEnv(&c.retention, "retention", defaultRetentionDuration,
		"Events retention duration. Default value is 12h")
	tools.DurationVarFlagWithEnv(&c.apiReadTimeout, "api-read-timeout", defaultAPIReadTimeout,
		"HTTP API read timeout. Default value is 30s.")
	tools.BoolVarFlagWithEnv(&c.development, "development", false, "Development mode.")
	tools.StringVarFlagWithEnv(&c.logLevel, "log-level", "INFO",
		"Logging level. Supported levels: DEBUG, INFO, WARN, ERROR, FATAL. Default logging level INFO.")
	c.vault = newNodemonVaultConfig()
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
	if c.timeout <= 0 {
		logger.Error("Invalid network timeout", zap.Stringer("timeout", c.timeout))
		return errInvalidParameters
	}
	if c.retention <= 0 {
		logger.Error("Invalid retention duration", zap.Stringer("retention", c.retention))
		return errInvalidParameters
	}
	if c.baseTargetThreshold == 0 {
		logger.Error("Invalid base target threshold", zap.Int("threshold", c.baseTargetThreshold))
		return errInvalidParameters
	}
	return c.vault.validate(logger)
}

func (c *nodemonConfig) runDiscordPairServer() bool { return c.nanomsgPairDiscordURL != "" }

func (c *nodemonConfig) runTelegramPairServer() bool { return c.nanomsgPairTelegramURL != "" }

func run() error {
	cfg := newNodemonConfig()
	flag.Parse()

	logger, atom, err := tools.SetupZapLogger(cfg.logLevel, cfg.development)
	if err != nil {
		log.Printf("Failed to setup zap logger: %v", err)
		return errInvalidParameters
	}
	defer func() {
		_ = logger.Sync() // intentionally ignore error
	}()

	if validateErr := cfg.validate(logger); validateErr != nil {
		return validateErr
	}

	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()

	ns, err := createNodesStorage(ctx, cfg, logger)
	if err != nil {
		return err
	}
	defer func(cs nodes.Storage) {
		if closeErr := cs.Close(); closeErr != nil {
			logger.Error("failed to close nodes storage", zap.Error(closeErr))
		}
	}(ns)

	es, err := events.NewStorage(cfg.retention, logger)
	if err != nil {
		logger.Error("failed to initialize events storage", zap.Error(err))
		return err
	}
	defer func(es *events.Storage) {
		if closeErr := es.Close(); closeErr != nil {
			logger.Error("failed to close events storage", zap.Error(closeErr))
		}
	}(es)

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

	notifications := scraper.Start(ctx)
	notifications = privateNodesHandler.Run(notifications) // wraps scrapper's notification with private nodes handler

	a, err := api.NewAPI(cfg.bindAddress, ns, es, cfg.apiReadTimeout, logger,
		privateNodesHandler.PrivateNodesEventsWriter(), atom, cfg.development,
	)
	if err != nil {
		logger.Error("failed to initialize API", zap.Error(err))
		return err
	}
	if apiErr := a.Start(); apiErr != nil {
		logger.Error("failed to start API", zap.Error(apiErr))
		return apiErr
	}

	alerts := runAnalyzer(cfg, es, logger, notifications)

	runMessagingServices(ctx, cfg, alerts, logger, ns, es)

	<-ctx.Done()
	a.Shutdown()
	logger.Info("shutting down")
	return nil
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
) {
	go func() {
		pubSubErr := pubsub.StartPubMessagingServer(ctx, cfg.nanomsgPubSubURL, alerts, logger)
		if pubSubErr != nil {
			logger.Fatal("failed to start pub messaging server", zap.Error(pubSubErr))
		}
	}()

	if cfg.runTelegramPairServer() {
		go func() {
			pairErr := pair.StartPairMessagingServer(ctx, cfg.nanomsgPairTelegramURL, ns, es, logger)
			if pairErr != nil {
				logger.Fatal("failed to start pair messaging server", zap.Error(pairErr))
			}
		}()
	}

	if cfg.runDiscordPairServer() {
		go func() {
			pairErr := pair.StartPairMessagingServer(ctx, cfg.nanomsgPairDiscordURL, ns, es, logger)
			if pairErr != nil {
				logger.Fatal("failed to start pair messaging server", zap.Error(pairErr))
			}
		}()
	}
}
