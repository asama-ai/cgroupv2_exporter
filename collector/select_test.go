package collector

import (
	"log/slog"
	"testing"
)

func TestNewCgroupv2CollectorSelectMemoryStat(t *testing.T) {
	health, err := NewCgroupv2CollectorSelect(nil, slog.Default(), func(name string) bool {
		return name != "memory.stat"
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := health.Collectors["memory.stat"]; ok {
		t.Fatal("health must not include memory.stat")
	}
	if _, ok := health.Collectors["memory.current"]; !ok {
		t.Fatal("health must include memory.current")
	}

	analysis, err := NewCgroupv2CollectorSelect(nil, slog.Default(), func(name string) bool {
		return name == "memory.stat"
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(analysis.Collectors) != 1 {
		t.Fatalf("analysis collectors: %v", analysis.Collectors)
	}
	if _, ok := analysis.Collectors["memory.stat"]; !ok {
		t.Fatal("analysis must include memory.stat")
	}
}
