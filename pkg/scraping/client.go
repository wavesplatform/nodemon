package scraping

import (
	"context"
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

func (c *nodeClient) height(ctx context.Context) (uint64, error) {
	height, _, err := c.cl.Blocks.Height(ctx)
	if err != nil {
		nodeURL := c.cl.GetOptions().BaseUrl
		c.zap.Error("Height request failed", zap.String("node", nodeURL), zap.Error(err))
		return 0, err
	}
	return height.Height, nil
}

func (c *nodeClient) stateHash(ctx context.Context, height uint64) (*proto.StateHash, error) {
	sh, _, err := c.cl.Debug.StateHash(ctx, height)
	if err != nil {
		nodeURL := c.cl.GetOptions().BaseUrl
		c.zap.Error("State hash request failed", zap.String("node", nodeURL), zap.Error(err))
		return nil, err
	}
	return sh, nil
}

func (c *nodeClient) blockHeader(ctx context.Context, height uint64) (*client.Headers, error) {
	headers, _, err := c.cl.Blocks.HeadersAt(ctx, height)
	if err != nil {
		nodeURL := c.cl.GetOptions().BaseUrl
		c.zap.Error("headers at request failed",
			zap.String("node", nodeURL), zap.Uint64("height", height), zap.Error(err),
		)
		return nil, err
	}
	return headers, nil
}

func (c *nodeClient) baseTarget(ctx context.Context, height uint64) (uint64, error) {
	header, err := c.blockHeader(ctx, height)
	if err != nil {
		return 0, err
	}
	return header.NxtConsensus.BaseTarget, nil
}
