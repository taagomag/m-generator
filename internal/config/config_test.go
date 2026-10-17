package config

import (
	"strings"
	"testing"
	"time"
)

func validConfig() Config {
	return Config{
		Output: OutputConfig{
			Type: "json",
			JSON: JSONConfig{
				File: "metrics.json",
			},
		},
		Generator: GeneratorConfig{
			Rate: "100/s",
		},
		Metrics: []MetricConfig{{
			Name: "requests_total",
			Type: "counter",
			Help: "Total requests",
			Value: ValueConfig{
				Type: "increment",
			},
		}},
	}
}

func TestConfigValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantErr string
	}{
		{name: "valid"},
		{
			name: "unsupported output type",
			mutate: func(c *Config) {
				c.Output.Type = "unsupported"
			},
			wantErr: "output.type must be json or nats",
		},
		{
			name: "missing JSON file path",
			mutate: func(c *Config) {
				c.Output.JSON.File = ""
			},
			wantErr: "output.json.file is required",
		},
		{
			name: "invalid rate",
			mutate: func(c *Config) {
				c.Generator.Rate = "0/s"
			},
			wantErr: "generator.rate must use the form",
		},
		{
			name: "empty metrics",
			mutate: func(c *Config) {
				c.Metrics = nil
			},
			wantErr: "metrics must contain at least one metric",
		},
		{
			name: "invalid label distribution",
			mutate: func(c *Config) {
				c.Metrics[0].Labels = []LabelConfig{{
					Name:         "method",
					Values:       []string{"GET", "POST"},
					Distribution: []float64{0.5, 0.25},
				}}
			},
			wantErr: "distribution weights must sum to 1",
		},
		{
			name: "counter without increment",
			mutate: func(c *Config) {
				c.Metrics[0].Value = ValueConfig{Type: "random"}
			},
			wantErr: "counters must use value.type increment",
		},
		{
			name: "unsorted histogram buckets",
			mutate: func(c *Config) {
				c.Metrics[0].Type = "histogram"
				c.Metrics[0].Value = ValueConfig{Type: "random", Max: 1}
				c.Metrics[0].Buckets = []float64{0.5, 0.1}
			},
			wantErr: "buckets must be strictly increasing",
		},
		{
			name: "reserved le label on histogram",
			mutate: func(c *Config) {
				c.Metrics[0].Type = "histogram"
				c.Metrics[0].Value = ValueConfig{Type: "random", Max: 1}
				c.Metrics[0].Labels = []LabelConfig{{Name: "le", Values: []string{"x"}}}
			},
			wantErr: `label name "le" is reserved`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			if tt.mutate != nil {
				tt.mutate(&cfg)
			}
			err := cfg.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestNATSOutputValidate(t *testing.T) {
	cfg := validConfig()
	cfg.Output = OutputConfig{
		Type: "nats",
		NATS: NATSConfig{
			URL:           "nats://localhost:4222",
			Stream:        "METRICS",
			SubjectPrefix: "metrics",
		},
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}

	tests := []struct {
		name    string
		mutate  func(*NATSConfig)
		wantErr string
	}{
		{
			name: "invalid URL",
			mutate: func(c *NATSConfig) {
				c.URL = "http://localhost:4222"
			},
			wantErr: "output.nats.url must be a valid nats:// URL",
		},
		{
			name: "missing stream",
			mutate: func(c *NATSConfig) {
				c.Stream = ""
			},
			wantErr: "output.nats.stream is required",
		},
		{
			name: "invalid subject prefix",
			mutate: func(c *NATSConfig) {
				c.SubjectPrefix = "metrics..data"
			},
			wantErr: "output.nats.subject_prefix",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testConfig := cfg
			tt.mutate(&testConfig.Output.NATS)
			err := testConfig.Validate()
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate() error = %v, want substring %q", err, tt.wantErr)
			}
		})
	}
}

func TestRateInterval(t *testing.T) {
	tests := []struct {
		rate string
		want time.Duration
	}{
		{rate: "2/s", want: 500 * time.Millisecond},
		{rate: "4/m", want: 15 * time.Second},
	}
	for _, tt := range tests {
		t.Run(tt.rate, func(t *testing.T) {
			got, err := RateInterval(tt.rate)
			if err != nil {
				t.Fatalf("RateInterval() error = %v", err)
			}
			if got != tt.want {
				t.Errorf("RateInterval() = %s, want %s", got, tt.want)
			}
		})
	}
}
