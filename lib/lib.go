// Package lib is the host-agent absorb surface for cgroupv2_exporter.
package lib

import (
	"log/slog"
	"net/http"

	"github.com/VictoriaMetrics/metrics"
	"github.com/asama-ai/cgroupv2_exporter/collector"
)

// HandlerOpts controls metric namespace and collector caching.
type HandlerOpts struct {
	Namespace string // default cgroupv2
	Uncached  bool   // skip collector cache (watch-set dirs change)
}

// NewHandler returns an http.Handler that scrapes cgroupv2_* for the given dirs.
func NewHandler(cgroupDirs []string, logger *slog.Logger) (http.Handler, error) {
	return newHandler(cgroupDirs, logger, nil, HandlerOpts{})
}

// NewHandlerExcept scrapes every registered collector except the named ones.
func NewHandlerExcept(cgroupDirs []string, logger *slog.Logger, exclude ...string) (http.Handler, error) {
	return NewHandlerExceptOpts(cgroupDirs, logger, HandlerOpts{}, exclude...)
}

// NewHandlerExceptOpts is NewHandlerExcept with namespace / uncached options.
func NewHandlerExceptOpts(cgroupDirs []string, logger *slog.Logger, opts HandlerOpts, exclude ...string) (http.Handler, error) {
	skip := make(map[string]struct{}, len(exclude))
	for _, e := range exclude {
		skip[e] = struct{}{}
	}
	return newHandler(cgroupDirs, logger, func(name string) bool {
		_, ok := skip[name]
		return !ok
	}, opts)
}

// NewHandlerOnly scrapes only the named collectors.
func NewHandlerOnly(cgroupDirs []string, logger *slog.Logger, names ...string) (http.Handler, error) {
	return NewHandlerOnlyOpts(cgroupDirs, logger, HandlerOpts{}, names...)
}

// NewHandlerOnlyOpts is NewHandlerOnly with namespace / uncached options.
func NewHandlerOnlyOpts(cgroupDirs []string, logger *slog.Logger, opts HandlerOpts, names ...string) (http.Handler, error) {
	want := make(map[string]struct{}, len(names))
	for _, n := range names {
		want[n] = struct{}{}
	}
	return newHandler(cgroupDirs, logger, func(name string) bool {
		_, ok := want[name]
		return ok
	}, opts)
}

func newHandler(cgroupDirs []string, logger *slog.Logger, keep func(string) bool, opts HandlerOpts) (http.Handler, error) {
	if logger == nil {
		logger = slog.Default()
	}
	cgc, err := collector.NewCgroupv2CollectorSelectNS(cgroupDirs, logger, keep, collector.SelectOpts{
		Namespace: opts.Namespace,
		Uncached:  opts.Uncached,
	})
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
