package generator

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/taagomag/m-generator/internal/config"
)

func newTestGenerator(metrics ...config.MetricConfig) *Generator {
	return New(metrics, rand.New(rand.NewPCG(1, 2)))
}

func TestCounterIsCumulativePerSeries(t *testing.T) {
	gen := newTestGenerator(config.MetricConfig{
		Name:   "requests_total",
		Type:   "counter",
		Labels: []config.LabelConfig{{Name: "method", Values: []string{"GET", "POST"}}},
		Value:  config.ValueConfig{Type: "increment", Min: 1, Max: 5},
	})
	last := map[string]float64{}
	for range 100 {
		metrics, err := gen.Next(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		sample := metrics[0].Samples[0]
		method := sample.Labels["method"]
		if delta := sample.Value - last[method]; delta < 1 || delta > 5 {
			t.Fatalf("counter %s increased by %v, want [1, 5]", method, delta)
		}
		last[method] = sample.Value
	}
}

func TestHistogramEmitsCumulativeBuckets(t *testing.T) {
	gen := newTestGenerator(config.MetricConfig{
		Name:    "latency_seconds",
		Type:    "histogram",
		Buckets: []float64{0.1, 0.5},
		Value:   config.ValueConfig{Type: "random", Min: 0, Max: 1},
	})
	var samples []Sample
	for range 50 {
		metrics, err := gen.Next(time.Now())
		if err != nil {
			t.Fatal(err)
		}
		samples = metrics[0].Samples
	}

	wantNames := []string{"latency_seconds_bucket", "latency_seconds_bucket", "latency_seconds_bucket", "latency_seconds_sum", "latency_seconds_count"}
	wantLE := []string{"0.1", "0.5", "+Inf"}
	if len(samples) != len(wantNames) {
		t.Fatalf("got %d samples, want %d", len(samples), len(wantNames))
	}
	for i, name := range wantNames {
		if samples[i].Name != name {
			t.Errorf("samples[%d].Name = %q, want %q", i, samples[i].Name, name)
		}
	}
	for i, le := range wantLE {
		if samples[i].Labels["le"] != le {
			t.Errorf("bucket %d le = %q, want %q", i, samples[i].Labels["le"], le)
		}
		if i > 0 && samples[i].Value < samples[i-1].Value {
			t.Errorf("bucket %s count %v below previous bucket %v", le, samples[i].Value, samples[i-1].Value)
		}
	}
	count := samples[4].Value
	if count != 50 || samples[2].Value != count {
		t.Errorf("count = %v, +Inf bucket = %v, want 50", count, samples[2].Value)
	}
	if sum := samples[3].Value; sum < 0 || sum > count {
		t.Errorf("sum = %v, want within [0, %v]", sum, count)
	}
}
