package collector

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/asama-ai/cgroupv2_exporter/parsers"
	"github.com/prometheus/client_golang/prometheus"
)

// Namespace defines the common namespace to be used by all metrics.
const namespace = "cgroupv2"

var (
	scrapeDurationDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_duration_seconds"),
		"cgroupv2_exporter: Duration of a collector scrape.",
		[]string{"collector"},
		nil,
	)
	scrapeSuccessDesc = prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "scrape", "collector_success"),
		"cgroupv2_exporter: Whether a collector succeeded.",
		[]string{"collector"},
		nil,
	)
)

const (
	defaultEnabled  = true
	defaultDisabled = false
)

var (
	factories              = make(map[string]func(logger *slog.Logger, cgroups []string) (Collector, error))
	initiatedCollectorsMtx = sync.Mutex{}
	initiatedCollectors    = make(map[string]Collector)
	collectorState         = make(map[string]*bool)
	forcedCollectors       = map[string]bool{} // collectors which have been explicitly enabled or disabled
)

func registerCollector(collector string, isDefaultEnabled bool, factory func(logger *slog.Logger, cgroups []string) (Collector, error)) {
	var helpDefaultState string
	if isDefaultEnabled {
		helpDefaultState = "enabled"
	} else {
		helpDefaultState = "disabled"
	}

	flagName := fmt.Sprintf("collector.%s", collector)
	flagHelp := fmt.Sprintf("Enable the %s collector (default: %s).", collector, helpDefaultState)
	defaultValue := fmt.Sprintf("%v", isDefaultEnabled)

	flag := kingpin.Flag(flagName, flagHelp).Default(defaultValue).Action(collectorFlagAction(collector)).Bool()
	collectorState[collector] = flag

	factories[collector] = factory
}

type Cgroup2Collector struct {
	Collectors map[string]Collector
	logger     *slog.Logger
}

type Cgroupv2FileCollector struct {
	gaugeVecs   map[string]*prometheus.GaugeVec
	counterVecs map[string]*prometheus.CounterVec
	parser      parsers.Parser
	dirNames    []string
	fileName    string
	logger      *slog.Logger
	isCounter   func(metricName string) bool
}

// DisableDefaultCollectors sets the collector state to false for all collectors which
// have not been explicitly enabled on the command line.
func DisableDefaultCollectors() {
	for c := range collectorState {
		if _, ok := forcedCollectors[c]; !ok {
			*collectorState[c] = false
		}
	}
}

// collectorFlagAction generates a new action function for the given collector
// to track whether it has been explicitly enabled or disabled from the command line.
// A new action function is needed for each collector flag because the ParseContext
// does not contain information about which flag called the action.
// See: https://github.com/alecthomas/kingpin/issues/294
func collectorFlagAction(collector string) func(ctx *kingpin.ParseContext) error {
	return func(ctx *kingpin.ParseContext) error {
		forcedCollectors[collector] = true
		return nil
	}
}

func NewCgroupv2Collector(cgroups []string, logger *slog.Logger, filters ...string) (*Cgroup2Collector, error) {
	f := make(map[string]bool)
	for _, filter := range filters {
		enabled, exist := collectorState[filter]
		if !exist {
			return nil, fmt.Errorf("missing collector: %s", filter)
		}
		if !*enabled {
			return nil, fmt.Errorf("disabled collector: %s", filter)
		}
		f[filter] = true
	}
	collectors := make(map[string]Collector)
	initiatedCollectorsMtx.Lock()
	defer initiatedCollectorsMtx.Unlock()
	for key, enabled := range collectorState {
		if !*enabled || (len(f) > 0 && !f[key]) {
			continue
		}
		if collector, ok := initiatedCollectors[key]; ok {
			collectors[key] = collector
		} else {
			collector, err := factories[key](slog.With(logger, "collector", key), cgroups)
			if err != nil {
				return nil, err
			}
			collectors[key] = collector
			initiatedCollectors[key] = collector
		}
	}
	return &Cgroup2Collector{Collectors: collectors, logger: logger}, nil
}

func (cgc *Cgroup2Collector) Describe(ch chan<- *prometheus.Desc) {
	// Describe is required for the prometheus.Collector interface but is not used in this project.
}

func sanitizeP8sName(name string) string {
	// Noticed some cgroup names with escape sequence like \x2d. Clean them up.
	if unquoted, err := strconv.Unquote(`"` + name + `"`); err == nil {
		name = unquoted
	}

	// Use a regular expression to replace unsupported characters with underscores
	regex := regexp.MustCompile(`[^a-zA-Z0-9_:]`)
	name = regex.ReplaceAllString(name, "_")

	// squeeze underscore repeats
	regex = regexp.MustCompile(`_+`)
	name = regex.ReplaceAllString(name, "_")

	name = strings.Trim(name, "_")

	return name
}

