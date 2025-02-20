/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"fmt"
	stdlog "log"
	"log/slog"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"

	"github.com/prometheus/common/promslog"
	"github.com/prometheus/common/promslog/flag"
	"github.com/prometheus/common/version"

	"github.com/alecthomas/kingpin/v2"
	"github.com/asama-ai/cgroupv2_exporter/collector"
	"github.com/prometheus/client_golang/prometheus"
	promcollectors "github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/exporter-toolkit/web"
	"github.com/prometheus/exporter-toolkit/web/kingpinflag"
)

// slogWriter adapts slog.Logger to io.Writer for promhttp
type slogWriter struct {
	logger *slog.Logger
}

func (w *slogWriter) Write(p []byte) (n int, err error) {
	w.logger.Error(string(p))
	return len(p), nil
}

// handler wraps an unfiltered http.Handler but uses a filtered handler,
// created on the fly, if filtering is requested. Create instances with
// newHandler.
type handler struct {
	unfilteredHandler http.Handler
	// exporterMetricsRegistry is a separate registry for the metrics about
	// the exporter itself.
	exporterMetricsRegistry *prometheus.Registry
	includeExporterMetrics  bool
	maxRequests             int
	logger                  *slog.Logger
	cgroups                 []string
}

func newHandler(cgroups []string, includeExporterMetrics bool, maxRequests int, logger *slog.Logger) *handler {
	h := &handler{
		exporterMetricsRegistry: prometheus.NewRegistry(),
		includeExporterMetrics:  includeExporterMetrics,
		maxRequests:             maxRequests,
		logger:                  logger,
		cgroups:                 cgroups,
	}
	if h.includeExporterMetrics {
		h.exporterMetricsRegistry.MustRegister(
			promcollectors.NewProcessCollector(promcollectors.ProcessCollectorOpts{}),
			promcollectors.NewGoCollector(),
		)
	}
	if innerHandler, err := h.innerHandler(cgroups); err != nil {
		panic(fmt.Sprintf("Couldn't create metrics handler: %s", err))
	} else {
		h.unfilteredHandler = innerHandler
	}
	return h
}

// ServeHTTP implements http.Handler.
func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	filters := r.URL.Query()["collect[]"]
	h.logger.Debug("collect query", slog.Any("filters", filters))

	if len(filters) == 0 {
		// No filters, use the prepared unfiltered handler.
		h.unfilteredHandler.ServeHTTP(w, r)
		return
	}
	// To serve filtered metrics, we create a filtering handler on the fly.
	filteredHandler, err := h.innerHandler(h.cgroups, filters...)
	if err != nil {
		h.logger.Warn("Couldn't create filtered metrics handler", "err", err)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(fmt.Sprintf("Couldn't create filtered metrics handler: %s", err)))
		return
	}
	filteredHandler.ServeHTTP(w, r)
}

// innerHandler is used to create both the one unfiltered http.Handler to be
// wrapped by the outer handler and also the filtered handlers created on the
// fly. The former is accomplished by calling innerHandler without any arguments
// (in which case it will log all the collectors enabled via command-line
// flags).
func (h *handler) innerHandler(cgroups []string, filters ...string) (http.Handler, error) {
	cgc, err := collector.NewCgroupv2Collector(cgroups, h.logger, filters...)
	if err != nil {
		return nil, fmt.Errorf("couldn't create collector: %s", err)
	}

	// Only log the creation of an unfiltered handler, which should happen
	// only once upon startup.
	if len(filters) == 0 {
		h.logger.Info("enabled collectors")
		collectors := []string{}
		for n := range cgc.Collectors {
			collectors = append(collectors, n)
		}
		sort.Strings(collectors)
		for _, c := range collectors {
			h.logger.Info("collector enabled", "name", c)
		}
	}

	r := prometheus.NewRegistry()

	// Create version collector
	versionCollector := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "cgroupv2_exporter_build_info",
			Help: "A metric with a constant '1' value labeled by version, revision, branch, and goversion from which cgroupv2_exporter was built.",
		},
		[]string{"version", "revision", "branch", "goversion"},
	)
	versionCollector.WithLabelValues(version.Version, version.Revision, version.Branch, version.GoVersion).Set(1)
	r.MustRegister(versionCollector)

	if err := r.Register(cgc); err != nil {
		return nil, fmt.Errorf("couldn't register cgroupv2 collector: %s", err)
	}
	handler := promhttp.HandlerFor(
		prometheus.Gatherers{h.exporterMetricsRegistry, r},
		promhttp.HandlerOpts{
			ErrorLog:            stdlog.New(&slogWriter{logger: h.logger}, "", 0),
			ErrorHandling:       promhttp.ContinueOnError,
			MaxRequestsInFlight: h.maxRequests,
			Registry:            h.exporterMetricsRegistry,
		},
	)
	if h.includeExporterMetrics {
		// Note that we have to use h.exporterMetricsRegistry here to
		// use the same promhttp metrics for all expositions.
		handler = promhttp.InstrumentMetricHandler(
			h.exporterMetricsRegistry, handler,
		)
	}
	return handler, nil
}

