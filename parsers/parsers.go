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
	Parse(io.Reader) (map[string]float64, error)
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

func (p *SingleValueParser) Parse(file io.Reader) (map[string]float64, error) {
	content, err := readContent(file)
	if err != nil {
		p.Logger.Error("error reading file", "err", err)
		return nil, err
	}
	// Check if content is "max" and convert it to +Inf
	if content == "max" {
		p.Logger.Debug("converting max to +Inf")
		return map[string]float64{p.MetricPrefix: math.Inf(1)}, nil
	}

	value, err := strconv.ParseFloat(content, 64)
	if err != nil {
		p.Logger.Error("failed to parse value", "err", err)
		return nil, err
	}
	return map[string]float64{p.MetricPrefix: value}, nil
}

func (p *FlatKeyValueParser) Parse(file io.Reader) (map[string]float64, error) {
	metrics := map[string]float64{}

	// Read the file line by line and parse PSI statistics
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) != 2 {
			p.Logger.Error("invalid field count", "expected", 2, "got", len(parts))
			continue
		}
		metricName := fmt.Sprintf("%s_%s", p.MetricPrefix, parts[0])
		metrics[metricName], _ = strconv.ParseFloat(parts[1], 64)
	}

	if err := scanner.Err(); err != nil {
		p.Logger.Error("scanner error", "err", err)
		return nil, err
	}

	return metrics, nil
}

func (p *NestedKeyValueParser) Parse(file io.Reader) (map[string]float64, error) {
	metrics := map[string]float64{}

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
			metricName := fmt.Sprintf("%s_%s_%s", p.MetricPrefix, prefix, metric[0])
			metrics[metricName], _ = strconv.ParseFloat(metric[1], 64)
		}
	}

	if err := scanner.Err(); err != nil {
		p.Logger.Error("scanner error", "err", err)
		return nil, err
	}

	return metrics, nil
}

func (p *RangeListCountParser) Parse(file io.Reader) (map[string]float64, error) {
	metrics := map[string]float64{}

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
					// Use base metric name with label value as separator for collector to parse
					// Format: "base_metric_name|label_value" (label name will be detected in collector)
					metricName := p.MetricPrefix + "|" + strconv.Itoa(i)
					metrics[metricName] = 1
				}

			} else {
				val, err := strconv.Atoi(r)
				if err != nil {
					p.Logger.Error("invalid value", "input", r, "err", err)
					continue
				}
				// Use base metric name with label value as separator for collector to parse
				// Format: "base_metric_name|label_value" (label name will be detected in collector)
				metricName := p.MetricPrefix + "|" + strconv.Itoa(val)
				metrics[metricName] = 1
			}
		}
	}

	if err := scanner.Err(); err != nil {
		p.Logger.Error("scanner error", "err", err)
		return nil, err
	}

	return metrics, nil
}
