package api_test

import (
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"

	"nodemon/pkg/api"
)

func TestAPIServeErrChannel(t *testing.T) {
	t.Parallel()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	a, err := api.NewAPI(":0", nil, nil, 0, logger, nil, false)
	require.NoError(t, err)
	ch := a.ServeErr()
	require.NotNil(t, ch)
}
