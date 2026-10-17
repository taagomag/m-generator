package generator

import (
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/taagomag/m-generator/internal/config"
)

var DefaultBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10}

type Metric struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Help    string   `json:"help"`
	Samples []Sample `json:"samples"`
}

type Sample struct {
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels"`
	Value     float64           `json:"value"`
	Timestamp int64             `json:"timestamp"`
}

// Generator produces batches of metrics and keeps per-series state so that
// counters and histograms accumulate across batches.
type Generator struct {
	metrics []config.MetricConfig
	buckets [][]float64
	rng     *rand.Rand
	series  map[string]*series
}

type series struct {
	value   float64
	buckets []uint64
	count   uint64
	sum     float64
}

func New(metrics []config.MetricConfig, rng *rand.Rand) *Generator {
	buckets := make([][]float64, len(metrics))
	for i, metric := range metrics {
		if metric.Type == "histogram" {
			buckets[i] = metric.Buckets
			if len(buckets[i]) == 0 {
				buckets[i] = DefaultBuckets
			}
		}
	}
	return &Generator{
		metrics: metrics,
		buckets: buckets,
		rng:     rng,
		series:  make(map[string]*series),
	}
}

// Next returns one sample per configured metric (or one bucket/sum/count set
// for histograms) for a randomly selected label combination.
func (g *Generator) Next(now time.Time) ([]Metric, error) {
	timestamp := now.UnixMilli()
	metrics := make([]Metric, 0, len(g.metrics))
	for i, metricConfig := range g.metrics {
		labels, key := g.chooseLabels(metricConfig)
		state := g.seriesFor(key, len(g.buckets[i]))
		var samples []Sample
		switch metricConfig.Type {
		case "counter":
			state.value += g.increment(metricConfig.Value)
			samples = []Sample{{Name: metricConfig.Name, Labels: labels, Value: state.value, Timestamp: timestamp}}
		case "gauge":
			samples = []Sample{{Name: metricConfig.Name, Labels: labels, Value: g.random(metricConfig.Value), Timestamp: timestamp}}
		case "histogram":
			state.observe(g.random(metricConfig.Value), g.buckets[i])
			samples = histogramSamples(metricConfig.Name, labels, state, g.buckets[i], timestamp)
		default:
			return nil, fmt.Errorf("generate metric %q: unsupported type %q", metricConfig.Name, metricConfig.Type)
		}
		metrics = append(metrics, Metric{
			Name:    metricConfig.Name,
			Type:    metricConfig.Type,
			Help:    metricConfig.Help,
			Samples: samples,
		})
	}
	return metrics, nil
}

func (g *Generator) seriesFor(key string, bucketCount int) *series {
	state, ok := g.series[key]
	if !ok {
		state = &series{buckets: make([]uint64, bucketCount)}
		g.series[key] = state
	}
	return state
}

func (g *Generator) chooseLabels(metric config.MetricConfig) (map[string]string, string) {
	labels := make(map[string]string, len(metric.Labels))
	var key strings.Builder
	key.WriteString(metric.Name)
	for _, label := range metric.Labels {
		value := g.chooseLabelValue(label)
		labels[label.Name] = value
		key.WriteByte(0xff)
		key.WriteString(value)
	}
	return labels, key.String()
}

func (g *Generator) chooseLabelValue(label config.LabelConfig) string {
	if len(label.Distribution) == 0 {
		return label.Values[g.rng.IntN(len(label.Values))]
	}
	target := g.rng.Float64()
	total := 0.0
	for i, weight := range label.Distribution {
		total += weight
		if target < total {
			return label.Values[i]
		}
	}
	return label.Values[len(label.Values)-1]
}

func (g *Generator) increment(value config.ValueConfig) float64 {
	if value.Max <= value.Min && value.Min <= 0 {
		return 1
	}
	return g.random(value)
}

func (g *Generator) random(value config.ValueConfig) float64 {
	if value.Max <= value.Min {
		return value.Min
	}
	return value.Min + g.rng.Float64()*(value.Max-value.Min)
}

func (s *series) observe(value float64, bounds []float64) {
	s.count++
	s.sum += value
	for i, bound := range bounds {
		if value <= bound {
			s.buckets[i]++
		}
	}
}

func histogramSamples(name string, labels map[string]string, state *series, bounds []float64, timestamp int64) []Sample {
	samples := make([]Sample, 0, len(bounds)+3)
	addBucket := func(le string, count uint64) {
		bucketLabels := make(map[string]string, len(labels)+1)
		for k, v := range labels {
			bucketLabels[k] = v
		}
		bucketLabels["le"] = le
		samples = append(samples, Sample{Name: name + "_bucket", Labels: bucketLabels, Value: float64(count), Timestamp: timestamp})
	}
	for i, bound := range bounds {
		addBucket(strconv.FormatFloat(bound, 'g', -1, 64), state.buckets[i])
	}
	addBucket("+Inf", state.count)
	return append(samples,
		Sample{Name: name + "_sum", Labels: labels, Value: state.sum, Timestamp: timestamp},
		Sample{Name: name + "_count", Labels: labels, Value: float64(state.count), Timestamp: timestamp},
	)
}
