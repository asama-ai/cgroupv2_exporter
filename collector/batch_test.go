package collector

import "testing"

func TestBatchKeepsOneSamplePerLabelSet(t *testing.T) {
	var b Batch
	labels := map[string]string{"cgroup": "asama", "stat": "anon"}
	b.Gauge("cgroupv2_memory_stat", labels, 1)
	b.Gauge("cgroupv2_memory_stat", map[string]string{"stat": "anon", "cgroup": "asama"}, 4)
	b.Gauge("cgroupv2_memory_stat", map[string]string{"cgroup": "other", "stat": "anon"}, 9)

	fams := b.Families()
	if len(fams) != 1 {
		t.Fatalf("families: %d", len(fams))
	}
	if n := len(fams[0].Metric); n != 2 {
		t.Fatalf("samples: got %d want 2", n)
	}
	if v := fams[0].Metric[0].GetGauge().GetValue(); v != 4 {
		t.Fatalf("replaced sample: got %v want 4", v)
	}
	if v := fams[0].Metric[1].GetGauge().GetValue(); v != 9 {
		t.Fatalf("other cgroup: got %v want 9", v)
	}
}
