package parsers

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"math"
	"strconv"
	"strings"
)

// Parser defines the interface for file parsers.
type Parser interface {
	Parse(io.Reader) ([]Metric, error)
}

// Metric represents a parsed metric with its name, value, and labels.
type Metric struct {
	Name   string
	Value  float64
	Labels map[string]string
}

type SingleValueParser struct {
	MetricPrefix string
	Logger       *slog.Logger
}

type FlatKeyValueParser struct {
	MetricPrefix string
	Logger       *slog.Logger
}

type NestedKeyValueParser struct {
	MetricPrefix string
	Logger       *slog.Logger
}

type RangeListCountParser struct {
	MetricPrefix string
	Logger       *slog.Logger
}

func readContent(file io.Reader) (string, error) {
	// Read the entire file content
	var content strings.Builder
	_, err := io.Copy(&content, file)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(content.String()), nil
}

func (p *SingleValueParser) Parse(file io.Reader) ([]Metric, error) {
	content, err := readContent(file)
	if err != nil {
		p.Logger.Error("error reading file", "err", err)
		return nil, err
	}
	// Check if content is "max" and convert it to +Inf
	var value float64
	if content == "max" {
		p.Logger.Debug("converting max to +Inf")
		value = math.Inf(1)
	} else {
		var err error
		value, err = strconv.ParseFloat(content, 64)
		if err != nil {
			p.Logger.Error("failed to parse value", "err", err)
			return nil, err
		}
	}
	return []Metric{
		{
			Name:   p.MetricPrefix,
			Value:  value,
			Labels: map[string]string{},
		},
	}, nil
}

func (p *FlatKeyValueParser) Parse(file io.Reader) ([]Metric, error) {
	var metrics []Metric

	// Read the file line by line and parse key-value pairs
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) != 2 {
			p.Logger.Error("invalid field count", "expected", 2, "got", len(parts))
			continue
		}
		value, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			p.Logger.Error("failed to parse value", "err", err)
			continue
		}
		// Use parts[0] as a label instead of embedding in metric name
		metrics = append(metrics, Metric{
			Name:   p.MetricPrefix,
			Value:  value,
			Labels: map[string]string{"stat": parts[0]},
		})
	}

	if err := scanner.Err(); err != nil {
		p.Logger.Error("scanner error", "err", err)
		return nil, err
	}

	return metrics, nil
}

func (p *NestedKeyValueParser) Parse(file io.Reader) ([]Metric, error) {
	var metrics []Metric

	// Read the file line by line and parse
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) < 2 {
			p.Logger.Error("invalid field count", "expected_min", 2, "got", len(parts))
			continue
		}
		prefix := parts[0]
		for _, m := range parts[1:] {
			metric := strings.Split(m, "=")
			if len(metric) != 2 {
				p.Logger.Error("failed to parse key-value pair", "input", m)
				continue
			}
			metricName := fmt.Sprintf("%s_%s", p.MetricPrefix, metric[0])
			value, err := strconv.ParseFloat(metric[1], 64)
			if err != nil {
				p.Logger.Error("failed to parse value", "err", err)
				continue
			}
			// Use prefix as a label (e.g., device ID like "259:0" or pressure type like "some", "full")
			// Detect label name: if metric prefix contains "pressure", use "type", otherwise use "device"
			labelName := "device"
			if strings.Contains(p.MetricPrefix, "pressure") {
				labelName = "type"
			}
			metrics = append(metrics, Metric{
				Name:   metricName,
				Value:  value,
				Labels: map[string]string{labelName: prefix},
			})
		}
	}

	if err := scanner.Err(); err != nil {
		p.Logger.Error("scanner error", "err", err)
		return nil, err
	}

	return metrics, nil
}

func (p *RangeListCountParser) Parse(file io.Reader) ([]Metric, error) {
	var metrics []Metric

	// cpuset.cpus or cpuset.cpus.effective → "cpucore"
	// cpuset.mems or cpuset.mems.effective → "numanode"
	labelName := "cpucore" // Default for CPU-related metrics
	if strings.Contains(p.MetricPrefix, "mems") {
		labelName = "numanode"
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		ranges := strings.Split(line, ",")
		for _, r := range ranges {
			r = strings.TrimSpace(r)
			if strings.Contains(r, "-") {
				bounds := strings.Split(r, "-")
				if len(bounds) != 2 {
					p.Logger.Error("invalid range", "input", r)
					continue
				}

				start, err := strconv.Atoi(bounds[0])
				if err != nil {
					p.Logger.Error("invalid start in range", "input", r, "err", err)
					continue
				}
				end, err := strconv.Atoi(bounds[1])
				if err != nil {
					p.Logger.Error("invalid end in range", "input", r, "err", err)
					continue
				}

				for i := start; i <= end; i++ {
					metrics = append(metrics, Metric{
						Name:   p.MetricPrefix,
						Value:  1,
						Labels: map[string]string{labelName: strconv.Itoa(i)},
					})
				}

			} else {
				val, err := strconv.Atoi(r)
				if err != nil {
					p.Logger.Error("invalid value", "input", r, "err", err)
					continue
				}
				metrics = append(metrics, Metric{
					Name:   p.MetricPrefix,
					Value:  1,
					Labels: map[string]string{labelName: strconv.Itoa(val)},
				})
			}
		}
	}

	if err := scanner.Err(); err != nil {
		p.Logger.Error("scanner error", "err", err)
		return nil, err
	}

	return metrics, nil
}
