package collector

import (
	"log/slog"
	"strings"

	"github.com/asama-ai/cgroupv2_exporter/parsers"
)

// memoryStatIsCounter classifies cgroup memory.stat lines: most keys are current usage (gauge);
// page fault, reclaim, workingset, and THP event keys are cumulative.
func memoryStatIsCounter(stat string) bool {
	if stat == "total" || strings.HasSuffix(stat, "_total") {
		return true
	}
	if strings.HasPrefix(stat, "pgscan_") || strings.HasPrefix(stat, "pgsteal_") {
		return true
	}
	if strings.HasPrefix(stat, "workingset_") {
		return true
	}
	if strings.HasPrefix(stat, "thp_") {
		return true
	}
	switch stat {
	case "pgfault", "pgmajfault", "pgrefill", "pgactivate", "pgdeactivate",
		"oom_kill", "pglazyfree", "pglazyfreed":
		return true
	default:
		return false
	}
}

func NewMemoryPressureCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "memory.pressure"
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

func NewMemoryCurrentCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "memory.current"
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

func NewMemorySwapCurrentCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "memory.swap.current"
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

func NewMemoryHighCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "memory.high"
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

func NewMemoryStatCollector(logger *slog.Logger, cgroups []string) (Collector, error) {
	file := "memory.stat"
	fileLogger := slog.With(logger, "file", file)

	return &Cgroupv2FileCollector{
		parser: &parsers.FlatKeyValueParser{
			MetricPrefix: sanitizeP8sName(file),
			Logger:       fileLogger,
		},
		dirNames: cgroups,
		fileName: file,
		logger:   fileLogger,
		isCounter: func(_ string, labels map[string]string) bool {
			stat, ok := labels["stat"]
			if !ok {
				return false
			}
			return memoryStatIsCounter(stat)
		},
	}, nil
}
