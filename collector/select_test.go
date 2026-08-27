package collector

import (
	"log/slog"
	"strings"
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

func TestNewCgroupv2CollectorSelectNSRejectsInvalidNamespace(t *testing.T) {
	keepNone := func(string) bool { return false }
	for _, ns := range []string{"virt-cgroup", "2virt", "foo bar", "virt/cgroup"} {
		_, err := NewCgroupv2CollectorSelectNS(nil, slog.Default(), keepNone, SelectOpts{Namespace: ns, Uncached: true})
		if err == nil {
			t.Fatalf("namespace %q: want error", ns)
		}
		if !strings.Contains(err.Error(), "invalid metric namespace") {
			t.Fatalf("namespace %q: %v", ns, err)
		}
	}
}

func TestNewCgroupv2CollectorSelectNSAcceptsValidNamespace(t *testing.T) {
	keepNone := func(string) bool { return false }
	for _, ns := range []string{"", "cgroupv2", "virt", "container"} {
		c, err := NewCgroupv2CollectorSelectNS(nil, slog.Default(), keepNone, SelectOpts{Namespace: ns, Uncached: true})
		if err != nil {
			t.Fatalf("namespace %q: %v", ns, err)
		}
		want := ns
		if want == "" {
			want = namespace
		}
		if c.namespace != want {
			t.Fatalf("namespace %q: got %q", ns, c.namespace)
		}
	}
}
