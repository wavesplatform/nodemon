package scraping

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/wavesplatform/gowaves/pkg/client"
	"github.com/wavesplatform/gowaves/pkg/proto"
	"go.uber.org/zap"
)

type nodeClient struct {
	cl  *client.Client
	zap *zap.Logger
}

// ATTENTION! `url` MUST BE validated for proper format before passing to this function.
func newNodeClient(url string, timeout time.Duration, logger *zap.Logger) *nodeClient {
	opts := client.Options{
		BaseUrl: url,
		Client:  &http.Client{Timeout: timeout},
	}
	// The error can be safely ignored because `NewClient` function only checks the number of passed `opts`
	cl, _ := client.NewClient(opts)
	return &nodeClient{cl: cl, zap: logger}
}

func (c *nodeClient) version(ctx context.Context) (string, error) {
	version, _, err := c.cl.NodeInfo.Version(ctx)
	if err != nil {
		nodeURL := c.cl.GetOptions().BaseUrl
		c.zap.Error("Version request failed", zap.String("node", nodeURL), zap.Error(err))
		return "", err
	}
	return version, nil
}

func (c *nodeClient) height(ctx context.Context) (int, error) {
	height, _, err := c.cl.Blocks.Height(ctx)
	if err != nil {
		nodeURL := c.cl.GetOptions().BaseUrl
		c.zap.Error("Height request failed", zap.String("node", nodeURL), zap.Error(err))
		return 0, err
	}
	return int(height.Height), nil
}

func (c *nodeClient) stateHash(ctx context.Context, height int) (*proto.StateHash, error) {
	sh, _, err := c.cl.Debug.StateHash(ctx, uint64(height))
	if err != nil {
		nodeURL := c.cl.GetOptions().BaseUrl
		c.zap.Error("State hash request failed", zap.String("node", nodeURL), zap.Error(err))
		return nil, err
	}
	return sh, nil
}

func (c *nodeClient) baseTarget(ctx context.Context, height int) (int, error) {

	_, resp, err := c.cl.Blocks.HeadersAt(ctx, uint64(height))
	if err != nil {
		nodeURL := c.cl.GetOptions().BaseUrl
		c.zap.Error("headers at request failed", zap.String("node", nodeURL), zap.Error(err))
		return 0, err
	}
	b := struct {
		NxtConsensus struct {
			BaseTarget int `json:"base-target"`
		} `json:"nxt-consensus"`
	}{}

	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
		return 0, err
	}

	return b.NxtConsensus.BaseTarget, nil
}
