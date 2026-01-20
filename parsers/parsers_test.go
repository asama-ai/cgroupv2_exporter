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
	// Define the expected metrics with labels
	expectedMetrics := []Metric{
		{Name: "memory_pressure_avg10", Value: 1.23, Labels: map[string]string{"type": "some"}},
		{Name: "memory_pressure_avg60", Value: 4.56, Labels: map[string]string{"type": "some"}},
		{Name: "memory_pressure_avg300", Value: 7.89, Labels: map[string]string{"type": "some"}},
		{Name: "memory_pressure_total", Value: 1234, Labels: map[string]string{"type": "some"}},
		{Name: "memory_pressure_avg10", Value: 5.67, Labels: map[string]string{"type": "full"}},
		{Name: "memory_pressure_avg60", Value: 8.90, Labels: map[string]string{"type": "full"}},
		{Name: "memory_pressure_avg300", Value: 0.12, Labels: map[string]string{"type": "full"}},
		{Name: "memory_pressure_total", Value: 5678, Labels: map[string]string{"type": "full"}},
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

	if len(metrics) != len(expectedMetrics) {
		t.Fatalf("Expected %d metrics, got %d", len(expectedMetrics), len(metrics))
	}

	// Build a map for easier comparison
	actualMap := make(map[string]Metric)
	for _, m := range metrics {
		key := fmt.Sprintf("%s|%v", m.Name, m.Labels)
		actualMap[key] = m
	}

	for _, expected := range expectedMetrics {
		key := fmt.Sprintf("%s|%v", expected.Name, expected.Labels)
		actual, ok := actualMap[key]
		if !ok {
			t.Errorf("Metric %s with labels %v not found", expected.Name, expected.Labels)
			continue
		}

		if actual.Value != expected.Value {
			t.Errorf("Metric %s with labels %v has unexpected value. Expected: %f, Actual: %f", expected.Name, expected.Labels, expected.Value, actual.Value)
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

	metrics, err := parser.Parse(file)
	if err != nil {
		t.Fatalf("Error calling Metrics: %v", err)
	}

	if len(metrics) != 1 {
		t.Fatalf("Expected 1 metric, got %d", len(metrics))
	}

	if metrics[0].Name != "memory_current" {
		t.Errorf("Expected metric name 'memory_current', got '%s'", metrics[0].Name)
	}

	if metrics[0].Value != 5678 {
		t.Errorf("Expected value 5678, got %f", metrics[0].Value)
	}

	if len(metrics[0].Labels) != 0 {
		t.Errorf("Expected empty labels, got %v", metrics[0].Labels)
	}
}

func TestMaxValue(t *testing.T) {
	fileContent := `max`
	file := strings.NewReader(fileContent)

	parser := &SingleValueParser{
		MetricPrefix: "memory_high",
		Logger:       logger,
	}

	metrics, err := parser.Parse(file)
	if err != nil {
		t.Fatalf("Error calling Metrics: %v", err)
	}

	if len(metrics) != 1 {
		t.Fatalf("Expected 1 metric, got %d", len(metrics))
	}

	if metrics[0].Name != "memory_high" {
		t.Errorf("Expected metric name 'memory_high', got '%s'", metrics[0].Name)
	}

	if !math.IsInf(metrics[0].Value, 1) {
		t.Errorf("Expected +Inf, got %f", metrics[0].Value)
	}

	if len(metrics[0].Labels) != 0 {
		t.Errorf("Expected empty labels, got %v", metrics[0].Labels)
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
	// Define the expected metrics with labels
	expectedMetrics := []Metric{
		{Name: "memory_events", Value: 0, Labels: map[string]string{"stat": "low"}},
		{Name: "memory_events", Value: 5335362, Labels: map[string]string{"stat": "high"}},
		{Name: "memory_events", Value: 0, Labels: map[string]string{"stat": "max"}},
		{Name: "memory_events", Value: 0, Labels: map[string]string{"stat": "oom"}},
		{Name: "memory_events", Value: 0, Labels: map[string]string{"stat": "oom_kill"}},
	}

	metrics, err := parser.Parse(file)
	if err != nil {
		t.Fatalf("Error calling Metrics: %v", err)
	}

	if len(metrics) != len(expectedMetrics) {
		t.Fatalf("Expected %d metrics, got %d", len(expectedMetrics), len(metrics))
	}

	// Build a map for easier comparison using metric name and stat label
	actualMap := make(map[string]Metric)
	for _, m := range metrics {
		key := fmt.Sprintf("%s|%s", m.Name, m.Labels["stat"])
		actualMap[key] = m
	}

	for _, expected := range expectedMetrics {
		key := fmt.Sprintf("%s|%s", expected.Name, expected.Labels["stat"])
		actual, ok := actualMap[key]
		if !ok {
			t.Errorf("Metric %s with stat=%s not found", expected.Name, expected.Labels["stat"])
			continue
		}

		if actual.Value != expected.Value {
			t.Errorf("Metric %s with stat=%s has unexpected value. Expected: %f, Actual: %f", expected.Name, expected.Labels["stat"], expected.Value, actual.Value)
		}

		if actual.Labels["stat"] != expected.Labels["stat"] {
			t.Errorf("Metric %s has unexpected stat label. Expected: %s, Actual: %s", expected.Name, expected.Labels["stat"], actual.Labels["stat"])
		}
	}
}
func TestRangeListCountParser(t *testing.T) {
	tests := []struct {
		name         string
		fileContent  string
		metricPrefix string
		expected     []Metric
	}{
		{
			name:         "range and single values with cpu",
			fileContent:  "0-3,8,10-11",
			metricPrefix: "cpuset_cpus",
			expected: []Metric{
				{Name: "cpuset_cpus", Value: 1, Labels: map[string]string{"cpucore": "0"}},
				{Name: "cpuset_cpus", Value: 1, Labels: map[string]string{"cpucore": "1"}},
				{Name: "cpuset_cpus", Value: 1, Labels: map[string]string{"cpucore": "2"}},
				{Name: "cpuset_cpus", Value: 1, Labels: map[string]string{"cpucore": "3"}},
				{Name: "cpuset_cpus", Value: 1, Labels: map[string]string{"cpucore": "8"}},
				{Name: "cpuset_cpus", Value: 1, Labels: map[string]string{"cpucore": "10"}},
				{Name: "cpuset_cpus", Value: 1, Labels: map[string]string{"cpucore": "11"}},
			},
		},
		{
			name:         "single range with cpu",
			fileContent:  "4-8",
			metricPrefix: "cpuset_cpus",
			expected: []Metric{
				{Name: "cpuset_cpus", Value: 1, Labels: map[string]string{"cpucore": "4"}},
				{Name: "cpuset_cpus", Value: 1, Labels: map[string]string{"cpucore": "5"}},
				{Name: "cpuset_cpus", Value: 1, Labels: map[string]string{"cpucore": "6"}},
				{Name: "cpuset_cpus", Value: 1, Labels: map[string]string{"cpucore": "7"}},
				{Name: "cpuset_cpus", Value: 1, Labels: map[string]string{"cpucore": "8"}},
			},
		},
		{
			name:         "single value with cpu",
			fileContent:  "5",
			metricPrefix: "cpuset_cpus",
			expected: []Metric{
				{Name: "cpuset_cpus", Value: 1, Labels: map[string]string{"cpucore": "5"}},
			},
		},
		{
			name:         "range with numanode",
			fileContent:  "0-1",
			metricPrefix: "cpuset_mems",
			expected: []Metric{
				{Name: "cpuset_mems", Value: 1, Labels: map[string]string{"numanode": "0"}},
				{Name: "cpuset_mems", Value: 1, Labels: map[string]string{"numanode": "1"}},
			},
		},
		{
			name:         "empty file",
			fileContent:  "",
			metricPrefix: "cpuset_cpus",
			expected:     []Metric{},
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

			if len(metrics) != len(tt.expected) {
				t.Fatalf("Expected %d metrics, got %d", len(tt.expected), len(metrics))
			}

			// Build a map for easier comparison using metric name and labels as key
			actualMap := make(map[string]Metric)
			for _, m := range metrics {
				key := fmt.Sprintf("%s|%v", m.Name, m.Labels)
				actualMap[key] = m
			}

			for _, expected := range tt.expected {
				key := fmt.Sprintf("%s|%v", expected.Name, expected.Labels)
				actual, ok := actualMap[key]
				if !ok {
					t.Errorf("Metric %s with labels %v not found", expected.Name, expected.Labels)
					continue
				}

				if actual.Value != expected.Value {
					t.Errorf("Metric %s with labels %v has unexpected value. Expected: %f, Actual: %f", expected.Name, expected.Labels, expected.Value, actual.Value)
				}
			}
		})
	}
}
