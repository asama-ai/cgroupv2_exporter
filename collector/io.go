package collector

import (
	"log/slog"

	"github.com/asama-ai/cgroupv2_exporter/parsers"
)

func NewIoPressureCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "io.pressure"
	fileLogger := slog.With(logger, "file", file)

	return &Cgroupv2FileCollector{
		parser: &parsers.NestedKeyValueParser{
			MetricPrefix: sanitizeP8sName(file),
			Logger:       fileLogger,
		},
		dirNames:  cgroups,
		fileName:  file,
		logger:    fileLogger,
		isCounter: func(metricName string, _ map[string]string) bool { return isPressureTotalField(metricName) },
	}, nil
}

func NewIoStatCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "io.stat"
	fileLogger := slog.With(logger, "file", file)

	return &Cgroupv2FileCollector{
		parser: &parsers.NestedKeyValueParser{
			MetricPrefix: sanitizeP8sName(file),
			Logger:       fileLogger,
		},
		dirNames: cgroups,
		fileName: file,
		logger:   fileLogger,
		// Per-device rbytes, wbytes, rios, wios, etc. are cumulative.
		isCounter: func(metricName string, labels map[string]string) bool { return true },
	}, nil
}
