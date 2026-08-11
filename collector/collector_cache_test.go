package collector

import (
	"log/slog"
	"testing"
)

func TestNewCgroupv2CollectorAllReusesOnlyIdenticalCgroups(t *testing.T) {
	a, err := NewCgroupv2CollectorAll([]string{"/tmp/cgroup-a"}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewCgroupv2CollectorAll([]string{"/tmp/cgroup-b"}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	aAgain, err := NewCgroupv2CollectorAll([]string{"/tmp/cgroup-a"}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	aReordered, err := NewCgroupv2CollectorAll([]string{"/tmp/cgroup-z", "/tmp/cgroup-a"}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	aReorderedSame, err := NewCgroupv2CollectorAll([]string{"/tmp/cgroup-a", "/tmp/cgroup-z"}, slog.Default())
	if err != nil {
		t.Fatal(err)
	}

	if len(a.Collectors) == 0 {
		t.Fatal("expected registered collectors")
	}
	for name, ca := range a.Collectors {
		if ca == b.Collectors[name] {
			t.Fatalf("collector %q reused across different cgroup dirs", name)
		}
		if ca != aAgain.Collectors[name] {
			t.Fatalf("collector %q not reused for identical cgroup dirs", name)
		}
		if aReordered.Collectors[name] != aReorderedSame.Collectors[name] {
			t.Fatalf("collector %q not reused for same cgroup set in different order", name)
		}
	}
}
