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

	"nodemon/pkg/entities"

	"go.uber.org/zap"
)

const l2NodesSameHeightTimerDuration = 5 * time.Minute
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

func collectL2Height(ctx context.Context, url string, ch chan<- uint64, logger *zap.Logger) {
	// Validate the URL
	if _, err := urlPackage.ParseRequestURI(url); err != nil {
		logger.Error("Invalid URL", zap.String("url", url), zap.Error(err))
		return
	}

	requestBody, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "eth_blockNumber",
		"params":  []interface{}{},
	})
	if err != nil {
		logger.Error("Failed to build a request body for l2 node", zap.Error(err))
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(requestBody))
	if err != nil {
		logger.Error("Failed to create a HTTP request to l2 node", zap.Error(err))
		return
	}
	httpClient := http.Client{Timeout: l2HeightRequestTimeout}
	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error("Failed to send a request to l2 node", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response body from l2 node", zap.Error(err))
		return
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		logger.Error("Failed unmarshalling response:", zap.Error(err))
		return
	}

	height, err := hexStringToInt(response.Result)
	if err != nil {
		logger.Error("Failed converting hex string to integer:", zap.Error(err))
		return
	}
	if height < 0 {
		logger.Error("The received height is negative, " + strconv.Itoa(int(height)))
	}
	ch <- uint64(height)
}

func RunL2Analyzer(
	ctx context.Context,
	zap *zap.Logger,
	nodeURL string,
	nodeName string,
) <-chan entities.Alert {
	alertsL2 := make(chan entities.Alert)
	heightCh := make(chan uint64)

	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				collectL2Height(ctx, nodeURL, heightCh, zap)
			case <-ctx.Done():
				return
			}
		}
	}()

	var lastHeight uint64
	alertTimer := time.NewTimer(l2NodesSameHeightTimerDuration)
	defer alertTimer.Stop()

	go func(alertsL2 chan<- entities.Alert) {
		defer close(alertsL2)
		defer close(heightCh)
		for {
			select {
			case height := <-heightCh:
				if height != lastHeight {
					lastHeight = height
					alertTimer.Reset(l2NodesSameHeightTimerDuration)
				}
			case <-alertTimer.C:

				zap.Info(fmt.Sprintf("Alert: Height of an l2 node %s didn't change in 5 minutes, node url:%s", nodeName, nodeURL))
				alertsL2 <- entities.NewL2StuckAlert(time.Now().Unix(), lastHeight, nodeName)
				alertTimer.Reset(l2NodesSameHeightTimerDuration)
			case <-ctx.Done():
				return
			}
		}
	}(alertsL2)
	return alertsL2
}
