// Package lib is the host-agent absorb surface for cgroupv2_exporter.
package lib

import (
	"log/slog"
	"net/http"

	"github.com/asama-ai/cgroupv2_exporter/collector"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
)

// HandlerOpts controls metric namespace and collector caching.
type HandlerOpts struct {
	Namespace string // default cgroupv2
	Uncached  bool   // skip collector cache (watch-set dirs change)
}

// Gather returns families for every registered collector.
func Gather(cgroupDirs []string, logger *slog.Logger) ([]*dto.MetricFamily, error) {
	return gather(cgroupDirs, logger, nil, HandlerOpts{})
}

// GatherExcept returns families for every registered collector except the named ones.
func GatherExcept(cgroupDirs []string, logger *slog.Logger, exclude ...string) ([]*dto.MetricFamily, error) {
	return GatherExceptOpts(cgroupDirs, logger, HandlerOpts{}, exclude...)
}

// GatherExceptOpts is GatherExcept with namespace / uncached options.
func GatherExceptOpts(cgroupDirs []string, logger *slog.Logger, opts HandlerOpts, exclude ...string) ([]*dto.MetricFamily, error) {
	skip := make(map[string]struct{}, len(exclude))
	for _, e := range exclude {
		skip[e] = struct{}{}
	}
	return gather(cgroupDirs, logger, func(name string) bool {
		_, ok := skip[name]
		return !ok
	}, opts)
}

// GatherOnly returns families for the named collectors.
func GatherOnly(cgroupDirs []string, logger *slog.Logger, names ...string) ([]*dto.MetricFamily, error) {
	return GatherOnlyOpts(cgroupDirs, logger, HandlerOpts{}, names...)
}

// GatherOnlyOpts is GatherOnly with namespace / uncached options.
func GatherOnlyOpts(cgroupDirs []string, logger *slog.Logger, opts HandlerOpts, names ...string) ([]*dto.MetricFamily, error) {
	want := make(map[string]struct{}, len(names))
	for _, n := range names {
		want[n] = struct{}{}
	}
	return gather(cgroupDirs, logger, func(name string) bool {
		_, ok := want[name]
		return ok
	}, opts)
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

func gather(cgroupDirs []string, logger *slog.Logger, keep func(string) bool, opts HandlerOpts) ([]*dto.MetricFamily, error) {
	cgc, err := newCollector(cgroupDirs, logger, keep, opts)
	if err != nil {
		return nil, err
	}
	return cgc.Gather()
}

func newHandler(cgroupDirs []string, logger *slog.Logger, keep func(string) bool, opts HandlerOpts) (http.Handler, error) {
	cgc, err := newCollector(cgroupDirs, logger, keep, opts)
	if err != nil {
		return nil, err
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mfs, err := cgc.Gather()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeFamilies(w, mfs)
	}), nil
}

func newCollector(cgroupDirs []string, logger *slog.Logger, keep func(string) bool, opts HandlerOpts) (*collector.Cgroup2Collector, error) {
	if logger == nil {
		logger = slog.Default()
	}
	return collector.NewCgroupv2CollectorSelectNS(cgroupDirs, logger, keep, collector.SelectOpts{
		Namespace: opts.Namespace,
		Uncached:  opts.Uncached,
	})
}

func writeFamilies(w http.ResponseWriter, mfs []*dto.MetricFamily) {
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	for _, mf := range mfs {
		if mf == nil {
			continue
		}
		_, _ = expfmt.MetricFamilyToText(w, mf)
	}
}
