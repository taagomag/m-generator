package config

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Output    OutputConfig    `mapstructure:"output"`
	Generator GeneratorConfig `mapstructure:"generator"`
	Metrics   []MetricConfig  `mapstructure:"metrics"`
}

type OutputConfig struct {
	Type string     `mapstructure:"type"`
	JSON JSONConfig `mapstructure:"json"`
}

type JSONConfig struct {
	File string `mapstructure:"file"`
}

type GeneratorConfig struct {
	Rate string `mapstructure:"rate"`
}

type MetricConfig struct {
	Name   string        `mapstructure:"name"`
	Type   string        `mapstructure:"type"`
	Help   string        `mapstructure:"help"`
	Labels []LabelConfig `mapstructure:"labels"`
	Value  ValueConfig   `mapstructure:"value"`
}

type LabelConfig struct {
	Name         string    `mapstructure:"name"`
	Values       []string  `mapstructure:"values"`
	Distribution []float64 `mapstructure:"distribution"`
}

type ValueConfig struct {
	Type         string  `mapstructure:"type"`
	Distribution string  `mapstructure:"distribution"`
	Min          float64 `mapstructure:"min"`
	Max          float64 `mapstructure:"max"`
	Mean         float64 `mapstructure:"mean"`
	Initial      float64 `mapstructure:"initial"`
	StepMin      float64 `mapstructure:"step_min"`
	StepMax      float64 `mapstructure:"step_max"`
	Period       string  `mapstructure:"period"`
	Amplitude    float64 `mapstructure:"amplitude"`
	Offset       float64 `mapstructure:"offset"`
}

var (
	metricNamePattern = regexp.MustCompile(`^[a-zA-Z_:][a-zA-Z0-9_:]*$`)
	labelNamePattern  = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
)

func (c Config) Validate() error {
	if c.Output.Type != "json" {
		return fmt.Errorf("output.type must be json")
	}
	if strings.TrimSpace(c.Output.JSON.File) == "" {
		return fmt.Errorf("output.json.file is required")
	}

	if c.Generator.Rate != "" {
		if err := ValidateRate(c.Generator.Rate); err != nil {
			return err
		}
	}
	if len(c.Metrics) == 0 {
		return fmt.Errorf("metrics must contain at least one metric")
	}

	metricNames := make(map[string]struct{}, len(c.Metrics))
	for i, metric := range c.Metrics {
		if !metricNamePattern.MatchString(metric.Name) {
			return fmt.Errorf("metrics[%d].name is not a valid metric name", i)
		}
		if _, exists := metricNames[metric.Name]; exists {
			return fmt.Errorf("metrics[%d].name %q is duplicated", i, metric.Name)
		}
		metricNames[metric.Name] = struct{}{}
		switch metric.Type {
		case "counter", "gauge", "histogram":
		default:
			return fmt.Errorf("metrics[%d].type must be counter, gauge, or histogram", i)
		}
		if strings.TrimSpace(metric.Help) == "" {
			return fmt.Errorf("metrics[%d].help is required", i)
		}
		if err := validateLabels(i, metric.Labels); err != nil {
			return err
		}
		switch metric.Value.Type {
		case "increment", "random", "walk", "sine":
		default:
			return fmt.Errorf("metrics[%d].value.type must be increment, random, walk, or sine", i)
		}
		if err := validateValue(i, metric.Value); err != nil {
			return err
		}
	}
	return nil
}

func validateValue(metricIndex int, value ValueConfig) error {
	switch value.Type {
	case "increment":
		if value.Min < 0 {
			return fmt.Errorf("metrics[%d].value.min must not be negative for increment values", metricIndex)
		}
		if value.Max < value.Min {
			return fmt.Errorf("metrics[%d].value.max must be greater than or equal to min", metricIndex)
		}
	case "random":
		switch value.Distribution {
		case "", "uniform":
			if value.Max < value.Min {
				return fmt.Errorf("metrics[%d].value.max must be greater than or equal to min", metricIndex)
			}
		case "exponential":
			if value.Mean <= 0 {
				return fmt.Errorf("metrics[%d].value.mean must be greater than zero for exponential distribution", metricIndex)
			}
			if value.Max < 0 {
				return fmt.Errorf("metrics[%d].value.max must not be negative", metricIndex)
			}
		default:
			return fmt.Errorf("metrics[%d].value.distribution must be uniform or exponential", metricIndex)
		}
	case "walk":
		if value.StepMax < value.StepMin {
			return fmt.Errorf("metrics[%d].value.step_max must be greater than or equal to step_min", metricIndex)
		}
		if value.Min > value.Max && value.Max != 0 {
			return fmt.Errorf("metrics[%d].value.max must be greater than or equal to min", metricIndex)
		}
	case "sine":
		period, err := time.ParseDuration(value.Period)
		if err != nil || period <= 0 {
			return fmt.Errorf("metrics[%d].value.period must be a positive duration", metricIndex)
		}
	}
	for _, number := range []float64{value.Min, value.Max, value.Mean, value.Initial, value.StepMin, value.StepMax, value.Amplitude, value.Offset} {
		if math.IsNaN(number) || math.IsInf(number, 0) {
			return fmt.Errorf("metrics[%d].value numbers must be finite", metricIndex)
		}
	}
	return nil
}

func ValidateRate(rate string) error {
	_, err := RateInterval(rate)
	return err
}

func RateInterval(rate string) (time.Duration, error) {
	parts := strings.Split(rate, "/")
	if len(parts) != 2 || (parts[1] != "s" && parts[1] != "m") {
		return 0, fmt.Errorf("generator.rate must use the form <positive integer>/s or <positive integer>/m")
	}
	count, err := strconv.Atoi(parts[0])
	if err != nil || count <= 0 {
		return 0, fmt.Errorf("generator.rate must use the form <positive integer>/s or <positive integer>/m")
	}
	unit := time.Second
	if parts[1] == "m" {
		unit = time.Minute
	}
	interval := unit / time.Duration(count)
	if interval <= 0 {
		return 0, fmt.Errorf("generator.rate is too high to represent")
	}
	return interval, nil
}

func validateLabels(metricIndex int, labels []LabelConfig) error {
	labelNames := make(map[string]struct{}, len(labels))
	for labelIndex, label := range labels {
		if !labelNamePattern.MatchString(label.Name) || strings.HasPrefix(label.Name, "__") {
			return fmt.Errorf("metrics[%d].labels[%d].name is not a valid label name", metricIndex, labelIndex)
		}
		if _, exists := labelNames[label.Name]; exists {
			return fmt.Errorf("metrics[%d].labels[%d].name %q is duplicated", metricIndex, labelIndex, label.Name)
		}
		labelNames[label.Name] = struct{}{}
		if len(label.Values) == 0 {
			return fmt.Errorf("metrics[%d].labels[%d].values must not be empty", metricIndex, labelIndex)
		}
		if len(label.Distribution) > 0 {
			if len(label.Distribution) != len(label.Values) {
				return fmt.Errorf("metrics[%d].labels[%d].distribution must match values length", metricIndex, labelIndex)
			}
			total := 0.0
			for _, weight := range label.Distribution {
				if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 {
					return fmt.Errorf("metrics[%d].labels[%d].distribution weights must not be negative", metricIndex, labelIndex)
				}
				total += weight
			}
			if total < 0.999 || total > 1.001 {
				return fmt.Errorf("metrics[%d].labels[%d].distribution weights must sum to 1", metricIndex, labelIndex)
			}
		}
	}
	return nil
}