func main() {
	var (
		cgroupGlobs = kingpin.Flag(
			"cgroup.glob",
			"glob of cgroup directories to scrape (can be specified multiple times)",
		).Default("/sys/fs/cgroup/*").Strings()
		metricsPath = kingpin.Flag(
			"web.telemetry-path",
			"Path under which to expose metrics.",
		).Default("/metrics").String()
		disableExporterMetrics = kingpin.Flag(
			"web.disable-exporter-metrics",
			"Exclude metrics about the exporter itself (promhttp_*, process_*, go_*).",
		).Bool()
		maxRequests = kingpin.Flag(
			"web.max-requests",
			"Maximum number of parallel scrape requests. Use 0 to disable.",
		).Default("4").Int()
		disableDefaultCollectors = kingpin.Flag(
			"collector.disable-defaults",
			"Set all collectors to disabled by default.",
		).Default("false").Bool()
		maxProcs = kingpin.Flag(
			"runtime.gomaxprocs", "The target number of CPUs Go will run on (GOMAXPROCS)",
		).Envar("GOMAXPROCS").Default("1").Int()
		toolkitFlags = kingpinflag.AddFlags(kingpin.CommandLine, ":9100")
	)

	promslogConfig := &promslog.Config{}
	flag.AddFlags(kingpin.CommandLine, promslogConfig)
	kingpin.Version(version.Print("cgroupv2_exporter"))
	kingpin.CommandLine.UsageWriter(os.Stdout)
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	logger := promslog.New(promslogConfig)

	if *disableDefaultCollectors {
		collector.DisableDefaultCollectors()
	}
	logger.Info("starting cgroupv2_exporter", "version", version.Info())
	logger.Info("build context", "context", version.BuildContext())
	if user, err := user.Current(); err == nil && user.Uid == "0" {
		logger.Warn("CgroupV2 Exporter is running as root user. This exporter is designed to run as unprivileged user, root is not required.")
	}
	runtime.GOMAXPROCS(*maxProcs)
	logger.Debug("Go MAXPROCS", "procs", runtime.GOMAXPROCS(0))

	// Expand all glob patterns to get list of directories
	var allCgroups []string
	for _, globPattern := range *cgroupGlobs {
		matches, err := filepath.Glob(globPattern)
		if err != nil {
			logger.Error("Failed to expand glob pattern", "pattern", globPattern, "err", err)
			continue
		}
		// Only append directories
		for _, match := range matches {
			fi, err := os.Stat(match)
			if err != nil {
				logger.Error("Failed to stat path", "path", match, "err", err)
				continue
			}
			if fi.IsDir() {
				allCgroups = append(allCgroups, match)
			}
		}
	}

	if len(allCgroups) == 0 {
		logger.Error("No cgroup directories found from any glob pattern")
	}

	http.Handle(*metricsPath, newHandler(allCgroups, !*disableExporterMetrics, *maxRequests, logger))
	if *metricsPath != "/" {
		landingConfig := web.LandingConfig{
			Name:        "CgroupV2 Exporter",
			Description: "Prometheus CgroupV2 Exporter",
			Version:     version.Info(),
			Links: []web.LandingLinks{
				{
					Address: *metricsPath,
					Text:    "Metrics",
				},
			},
		}
		landingPage, err := web.NewLandingPage(landingConfig)
		if err != nil {
			logger.Error("Error creating landing page", "err", err)
			os.Exit(1)
		}
		http.Handle("/", landingPage)
	}

	server := &http.Server{}
	if err := web.ListenAndServe(server, toolkitFlags, logger); err != nil {
		logger.Error("Server error", "err", err)
		os.Exit(1)
	}
}
