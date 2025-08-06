package scraping

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/wavesplatform/gowaves/pkg/client"
	"github.com/wavesplatform/gowaves/pkg/proto"

	"nodemon/pkg/tools/logging/attrs"
)

type nodeClient struct {
	cl     *client.Client
	logger *slog.Logger
}

// ATTENTION! `url` MUST BE validated for proper format before passing to this function.
func newNodeClient(url string, timeout time.Duration, logger *slog.Logger) *nodeClient {
	opts := client.Options{
		BaseUrl: url,
		Client:  &http.Client{Timeout: timeout},
	}
	// The error can be safely ignored because `NewClient` function only checks the number of passed `opts`
	cl, _ := client.NewClient(opts)
	return &nodeClient{cl: cl, logger: logger}
}

func (c *nodeClient) version(ctx context.Context) (string, error) {
	version, _, err := c.cl.NodeInfo.Version(ctx)
	if err != nil {
		nodeURL := c.cl.GetOptions().BaseUrl
		c.logger.Error("Version request failed", slog.String("node", nodeURL), attrs.Error(err))
		return "", err
	}
	return version, nil
}

func (c *nodeClient) height(ctx context.Context) (uint64, error) {
	height, _, err := c.cl.Blocks.Height(ctx)
	if err != nil {
		nodeURL := c.cl.GetOptions().BaseUrl
		c.logger.Error("Height request failed", slog.String("node", nodeURL), attrs.Error(err))
		return 0, err
	}
	return height.Height, nil
}

func (c *nodeClient) stateHash(ctx context.Context, height uint64) (*proto.StateHash, error) {
	sh, _, err := c.cl.Debug.StateHash(ctx, height)
	if err != nil {
		nodeURL := c.cl.GetOptions().BaseUrl
		c.logger.Error("State hash request failed", slog.String("node", nodeURL), attrs.Error(err))
		return nil, err
	}
	return sh, nil
}

func (c *nodeClient) blockHeader(ctx context.Context, height uint64) (*client.Headers, error) {
	headers, _, err := c.cl.Blocks.HeadersAt(ctx, height)
	if err != nil {
		nodeURL := c.cl.GetOptions().BaseUrl
		c.logger.Error("Headers at request failed",
			slog.String("node", nodeURL), slog.Uint64("height", height), attrs.Error(err),
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
