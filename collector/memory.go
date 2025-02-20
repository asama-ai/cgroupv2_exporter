package collector

import (
	"log/slog"
	"strings"

	"github.com/asama-ai/cgroupv2_exporter/parsers"
	"github.com/prometheus/client_golang/prometheus"
)

func NewMemoryPressureCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "memory.pressure"
	fileLogger := slog.With(logger, "file", file)

	return &Cgroupv2FileCollector{
		gaugeVecs:   make(map[string]*prometheus.GaugeVec),
		counterVecs: make(map[string]*prometheus.CounterVec),
		parser: &parsers.NestedKeyValueParser{
			MetricPrefix: sanitizeP8sName(file),
			Logger:       fileLogger,
		},
		dirNames: cgroups,
		fileName: file,
		logger:   fileLogger,
		isCounter: func(metricName string) bool {
			// total values are counters, avg values are gauges
			return strings.HasSuffix(metricName, "total")
		},
	}, nil
}

func NewMemoryCurrentCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "memory.current"
	fileLogger := slog.With(logger, "file", file)

	return &Cgroupv2FileCollector{
		gaugeVecs:   make(map[string]*prometheus.GaugeVec),
		counterVecs: make(map[string]*prometheus.CounterVec),
		parser: &parsers.SingleValueParser{
			MetricPrefix: sanitizeP8sName(file),
			Logger:       fileLogger,
		},
		dirNames:  cgroups,
		fileName:  file,
		logger:    fileLogger,
		isCounter: func(metricName string) bool { return false },
	}, nil
}

func NewMemorySwapCurrentCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "memory.swap.current"
	fileLogger := slog.With(logger, "file", file)

	return &Cgroupv2FileCollector{
		gaugeVecs:   make(map[string]*prometheus.GaugeVec),
		counterVecs: make(map[string]*prometheus.CounterVec),
		parser: &parsers.SingleValueParser{
			MetricPrefix: sanitizeP8sName(file),
			Logger:       fileLogger,
		},
		dirNames:  cgroups,
		fileName:  file,
		logger:    fileLogger,
		isCounter: func(metricName string) bool { return false },
	}, nil
}

func NewMemoryHighCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "memory.high"
	fileLogger := slog.With(logger, "file", file)

	return &Cgroupv2FileCollector{
		gaugeVecs:   make(map[string]*prometheus.GaugeVec),
		counterVecs: make(map[string]*prometheus.CounterVec),
		parser: &parsers.SingleValueParser{
			MetricPrefix: sanitizeP8sName(file),
			Logger:       fileLogger,
		},
		dirNames:  cgroups,
		fileName:  file,
		logger:    fileLogger,
		isCounter: func(metricName string) bool { return false },
	}, nil
}

func NewMemoryStatCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "memory.stat"
	fileLogger := slog.With(logger, "file", file)

	return &Cgroupv2FileCollector{
		gaugeVecs:   make(map[string]*prometheus.GaugeVec),
		counterVecs: make(map[string]*prometheus.CounterVec),
		parser: &parsers.FlatKeyValueParser{
			MetricPrefix: sanitizeP8sName(file),
			Logger:       fileLogger,
		},
		dirNames: cgroups,
		fileName: file,
		logger:   fileLogger,
		isCounter: func(metricName string) bool {
			return strings.HasSuffix(metricName, "_total")
		},
	}, nil
}
