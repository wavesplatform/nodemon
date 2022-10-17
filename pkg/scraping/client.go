package scraping

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"path"
	"time"

	"github.com/wavesplatform/gowaves/pkg/client"
	"github.com/wavesplatform/gowaves/pkg/proto"
)

type nodeClient struct {
	cl *client.Client
}

// ATTENTION! `url` MUST BE validated for proper format before passing to this function.
func newNodeClient(url string, timeout time.Duration) *nodeClient {
	opts := client.Options{
		BaseUrl: url,
		Client:  &http.Client{Timeout: timeout},
	}
	// The error can be safely ignored because `NewClient` function only checks the number of passed `opts`
	cl, _ := client.NewClient(opts)
	return &nodeClient{cl: cl}
}

func (c *nodeClient) version(ctx context.Context) (string, error) {
	version, _, err := c.cl.NodeInfo.Version(ctx)
	if err != nil {
		nodeURL := c.cl.GetOptions().BaseUrl
		log.Printf("Version request to %q failed: %v", nodeURL, err)
		return "", err
	}
	return version, nil
}

func (c *nodeClient) height(ctx context.Context) (int, error) {
	height, _, err := c.cl.Blocks.Height(ctx)
	if err != nil {
		nodeURL := c.cl.GetOptions().BaseUrl
		log.Printf("Height request to %q failed: %v", nodeURL, err)
		return 0, err
	}
	return int(height.Height), nil
}

func (c *nodeClient) stateHash(ctx context.Context, height int) (*proto.StateHash, error) {
	sh, _, err := c.cl.Debug.StateHash(ctx, uint64(height))
	if err != nil {
		nodeURL := c.cl.GetOptions().BaseUrl
		log.Printf("StateHash request to %q failed: %v", nodeURL, err)
		return nil, err
	}
	return sh, nil
}

func (c *nodeClient) baseTarget(height int) (int64, error) {
	url, err := joinUrl(c.cl.GetOptions().BaseUrl, fmt.Sprintf("/blocks/headers/at/%d", height))
	if err != nil {
		return 0, err
	}

	resp, err := http.Get(url.String())
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	b := struct {
		NxtConsensus struct {
			BaseTarget int `json:"base-target"`
		} `json:"nxt-consensus"`
	}{}

	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
		return 0, err
	}

	return int64(b.NxtConsensus.BaseTarget), nil
}

func joinUrl(baseRaw string, pathRaw string) (*url.URL, error) {
	baseUrl, err := url.Parse(baseRaw)
	if err != nil {
		return nil, err
	}

	pathUrl, err := url.Parse(pathRaw)
	if err != nil {
		return nil, err
	}
	// nosemgrep: go.lang.correctness.use-filepath-join.use-filepath-join
	baseUrl.Path = path.Join(baseUrl.Path, pathUrl.Path)

	query := baseUrl.Query()
	for k := range pathUrl.Query() {
		query.Set(k, pathUrl.Query().Get(k))
	}
	baseUrl.RawQuery = query.Encode()

	return baseUrl, nil
}
