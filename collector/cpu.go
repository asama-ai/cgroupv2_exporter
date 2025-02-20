package collector

import (
	"log/slog"
	"strings"

	"github.com/asama-ai/cgroupv2_exporter/parsers"
	"github.com/prometheus/client_golang/prometheus"
)

func NewCpuStatCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "cpu.stat"
	fileLogger := slog.With(logger, "file", file)

	return &Cgroupv2FileCollector{
		gaugeVecs:   make(map[string]*prometheus.GaugeVec),
		counterVecs: make(map[string]*prometheus.CounterVec),
		parser: &parsers.FlatKeyValueParser{
			MetricPrefix: sanitizeP8sName(file),
			Logger:       fileLogger,
		},
		dirNames:  cgroups,
		fileName:  file,
		logger:    fileLogger,
		isCounter: func(metricName string) bool { return true },
	}, nil
}

func NewCpuPressureCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "cpu.pressure"
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
