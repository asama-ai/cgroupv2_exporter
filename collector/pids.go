package collector

import (
	"log/slog"

	"github.com/asama-ai/cgroupv2_exporter/parsers"
)

func NewPidsCurrentCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "pids.current"
	fileLogger := slog.With(logger, "file", file)

	return &Cgroupv2FileCollector{
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
