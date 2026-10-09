package collector

import (
	"sort"
	"sync"

	dto "github.com/prometheus/client_model/go"
)

// Batch collects gauge and counter samples grouped by metric name.
type Batch struct {
	mu    sync.Mutex
	order []string
	fams  map[string]*dto.MetricFamily
}

// Gauge appends a gauge sample.
func (b *Batch) Gauge(name string, labels map[string]string, value float64) {
	b.add(name, labels, value, true)
}

// Counter appends a counter sample.
func (b *Batch) Counter(name string, labels map[string]string, value float64) {
	b.add(name, labels, value, false)
}

// Families returns the collected families in insertion order.
func (b *Batch) Families() []*dto.MetricFamily {
	if b == nil {
		return nil
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.order) == 0 {
		return nil
	}
	out := make([]*dto.MetricFamily, 0, len(b.order))
	for _, name := range b.order {
		if mf := b.fams[name]; mf != nil && len(mf.Metric) > 0 {
			out = append(out, mf)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (b *Batch) add(name string, labels map[string]string, value float64, gauge bool) {
	if b == nil || name == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.fams == nil {
		b.fams = map[string]*dto.MetricFamily{}
	}
	mf := b.fams[name]
	if mf == nil {
		n := name
		typ := dto.MetricType_COUNTER
		if gauge {
			typ = dto.MetricType_GAUGE
		}
		mf = &dto.MetricFamily{Name: &n, Type: &typ}
		b.fams[name] = mf
		b.order = append(b.order, name)
	}
	v := value
	m := &dto.Metric{Label: labelPairs(labels)}
	if gauge {
		m.Gauge = &dto.Gauge{Value: &v}
	} else {
		m.Counter = &dto.Counter{Value: &v}
	}
	mf.Metric = append(mf.Metric, m)
}

func labelPairs(labels map[string]string) []*dto.LabelPair {
	if len(labels) == 0 {
		return nil
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]*dto.LabelPair, 0, len(keys))
	for _, k := range keys {
		name, value := k, labels[k]
		out = append(out, &dto.LabelPair{Name: &name, Value: &value})
	}
	return out
}
