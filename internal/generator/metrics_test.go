package generator

import (
	"testing"
	"time"

	"github.com/taagomag/m-generator/internal/config"
)

func TestGenerateCreatesOneSamplePerMetric(t *testing.T) {
	now := time.Unix(1_700_000_000, 123_000_000)
	cfg := config.Config{
		Metrics: []config.MetricConfig{
			{
				Name: "requests_total",
				Type: "counter",
				Help: "Total requests",
				Labels: []config.LabelConfig{{
					Name:   "method",
					Values: []string{"GET"},
				}},
				Value: config.ValueConfig{
					Type: "increment",
					Min:  1,
					Max:  5,
				},
			},
		},
	}

	metrics, err := Generate(cfg, now)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if len(metrics) != 1 || len(metrics[0].Samples) != 1 {
		t.Fatalf("metrics = %+v, want one metric with one sample", metrics)
	}
	sample := metrics[0].Samples[0]
	if sample.Timestamp != now.UnixMilli() {
		t.Errorf("sample timestamp = %d, want %d", sample.Timestamp, now.UnixMilli())
	}
	if sample.Labels["method"] != "GET" {
		t.Errorf("method label = %q, want GET", sample.Labels["method"])
	}
	if sample.Value < 1 || sample.Value > 5 {
		t.Errorf("sample value = %v, want value in [1, 5]", sample.Value)
	}
}
