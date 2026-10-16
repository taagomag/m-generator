package generator

import (
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/taagomag/m-generator/internal/config"
)

type Metric struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Help    string   `json:"help"`
	Samples []Sample `json:"samples"`
}

type Sample struct {
	Labels    map[string]string `json:"labels"`
	Value     float64           `json:"value"`
	Timestamp int64             `json:"timestamp"`
}

func Generate(cfg config.Config, now time.Time) ([]Metric, error) {
	timestamp := now.UnixMilli()
	metrics := make([]Metric, 0, len(cfg.Metrics))
	for _, metricConfig := range cfg.Metrics {
		value, err := generateValue(metricConfig.Value, now)
		if err != nil {
			return nil, fmt.Errorf("generate metric %q: %w", metricConfig.Name, err)
		}
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("generate metric %q: value is not finite", metricConfig.Name)
		}
		labels := make(map[string]string, len(metricConfig.Labels))
		for _, label := range metricConfig.Labels {
			labels[label.Name] = chooseLabelValue(label)
		}
		metrics = append(metrics, Metric{
			Name: metricConfig.Name,
			Type: metricConfig.Type,
			Help: metricConfig.Help,
			Samples: []Sample{{
				Labels:    labels,
				Value:     value,
				Timestamp: timestamp,
			}},
		})
	}
	return metrics, nil
}

func chooseLabelValue(label config.LabelConfig) string {
	if len(label.Distribution) == 0 {
		return label.Values[rand.Intn(len(label.Values))]
	}
	target := rand.Float64()
	total := 0.0
	for i, weight := range label.Distribution {
		total += weight
		if target < total {
			return label.Values[i]
		}
	}
	return label.Values[len(label.Values)-1]
}

func generateValue(value config.ValueConfig, now time.Time) (float64, error) {
	switch value.Type {
	case "increment":
		if value.Max <= value.Min {
			if value.Min > 0 {
				return value.Min, nil
			}
			return 1, nil
		}
		return value.Min + rand.Float64()*(value.Max-value.Min), nil
	case "random":
		switch value.Distribution {
		case "", "uniform":
			if value.Max <= value.Min {
				return value.Min, nil
			}
			return value.Min + rand.Float64()*(value.Max-value.Min), nil
		case "exponential":
			result := rand.ExpFloat64() * value.Mean
			if value.Max > 0 && result > value.Max {
				return value.Max, nil
			}
			return result, nil
		default:
			return 0, fmt.Errorf("unsupported random distribution %q", value.Distribution)
		}
	case "walk":
		result := value.Initial + value.StepMin + rand.Float64()*(value.StepMax-value.StepMin)
		if value.Min < value.Max {
			result = math.Max(value.Min, math.Min(value.Max, result))
		}
		return result, nil
	case "sine":
		period, err := time.ParseDuration(value.Period)
		if err != nil || period <= 0 {
			return 0, fmt.Errorf("sine period must be a positive duration")
		}
		return value.Offset + value.Amplitude*math.Sin(2*math.Pi*float64(now.UnixNano()%int64(period))/float64(period)), nil
	default:
		return 0, fmt.Errorf("unsupported value type %q", value.Type)
	}
}
