package collector

import (
	"log/slog"

	"github.com/asama-ai/cgroupv2_exporter/parsers"
)

func NewCpuStatCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "cpu.stat"
	fileLogger := slog.With(logger, "file", file)

	return &Cgroupv2FileCollector{
		parser: &parsers.FlatKeyValueParser{
			MetricPrefix: sanitizeP8sName(file),
			Logger:       fileLogger,
		},
		dirNames: cgroups,
		fileName: file,
		logger:   fileLogger,
		// Cumulative kernel counters; FloatCounter.Set publishes the absolute value each scrape.
		isCounter: func(metricName string, labels map[string]string) bool { return true },
	}, nil
}

func NewCpuPressureCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "cpu.pressure"
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

func NewCPUSetCpusCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "cpuset.cpus"
	fileLogger := slog.With(logger, "file", file)

	return &Cgroupv2FileCollector{
		parser: &parsers.RangeListCountParser{
			MetricPrefix: sanitizeP8sName(file),
			Logger:       fileLogger,
		},
		dirNames:  cgroups,
		fileName:  file,
		logger:    fileLogger,
		isCounter: func(metricName string, labels map[string]string) bool { return false },
	}, nil
}

func NewCPUSetCpusEffectiveCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "cpuset.cpus.effective"
	fileLogger := slog.With(logger, "file", file)

	return &Cgroupv2FileCollector{
		parser: &parsers.RangeListCountParser{
			MetricPrefix: sanitizeP8sName(file),
			Logger:       fileLogger,
		},
		dirNames:  cgroups,
		fileName:  file,
		logger:    fileLogger,
		isCounter: func(metricName string, labels map[string]string) bool { return false },
	}, nil
}

func NewCPUSetMemsCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "cpuset.mems"
	fileLogger := slog.With(logger, "file", file)

	return &Cgroupv2FileCollector{
		parser: &parsers.RangeListCountParser{
			MetricPrefix: sanitizeP8sName(file),
			Logger:       fileLogger,
		},
		dirNames:  cgroups,
		fileName:  file,
		logger:    fileLogger,
		isCounter: func(metricName string, labels map[string]string) bool { return false },
	}, nil
}

func NewCPUSetMemsEffectiveCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "cpuset.mems.effective"
	fileLogger := slog.With(logger, "file", file)

	return &Cgroupv2FileCollector{
		parser: &parsers.RangeListCountParser{
			MetricPrefix: sanitizeP8sName(file),
			Logger:       fileLogger,
		},
		dirNames:  cgroups,
		fileName:  file,
		logger:    fileLogger,
		isCounter: func(metricName string, labels map[string]string) bool { return false },
	}, nil
}
