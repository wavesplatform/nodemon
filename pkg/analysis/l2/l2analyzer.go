package l2

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"nodemon/pkg/entities"
	"strconv"
	"time"

	"go.uber.org/zap"
)

type Response struct {
	Jsonrpc string `json:"jsonrpc"`
	ID      string `json:"id"`
	Result  string `json:"result"`
}

func hexStringToInt(hexString string) (int64, error) {
	// Parse the hexadecimal string to integer
	return strconv.ParseInt(hexString[2:], 16, 64)
}

func collectL2Height(url string, ch chan<- int64, logger *zap.Logger) {
	requestBody, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      "1",
		"method":  "eth_blockNumber",
		"params":  []interface{}{},
	})
	if err != nil {
		logger.Error("Failed to build a request body for l2 node", zap.Error(err))
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		logger.Error("Failed to send a request to l2 node", zap.Error(err))
		return
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
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
	ch <- height
}

func RunL2Analyzer(
	zap *zap.Logger,
	alerts chan<- entities.Alert,
	nodeURL string,
	nodeName string,
) {
	ch := make(chan int64)

	go func() {
		for {
			collectL2Height(nodeURL, ch, zap)
			time.Sleep(time.Minute)
		}
	}()

	var lastHeight int64
	alertTimer := time.NewTimer(5 * time.Minute)

	for {
		select {
		case height := <-ch:
			if height != lastHeight {
				lastHeight = height
				alertTimer.Reset(5 * time.Minute)
			}
		case <-alertTimer.C:
			zap.Info("Alert: Height of an l2 node didn't change in 5 minutes")
			alerts <- entities.NewL2StuckAlert(time.Now().Unix(), int(lastHeight), nodeName)
			alertTimer.Reset(5 * time.Minute)
		}
	}
}