// Collect implements the prometheus.Collector interface.
func (cgc *Cgroup2Collector) Collect(ch chan<- prometheus.Metric) {
	wg := sync.WaitGroup{}
	wg.Add(len(cgc.Collectors))
	for name, c := range cgc.Collectors {
		go func(name string, c Collector) {
			execute(name, c, ch, cgc.logger)
			wg.Done()
		}(name, c)
	}
	wg.Wait()
}

func execute(name string, c Collector, ch chan<- prometheus.Metric, logger *slog.Logger) {
	begin := time.Now()
	err := c.Update(ch)
	duration := time.Since(begin)
	var success float64

	if err != nil {
		if IsNoDataError(err) {
			logger.Debug("collector returned no data", "name", name, "duration_seconds", duration.Seconds(), "err", err)
		} else {
			logger.Error("collector failed", "name", name, "duration_seconds", duration.Seconds(), "err", err)
		}
		success = 0
	} else {
		logger.Debug("collector succeeded", "name", name, "duration_seconds", duration.Seconds())
		success = 1
	}
	ch <- prometheus.MustNewConstMetric(scrapeDurationDesc, prometheus.GaugeValue, duration.Seconds(), name)
	ch <- prometheus.MustNewConstMetric(scrapeSuccessDesc, prometheus.GaugeValue, success, name)
}

func (cc Cgroupv2FileCollector) Update(ch chan<- prometheus.Metric) error {
	// Use the parser to fetch metrics for the specified file in all cgroup directories
	for _, dirName := range cc.dirNames {
		filePath := filepath.Join(dirName, cc.fileName)
		file, err := os.Open(filePath)
		if err != nil {
			cc.logger.Error("failed to open file", "dir", dirName, "err", err)
			return err
		}
		defer file.Close()

		metrics, err := cc.parser.Parse(file)
		if err != nil {
			cc.logger.Error("failed to parse file", "dir", dirName, "err", err)
			return err
		}

		cgroupName := sanitizeP8sName(filepath.Base(dirName))
		// Set the metric value with the directory label
		for key, value := range metrics {
			metricName := sanitizeP8sName(key)

			if cc.isCounter(metricName) {
				// Handle as Counter
				if _, ok := cc.counterVecs[metricName]; !ok {
					cc.counterVecs[metricName] = prometheus.NewCounterVec(
						prometheus.CounterOpts{
							Namespace: "cgroupv2",
							Name:      metricName,
							Help:      fmt.Sprintf("metric %s from file %s", metricName, cc.fileName),
						},
						[]string{"cgroup"},
					)
				}
				cc.counterVecs[metricName].WithLabelValues(cgroupName).Add(value)
				cc.counterVecs[metricName].Collect(ch)
			} else {
				// Handle as Gauge (existing code)
				if _, ok := cc.gaugeVecs[metricName]; !ok {
					cc.gaugeVecs[metricName] = prometheus.NewGaugeVec(
						prometheus.GaugeOpts{
							Namespace: "cgroupv2",
							Name:      metricName,
							Help:      fmt.Sprintf("metric %s from file %s", metricName, cc.fileName),
						},
						[]string{"cgroup"},
					)
				}
				cc.gaugeVecs[metricName].WithLabelValues(cgroupName).Set(value)
				cc.gaugeVecs[metricName].Collect(ch)
			}
			cc.logger.Debug("collected metric", "name", metricName, "value", value, "cgroup", cgroupName)
		}
	}
	return nil
}

// Collector is the interface a collector has to implement.
type Collector interface {
	// Get new metrics and expose them via prometheus registry.
	Update(ch chan<- prometheus.Metric) error
}

// ErrNoData indicates the collector found no data to collect, but had no other error.
var ErrNoData = errors.New("collector returned no data")

func IsNoDataError(err error) bool {
	return err == ErrNoData
}

func init() {
	registerCollector("memory.pressure", defaultEnabled, NewMemoryPressureCollector)
	registerCollector("memory.current", defaultEnabled, NewMemoryCurrentCollector)
	registerCollector("memory.swap.current", defaultEnabled, NewMemorySwapCurrentCollector)
	registerCollector("memory.high", defaultEnabled, NewMemoryHighCollector)
	registerCollector("memory.stat", defaultEnabled, NewMemoryStatCollector)
	registerCollector("cpu.stat", defaultEnabled, NewCpuStatCollector)
	registerCollector("cpu.pressure", defaultEnabled, NewCpuPressureCollector)
	registerCollector("io.pressure", defaultEnabled, NewIoPressureCollector)
	registerCollector("io.stat", defaultEnabled, NewIoStatCollector)
	registerCollector("pids.current", defaultEnabled, NewPidsCurrentCollector)
	registerCollector("pids.peak", defaultEnabled, NewPidsPeakCollector)
}
