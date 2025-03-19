package l2

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	urlPackage "net/url"
	"strconv"
	"strings"
	"time"

	"nodemon/pkg/analysis/storage"
	"nodemon/pkg/entities"
	"nodemon/pkg/tools"

	"go.uber.org/zap"
)

const (
	heightCollectorTimeout         = 1 * time.Minute
	l2NodesSameHeightTimerDuration = 5 * time.Minute
)
const l2HeightRequestTimeout = 5 * time.Second

type Response struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      string `json:"id"`
	Result  string `json:"result"`
}

func hexStringToInt(hexString string) (int64, error) {
	// Parse the hexadecimal string to integer
	hexString = strings.TrimPrefix(hexString, "0x")
	return strconv.ParseInt(hexString, 16, 64)
}

func collectL2Height(ctx context.Context, url string, logger *zap.Logger) (uint64, bool) {
	// Validate the URL
	if _, err := urlPackage.ParseRequestURI(url); err != nil {
		logger.Error("Invalid node URL", zap.Error(err), zap.String("nodeURL", url))
		return 0, false
	}

	requestBody, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "eth_blockNumber",
		"params":  []interface{}{},
	})
	if err != nil {
		logger.Error("Failed to build a request body for l2 node", zap.Error(err), zap.String("nodeURL", url))
		return 0, false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(requestBody))
	if err != nil {
		logger.Error("Failed to create a HTTP request to l2 node", zap.Error(err), zap.String("nodeURL", url))
		return 0, false
	}
	req.Header.Set("Content-Type", "application/json") // Set the content type to JSON

	httpClient := http.Client{Timeout: l2HeightRequestTimeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error("Failed to send a request to l2 node", zap.Error(err), zap.String("nodeURL", url))
		return 0, false
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response body from l2 node", zap.Error(err), zap.String("nodeURL", url))
		return 0, false
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		logger.Error("Failed unmarshalling response", zap.Error(err), zap.String("nodeURL", url))
		return 0, false
	}

	height, err := hexStringToInt(response.Result)
	if err != nil {
		logger.Error("Failed converting hex string to integer", zap.Error(err), zap.String("nodeURL", url))
		return 0, false
	}
	if height < 0 {
		logger.Error("The received height is negative", zap.Int64("height", height), zap.String("nodeURL", url))
		return 0, false
	}
	return uint64(height), true
}

type Node struct {
	URL  string
	Name string
}

func runCollector(ctx context.Context, nodeURL string, logger *zap.Logger) <-chan uint64 {
	collectAndSend := func(heightCh chan<- uint64) {
		height, ok := collectL2Height(ctx, nodeURL, logger)
		if !ok {
			return // failed to collect height
		}
		logger.Info("L2 height collected", zap.Uint64("height", height), zap.String("nodeURL", nodeURL))
		select {
		case heightCh <- height:
		case <-ctx.Done():
		}
	}
	collector := func(heightCh chan<- uint64) {
		defer close(heightCh)
		ticker := time.NewTicker(heightCollectorTimeout)
		defer ticker.Stop()
		collectAndSend(heightCh) // collect height just after starting
		for {
			select {
			case <-ticker.C:
				collectAndSend(heightCh) // collect height every minute
			case <-ctx.Done():
				return
			}
		}
	}
	heightCh := make(chan uint64)
	go collector(heightCh)
	return heightCh
}

func vacuumAlerts(
	ctx context.Context,
	alertsL2 chan<- entities.Alert,
	s *storage.AlertsStorage,
) {
	vacuumedAlerts := s.Vacuum()
	for _, alert := range vacuumedAlerts {
		fixedAlert := &entities.AlertFixed{
			Timestamp: time.Now().Unix(),
			Fixed:     alert,
		}
		select {
		case alertsL2 <- fixedAlert:
		case <-ctx.Done():
			return
		}
	}
}

// defaultAlertVacuumQuota is a default value for the alerts storage vacuum quota.
// It is calculated as the number of vacuum stages required to vacuum an alert.
// The formula is: l2NodesSameHeightTimerDuration / heightCollectorTimeout + 1 + 4, where:
// - l2NodesSameHeightTimerDuration is the duration of the timer that triggers the alert about the same height of an L2,
// - heightCollectorTimeout is the timeout of the height collector,
// - 1 is added to compensate the first vacuum stage,
// - 2 is added to survive the vacuum stage after the last alert.
const defaultAlertVacuumQuota = int(l2NodesSameHeightTimerDuration/heightCollectorTimeout) + 1 + 2

func analyzerLoop(
	ctx context.Context,
	zap *zap.Logger,
	node Node,
	alertsL2 chan<- entities.Alert,
	heightCh <-chan uint64,
) {
	defer close(alertsL2)
	alertTimer := time.NewTimer(l2NodesSameHeightTimerDuration)
	defer alertTimer.Stop()
	s := storage.NewAlertsStorage(zap, storage.AlertVacuumQuota(defaultAlertVacuumQuota))

	var lastHeight uint64
	for {
		select {
		case height, ok := <-heightCh:
			if !ok { // chan is closed, same as ctx.Done()
				return
			}
			if height != lastHeight {
				lastHeight = height
				alertTimer.Reset(l2NodesSameHeightTimerDuration)
			}
		case <-alertTimer.C:
			zap.Sugar().Infof("Alert: Height of an l2 node %s didn't change in 5 minutes, node url:%s",
				node.Name, node.URL,
			)
			alert := entities.NewL2StuckAlert(time.Now().Unix(), lastHeight, node.Name)
			sendNow := s.PutAlert(alert)
			if sendNow {
				select {
				case alertsL2 <- alert:
				case <-ctx.Done():
					return
				}
			}
			alertTimer.Reset(l2NodesSameHeightTimerDuration)
		case <-ctx.Done():
			return
		}
		vacuumAlerts(ctx, alertsL2, s)
	}
}

func RunL2Analyzer(ctx context.Context, zap *zap.Logger, node Node) <-chan entities.Alert {
	heightCh := runCollector(ctx, node.URL, zap)
	alertsL2 := make(chan entities.Alert)
	go analyzerLoop(ctx, zap, node, alertsL2, heightCh)
	return alertsL2
}

func RunL2Analyzers(
	ctx context.Context,
	zap *zap.Logger,
	nodes []Node,
) <-chan entities.Alert {
	// intentionally not using tools.FanInSeqCtx to avoid context propagation
	return tools.FanInSeq(func(yield func(<-chan entities.Alert) bool) {
		if len(nodes) == 0 {
			zap.Warn("No l2 nodes to analyze")
			return
		}
		for _, node := range nodes {
			alertsL2 := RunL2Analyzer(ctx, zap, node)
			if !yield(alertsL2) {
				return
			}
		}
	})
}
