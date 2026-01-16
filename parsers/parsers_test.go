package parsers

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/prometheus/common/promslog"
)

var logger = promslog.New(&promslog.Config{})

func TestMultiKeyValueParser(t *testing.T) {
	fileContent := `some avg10=1.23 avg60=4.56 avg300=7.89 total=1234
full avg10=5.67 avg60=8.90 avg300=0.12 total=5678`
	// Define the expected metric names and values
	expectedMetrics := map[string]float64{
		"memory_pressure_some_avg10":  1.23,
		"memory_pressure_some_avg60":  4.56,
		"memory_pressure_some_avg300": 7.89,
		"memory_pressure_some_total":  1234,
		"memory_pressure_full_avg10":  5.67,
		"memory_pressure_full_avg60":  8.90,
		"memory_pressure_full_avg300": 0.12,
		"memory_pressure_full_total":  5678,
	}

	file := strings.NewReader(fileContent)
	parser := &NestedKeyValueParser{
		MetricPrefix: "memory_pressure",
		Logger:       logger,
	}
	metrics, err := parser.Parse(file)
	if err != nil {
		t.Fatalf("Error calling Metrics: %v", err)
	}
	// Compare the actual metrics to the expected metrics
	for metricName, expectedValue := range expectedMetrics {
		actualValue, ok := metrics[metricName]
		if !ok {
			t.Errorf("Metric %s not found in actual metrics", metricName)
			continue
		}

		if actualValue != expectedValue {
			t.Errorf("Metric %s has unexpected value. Expected: %f, Actual: %f", metricName, expectedValue, actualValue)
		}
	}
}

func TestSingleValueParser(t *testing.T) {
	fileContent := `5678`
	file := strings.NewReader(fileContent)

	parser := &SingleValueParser{
		MetricPrefix: "memory_current",
		Logger:       logger,
	}
	// Define the expected metric names and values
	expectedMetrics := map[string]float64{
		"memory_current": 5678,
	}

	metrics, err := parser.Parse(file)
	if err != nil {
		t.Fatalf("Error calling Metrics: %v", err)
	}
	// Compare the actual metrics to the expected metrics
	for metricName, expectedValue := range expectedMetrics {
		actualValue, ok := metrics[metricName]
		if !ok {
			t.Errorf("Metric %s not found in actual metrics", metricName)
			continue
		}

		if actualValue != expectedValue {
			t.Errorf("Metric %s has unexpected value. Expected: %f, Actual: %f", metricName, expectedValue, actualValue)
		}
	}
}

func TestMaxValue(t *testing.T) {
	fileContent := `max`
	file := strings.NewReader(fileContent)

	parser := &SingleValueParser{
		MetricPrefix: "memory_high",
		Logger:       logger,
	}
	// Define the expected metric names and values
	expectedMetrics := map[string]float64{
		"memory_high": math.Inf(1),
	}

	metrics, err := parser.Parse(file)
	if err != nil {
		t.Fatalf("Error calling Metrics: %v", err)
	}
	// Compare the actual metrics to the expected metrics
	for metricName, expectedValue := range expectedMetrics {
		actualValue, ok := metrics[metricName]
		if !ok {
			t.Errorf("Metric %s not found in actual metrics", metricName)
			continue
		}

		if actualValue != expectedValue {
			t.Errorf("Metric %s has unexpected value. Expected: %f, Actual: %f", metricName, expectedValue, actualValue)
		}
	}
}

func TestKeyValueParser(t *testing.T) {
	fileContent := `low 0
	high 5335362
	max 0
	oom 0
	oom_kill 0
`
	file := strings.NewReader(fileContent)

	parser := &FlatKeyValueParser{
		MetricPrefix: "memory_events",
		Logger:       logger,
	}
	// Define the expected metric names and values
	expectedMetrics := map[string]float64{
		"memory_events_low":      0,
		"memory_events_high":     5335362,
		"memory_events_max":      0,
		"memory_events_oom":      0,
		"memory_events_oom_kill": 0,
	}

	metrics, err := parser.Parse(file)
	fmt.Print(metrics)
	if err != nil {
		t.Fatalf("Error calling Metrics: %v", err)
	}
	// Compare the actual metrics to the expected metrics
	for metricName, expectedValue := range expectedMetrics {
		actualValue, ok := metrics[metricName]
		if !ok {
			t.Errorf("Metric %s not found in actual metrics", metricName)
			continue
		}

		if actualValue != expectedValue {
			t.Errorf("Metric %s has unexpected value. Expected: %f, Actual: %f", metricName, expectedValue, actualValue)
		}
	}
}
func TestRangeListCountParser(t *testing.T) {
	tests := []struct {
		name            string
		fileContent     string
		metricPrefix    string
		expectedMetrics map[string]float64
	}{
		{
			name:         "range and single values",
			fileContent:  "0-3,8,10-11",
			metricPrefix: "cpuset_cpus",
			expectedMetrics: map[string]float64{
				"cpuset_cpus_cpu_0":  1,
				"cpuset_cpus_cpu_1":  1,
				"cpuset_cpus_cpu_2":  1,
				"cpuset_cpus_cpu_3":  1,
				"cpuset_cpus_cpu_8":  1,
				"cpuset_cpus_cpu_10": 1,
				"cpuset_cpus_cpu_11": 1,
			},
		},
		{
			name:         "single range",
			fileContent:  "4-8",
			metricPrefix: "cpuset_cpus",
			expectedMetrics: map[string]float64{
				"cpuset_cpus_cpu_4": 1,
				"cpuset_cpus_cpu_5": 1,
				"cpuset_cpus_cpu_6": 1,
				"cpuset_cpus_cpu_7": 1,
				"cpuset_cpus_cpu_8": 1,
			},
		},
		{
			name:         "single value",
			fileContent:  "5",
			metricPrefix: "cpuset_cpus",
			expectedMetrics: map[string]float64{
				"cpuset_cpus_cpu_5": 1,
			},
		},
		{
			name:            "empty file",
			fileContent:     "",
			metricPrefix:    "cpuset_cpus",
			expectedMetrics: map[string]float64{},
		},
		{
			name:            "whitespace only",
			fileContent:     "   ",
			metricPrefix:    "cpuset_cpus",
			expectedMetrics: map[string]float64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			file := strings.NewReader(tt.fileContent)
			parser := &RangeListCountParser{
				MetricPrefix: tt.metricPrefix,
				Logger:       logger,
			}

			metrics, err := parser.Parse(file)
			if err != nil {
				t.Fatalf("Error parsing: %v", err)
			}

			if len(metrics) != len(tt.expectedMetrics) {
				t.Errorf("Expected %d metrics, got %d", len(tt.expectedMetrics), len(metrics))
			}

			for metricName, expectedValue := range tt.expectedMetrics {
				actualValue, ok := metrics[metricName]
				if !ok {
					t.Errorf("Metric %s not found in actual metrics", metricName)
					continue
				}

				if actualValue != expectedValue {
					t.Errorf("Metric %s has unexpected value. Expected: %f, Actual: %f", metricName, expectedValue, actualValue)
				}
			}
		})
	}
}
