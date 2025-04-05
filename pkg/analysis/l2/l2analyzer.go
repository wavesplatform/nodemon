package l2

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	urlPackage "net/url"
	"strconv"
	"strings"
	"time"

	"nodemon/internal"
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

const maxResponseSize = 1024 // 1 KB

type response struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      string `json:"id"`
	Result  string `json:"result"`
}

func hexStringToInt(hexString string) (int64, error) {
	// Parse the hexadecimal string to integer
	hexString = strings.TrimPrefix(hexString, "0x")
	return strconv.ParseInt(hexString, 16, 64)
}

func collectL2Height(ctx context.Context, url string, logger *zap.Logger) (_ uint64, runErr error) {
	// Validate the URL
	if _, err := urlPackage.ParseRequestURI(url); err != nil {
		logger.Error("Invalid node URL", zap.Error(err), zap.String("nodeURL", url))
		return 0, fmt.Errorf("invalid node URL: %w", err)
	}

	requestBody, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "eth_blockNumber",
		"params":  []interface{}{},
	})
	if err != nil {
		logger.Error("Failed to build a request body for l2 node", zap.Error(err), zap.String("nodeURL", url))
		return 0, fmt.Errorf("failed to build a request body for l2 node: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(requestBody))
	if err != nil {
		logger.Error("Failed to create a HTTP request to l2 node", zap.Error(err), zap.String("nodeURL", url))
		return 0, fmt.Errorf("failed to build a HTTP request to l2 node: %w", err)
	}
	now := time.Now().UTC()
	userAgent := fmt.Sprintf("nodemon/%s", internal.Version())
	requestID := fmt.Sprintf("%s-%s-%d", userAgent, url, now.UnixNano())
	timeSend := now.Format(http.TimeFormat)
	timeoutStr := l2HeightRequestTimeout.String()
	defer func() {
		if runErr != nil {
			runErr = fmt.Errorf("failed to collect l2 height by '%s' at '%s' with '%s' timeout: %w",
				userAgent, timeoutStr, requestID, runErr,
			)
		}
	}()
	req.Header.Set("Content-Type", "application/json") // Set the content type to JSON
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("X-Request-ID", requestID)
	req.Header.Set("X-Request-Time", timeSend)
	req.Header.Set("X-Request-Timeout", timeoutStr)

	httpClient := http.Client{Timeout: l2HeightRequestTimeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error("Failed to send a request to l2 node", zap.Error(err), zap.String("nodeURL", url))
		return 0, fmt.Errorf("failed to send request to l2 node: %w", err)
	}
	defer resp.Body.Close()

	if resp.ContentLength > maxResponseSize { // Content length can be -1 if not set, so use limited reader below
		logger.Error("Response body from l2 node is too large", zap.Int64("contentLength", resp.ContentLength),
			zap.Int("maxResponseSize", maxResponseSize), zap.String("nodeURL", url),
		)
		return 0, fmt.Errorf("response body is too large (%d bytes, max is %d)", resp.ContentLength, maxResponseSize)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize)) // Read the body, limit to maxResponseSize
	if err != nil {
		logger.Error("Failed to read response body from l2 node", zap.Error(err), zap.String("nodeURL", url))
		return 0, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logger.Error("Received non-200 response from l2 node", zap.Int("statusCode", resp.StatusCode),
			zap.String("nodeURL", url), zap.ByteString("responseBody", body),
		)
		return 0, fmt.Errorf("received non-200 response, body=%q", body)
	}

	var res response
	err = json.Unmarshal(body, &res)
	if err != nil {
		logger.Error("Failed unmarshalling response", zap.Error(err),
			zap.String("nodeURL", url), zap.ByteString("responseBody", body),
		)
		return 0, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	height, err := hexStringToInt(res.Result)
	if err != nil {
		logger.Error("Failed converting hex string to integer", zap.Error(err),
			zap.String("nodeURL", url), zap.String("resultHeight", res.Result),
		)
		return 0, fmt.Errorf("failed to convert hex string heightto integer: %w", err)
	}
	if height < 0 {
		logger.Error("The received height is negative", zap.Int64("height", height), zap.String("nodeURL", url))
		return 0, fmt.Errorf("the received height is negative: %d", height)
	}
	return uint64(height), nil
}

type Node struct {
	URL  string
	Name string
}

type heightCollectorResponse struct {
	height uint64
	err    error
}

func runCollector(ctx context.Context, nodeURL string, logger *zap.Logger) <-chan heightCollectorResponse {
	collectAndSend := func(heightCh chan<- heightCollectorResponse) {
		height, err := collectL2Height(ctx, nodeURL, logger)
		select {
		case heightCh <- heightCollectorResponse{height: height, err: err}:
		case <-ctx.Done():
		}
		if err != nil {
			return // failed to collect height
		}
		logger.Info("L2 height collected", zap.Uint64("height", height), zap.String("nodeURL", nodeURL))
	}
	collector := func(heightCh chan<- heightCollectorResponse) {
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
	heightCh := make(chan heightCollectorResponse)
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

func putAndSendAlert(
	ctx context.Context,
	s *storage.AlertsStorage,
	alertsL2 chan<- entities.Alert,
	alert entities.Alert,
) {
	sendNow := s.PutAlert(alert)
	if sendNow {
		select {
		case alertsL2 <- alert:
		case <-ctx.Done():
			return
		}
	}
}

func analyzerLoop(
	ctx context.Context,
	zap *zap.Logger,
	node Node,
	alertsL2 chan<- entities.Alert,
	heightCh <-chan heightCollectorResponse,
) {
	defer close(alertsL2)
	alertTimer := time.NewTimer(l2NodesSameHeightTimerDuration)
	defer alertTimer.Stop()
	s := storage.NewAlertsStorage(zap, storage.AlertVacuumQuota(defaultAlertVacuumQuota))

	var lastHeight uint64
	for ctx.Err() == nil {
		select {
		case response, ok := <-heightCh:
			if !ok { // chan is closed, same as ctx.Done()
				return
			}
			if response.err != nil {
				errMsg := fmt.Errorf("failed to update height for L2 node %q: %w", node.Name, response.err)
				alert := entities.NewInternalErrorAlert(time.Now().Unix(), errMsg)
				putAndSendAlert(ctx, s, alertsL2, alert)
				alertTimer.Reset(l2NodesSameHeightTimerDuration) // reset timer
				continue                                         // continue to the next iteration
			}
			if height := response.height; height != lastHeight {
				lastHeight = height
				alertTimer.Reset(l2NodesSameHeightTimerDuration)
			}
		case <-alertTimer.C:
			zap.Sugar().Infof("Alert: Height of an l2 node %s didn't change in 5 minutes, node url:%s",
				node.Name, node.URL,
			)
			alert := entities.NewL2StuckAlert(time.Now().Unix(), lastHeight, node.Name)
			putAndSendAlert(ctx, s, alertsL2, alert)
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
