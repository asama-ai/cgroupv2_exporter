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
	enabled := isDefaultEnabled
	collectorState[collector] = &enabled
	factories[collector] = factory
}

// RegisterCLIFlags registers kingpin flags for each collector and wires them to
// collectorState. Call only from the cgroupv2_exporter binary main, before
// kingpin.Parse, so library importers are not polluted with these flags.
func RegisterCLIFlags() {
	for collector, enabled := range collectorState {
		var helpDefaultState string
		if *enabled {
			helpDefaultState = "enabled"
		} else {
			helpDefaultState = "disabled"
		}

		flagName := fmt.Sprintf("collector.%s", collector)
		flagHelp := fmt.Sprintf("Enable the %s collector (default: %s).", collector, helpDefaultState)
		defaultValue := fmt.Sprintf("%v", *enabled)

		flag := kingpin.Flag(flagName, flagHelp).Default(defaultValue).Action(collectorFlagAction(collector)).Bool()
		collectorState[collector] = flag
	}
}

type Cgroup2Collector struct {
	Collectors map[string]Collector
	logger     *slog.Logger
	namespace  string
}

type Cgroupv2FileCollector struct {
	parser    parsers.Parser
	dirNames  []string
	fileName  string
	logger    *slog.Logger
	isCounter func(metricName string, labels map[string]string) bool
	namespace string
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

// collectorCacheKey identifies a cached Collector by namespace, name, and cgroup dirs so
// instances are reused only for identical directory configurations.
func collectorCacheKey(ns, name string, cgroups []string) string {
	if ns == "" {
		ns = namespace
	}
	dirs := append([]string(nil), cgroups...)
	sort.Strings(dirs)
	return ns + "\x00" + name + "\x00" + strings.Join(dirs, "\x00")
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
		cacheKey := collectorCacheKey("", key, cgroups)
		if collector, ok := initiatedCollectors[cacheKey]; ok {
			collectors[key] = collector
		} else {
			collector, err := factories[key](slog.With(logger, "collector", key), cgroups)
			if err != nil {
				return nil, err
			}
			setCollectorNamespace(collector, namespace)
			collectors[key] = collector
			initiatedCollectors[cacheKey] = collector
		}
	}
	return &Cgroup2Collector{Collectors: collectors, logger: logger, namespace: namespace}, nil
}

// NewCgroupv2CollectorAll instantiates every registered collector factory,
// ignoring enable/disable state and filters. Intended for host-agent absorb.
func NewCgroupv2CollectorAll(cgroups []string, logger *slog.Logger) (*Cgroup2Collector, error) {
	return NewCgroupv2CollectorSelect(cgroups, logger, nil)
}

// SelectOpts configures NewCgroupv2CollectorSelectNS.
type SelectOpts struct {
	Namespace string // default cgroupv2
	Uncached  bool   // skip initiatedCollectors (watch-set dirs change)
}

// NewCgroupv2CollectorSelect instantiates registered factories for which keep
// returns true. A nil keep keeps every factory (same as All). Ignores
// enable/disable flags.
func NewCgroupv2CollectorSelect(cgroups []string, logger *slog.Logger, keep func(string) bool) (*Cgroup2Collector, error) {
	return NewCgroupv2CollectorSelectNS(cgroups, logger, keep, SelectOpts{})
}

// NewCgroupv2CollectorSelectNS is Select with a metric namespace and optional uncached construction.
func NewCgroupv2CollectorSelectNS(cgroups []string, logger *slog.Logger, keep func(string) bool, opts SelectOpts) (*Cgroup2Collector, error) {
	ns := opts.Namespace
	if ns == "" {
		ns = namespace
	}
	if err := metrics.ValidateMetric(ns); err != nil {
		return nil, fmt.Errorf("invalid metric namespace %q: %w", ns, err)
	}
	collectors := make(map[string]Collector)
	if !opts.Uncached {
		initiatedCollectorsMtx.Lock()
		defer initiatedCollectorsMtx.Unlock()
	}
	for key, factory := range factories {
		if keep != nil && !keep(key) {
			continue
		}
		if !opts.Uncached {
			cacheKey := collectorCacheKey(ns, key, cgroups)
			if collector, ok := initiatedCollectors[cacheKey]; ok {
				collectors[key] = collector
				continue
			}
			collector, err := factory(slog.With(logger, "collector", key), cgroups)
			if err != nil {
				return nil, err
			}
			setCollectorNamespace(collector, ns)
			collectors[key] = collector
			initiatedCollectors[cacheKey] = collector
			continue
		}
		collector, err := factory(slog.With(logger, "collector", key), cgroups)
		if err != nil {
			return nil, err
		}
		setCollectorNamespace(collector, ns)
		collectors[key] = collector
	}
	return &Cgroup2Collector{Collectors: collectors, logger: logger, namespace: ns}, nil
}

func setCollectorNamespace(c Collector, ns string) {
	if fc, ok := c.(*Cgroupv2FileCollector); ok {
		fc.namespace = ns
	}
}

// Scrape runs all collectors and writes series into metricSet (typically a fresh Set per HTTP request).
func (cgc *Cgroup2Collector) Scrape(metricSet *metrics.Set) {
	wg := sync.WaitGroup{}
	wg.Add(len(cgc.Collectors))
	for name, c := range cgc.Collectors {
		go func(name string, c Collector) {
			defer wg.Done()
			execute(metricSet, name, c, cgc.logger, cgc.namespace)
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
	return joinFQNS(namespace, metricName)
}

func joinFQNS(ns, metricName string) string {
	if ns == "" {
		ns = namespace
	}
	return ns + "_" + metricName
}

func (cc *Cgroupv2FileCollector) fq(metricName string) string {
	return joinFQNS(cc.namespace, metricName)
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

func execute(metricSet *metrics.Set, name string, c Collector, logger *slog.Logger, ns string) {
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
	durID := formatMetricID(joinFQNS(ns, "scrape_collector_duration_seconds"), map[string]string{"collector": name})
	metricSet.GetOrCreateGauge(durID, nil).Set(duration.Seconds())
	okID := formatMetricID(joinFQNS(ns, "scrape_collector_success"), map[string]string{"collector": name})
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

				id := formatMetricID(cc.fq(metricName), labels)
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
	registerCollector("cpu.stat.detail", defaultEnabled, NewCpuStatDetailCollector)
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
