package tools

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func PrometheusHTTPMetricsHandler(logger promhttp.Logger) http.Handler {
	return promhttp.InstrumentMetricHandler(
		prometheus.DefaultRegisterer,
		promhttp.HandlerFor(prometheus.DefaultGatherer, promhttp.HandlerOpts{
			ErrorLog: logger,
		}),
	)
}
