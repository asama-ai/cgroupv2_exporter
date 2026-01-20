package collector

import (
	"log/slog"

	"github.com/asama-ai/cgroupv2_exporter/parsers"
	"github.com/prometheus/client_golang/prometheus"
)

func NewPidsCurrentCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "pids.current"
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
		isCounter: func(metricName string, labels map[string]string) bool { return false },
	}, nil
}

func NewPidsPeakCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "pids.peak"
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
		isCounter: func(metricName string, labels map[string]string) bool { return false },
	}, nil
}
