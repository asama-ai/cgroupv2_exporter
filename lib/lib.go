// Package lib is the host-agent absorb surface for cgroupv2_exporter.
package lib

import (
	"log/slog"
	"net/http"

	"github.com/VictoriaMetrics/metrics"
	"github.com/asama-ai/cgroupv2_exporter/collector"
)

// NewHandler returns an http.Handler that scrapes cgroupv2_* for the given dirs.
func NewHandler(cgroupDirs []string, logger *slog.Logger) (http.Handler, error) {
	if logger == nil {
		logger = slog.Default()
	}
	cgc, err := collector.NewCgroupv2CollectorAll(cgroupDirs, logger)
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ms := metrics.NewSet()
		cgc.Scrape(ms)
		ms.WritePrometheus(w)
	}), nil
}
