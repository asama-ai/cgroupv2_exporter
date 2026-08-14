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
	return newHandler(cgroupDirs, logger, nil)
}

// NewHandlerExcept scrapes every registered collector except the named ones.
func NewHandlerExcept(cgroupDirs []string, logger *slog.Logger, exclude ...string) (http.Handler, error) {
	skip := make(map[string]struct{}, len(exclude))
	for _, e := range exclude {
		skip[e] = struct{}{}
	}
	return newHandler(cgroupDirs, logger, func(name string) bool {
		_, ok := skip[name]
		return !ok
	})
}

// NewHandlerOnly scrapes only the named collectors.
func NewHandlerOnly(cgroupDirs []string, logger *slog.Logger, names ...string) (http.Handler, error) {
	want := make(map[string]struct{}, len(names))
	for _, n := range names {
		want[n] = struct{}{}
	}
	return newHandler(cgroupDirs, logger, func(name string) bool {
		_, ok := want[name]
		return ok
	})
}

func newHandler(cgroupDirs []string, logger *slog.Logger, keep func(string) bool) (http.Handler, error) {
	if logger == nil {
		logger = slog.Default()
	}
	cgc, err := collector.NewCgroupv2CollectorSelect(cgroupDirs, logger, keep)
	if err != nil {
		return nil, err
	}
	metrics.ExposeMetadata(true)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ms := metrics.NewSet()
		cgc.Scrape(ms)
		ms.WritePrometheus(w)
	}), nil
}
