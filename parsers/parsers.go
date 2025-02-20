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
