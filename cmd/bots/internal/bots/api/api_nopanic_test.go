package api_test

import (
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"nodemon/cmd/bots/internal/bots/api"
)

func TestBotAPIServeErrChannel(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.DiscardHandler)
	a, err := api.NewBotAPI(":0", nil, nil, 0, logger, false)
	require.NoError(t, err)
	ch := a.ServeErr()
	require.NotNil(t, ch)
}
