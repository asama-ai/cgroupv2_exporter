package collector

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/alecthomas/kingpin/v2"
	"github.com/asama-ai/cgroupv2_exporter/parsers"
)

// Namespace defines the common namespace to be used by all metrics.
const namespace = "cgroupv2"

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
	parser    parsers.Parser
	dirNames  []string
	fileName  string
	logger    *slog.Logger
	isCounter func(metricName string, labels map[string]string) bool
}

// isPressureTotalField matches cgroup *.pressure cumulative stall time (the total=... field).
// NestedKeyValueParser uses label "type" for some|full; the field name is the metric suffix (_total).
func isPressureTotalField(metricName string) bool {
	return strings.HasSuffix(metricName, "_total")
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

// Scrape runs all collectors and writes series into metricSet (typically a fresh Set per HTTP request).
func (cgc *Cgroup2Collector) Scrape(metricSet *metrics.Set) {
	wg := sync.WaitGroup{}
	wg.Add(len(cgc.Collectors))
	for name, c := range cgc.Collectors {
		go func(name string, c Collector) {
			defer wg.Done()
			execute(metricSet, name, c, cgc.logger)
		}(name, c)
	}
	wg.Wait()
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

func joinFQ(metricName string) string {
	return namespace + "_" + metricName
}

// escapeLabelValue formats s as a Prometheus label value (quoted, escaped).
func escapeLabelValue(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func formatMetricID(fqMetricName string, labels map[string]string) string {
	if len(labels) == 0 {
		return fqMetricName
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(fqMetricName)
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(escapeLabelValue(labels[k]))
	}
	b.WriteByte('}')
	return b.String()
}

// BuildInfoMetric returns the metric id for cgroupv2_exporter_build_info with the given labels.
func BuildInfoMetric(version, revision, branch, goversion string) string {
	return formatMetricID(joinFQ("exporter_build_info"), map[string]string{
		"version":   version,
		"revision":  revision,
		"branch":    branch,
		"goversion": goversion,
	})
}

func execute(metricSet *metrics.Set, name string, c Collector, logger *slog.Logger) {
	begin := time.Now()
	err := c.Update(metricSet)
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
	durID := formatMetricID(joinFQ("scrape_collector_duration_seconds"), map[string]string{"collector": name})
	metricSet.GetOrCreateGauge(durID, nil).Set(duration.Seconds())
	okID := formatMetricID(joinFQ("scrape_collector_success"), map[string]string{"collector": name})
	metricSet.GetOrCreateGauge(okID, nil).Set(success)
}

func (cc *Cgroupv2FileCollector) Update(metricSet *metrics.Set) error {
	for _, dirName := range cc.dirNames {
		filePath := filepath.Join(dirName, cc.fileName)
		file, err := os.Open(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				cc.logger.Debug("file not found, skipping", "file", cc.fileName, "dir", dirName)
				continue
			}
			cc.logger.Error("failed to open file", "dir", dirName, "err", err)
			continue
		}
		func() {
			defer file.Close()
			metricsFromFile, err := cc.parser.Parse(file)
			if err != nil {
				cc.logger.Error("failed to parse file", "dir", dirName, "err", err)
				return
			}

			cgroupName := sanitizeP8sName(filepath.Base(dirName))
			for _, metric := range metricsFromFile {
				metricName := sanitizeP8sName(metric.Name)

				labels := make(map[string]string, 1+len(metric.Labels))
				labels["cgroup"] = cgroupName
				for labelName, labelValue := range metric.Labels {
					labels[labelName] = labelValue
				}

				id := formatMetricID(joinFQ(metricName), labels)
				if cc.isCounter(metricName, metric.Labels) {
					metricSet.GetOrCreateFloatCounter(id).Set(metric.Value)
				} else {
					metricSet.GetOrCreateGauge(id, nil).Set(metric.Value)
				}
				cc.logger.Debug("collected metric", "name", metricName, "value", metric.Value, "labels", metric.Labels, "cgroup", cgroupName)
			}
		}()
	}

	return nil
}

// Collector is the interface a collector has to implement.
type Collector interface {
	Update(metricSet *metrics.Set) error
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
	registerCollector("memory.stat", defaultDisabled, NewMemoryStatCollector)
	registerCollector("cpu.pressure", defaultEnabled, NewCpuPressureCollector)
	registerCollector("cpuset.cpus", defaultEnabled, NewCPUSetCpusCollector)
	registerCollector("cpuset.cpus.effective", defaultEnabled, NewCPUSetCpusEffectiveCollector)
	registerCollector("cpu.stat", defaultEnabled, NewCpuStatCollector)
	registerCollector("cpuset.mems", defaultEnabled, NewCPUSetMemsCollector)
	registerCollector("cpuset.mems.effective", defaultEnabled, NewCPUSetMemsEffectiveCollector)
	registerCollector("io.pressure", defaultEnabled, NewIoPressureCollector)
	registerCollector("io.stat", defaultEnabled, NewIoStatCollector)
	registerCollector("pids.current", defaultEnabled, NewPidsCurrentCollector)
	registerCollector("pids.peak", defaultEnabled, NewPidsPeakCollector)
}

const (
	defaultEnabled  = true
	defaultDisabled = false
)
