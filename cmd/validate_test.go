package cmd

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateCommand(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	configData := `output:
  type: json
  json:
    file: metrics.json
generator:
  rate: 100/s
metrics:
  - name: requests_total
    type: counter
    help: Total requests
    value:
      type: increment
`
	if err := os.WriteFile(configFile, []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}

	output := new(bytes.Buffer)
	root := NewRootCmd()
	root.SetOut(output)
	root.SetErr(output)
	root.SetArgs([]string{"validate", "--config", configFile})

	if err := root.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if !strings.Contains(output.String(), "Configuration is valid") {
		t.Errorf("output = %q, want validation success", output.String())
	}
}

func TestValidateCommandRejectsInvalidConfig(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(configFile, []byte("output: {type: unsupported}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	root := NewRootCmd()
	root.SetArgs([]string{"validate", "--config", configFile})
	err := root.ExecuteContext(t.Context())
	if err == nil || !strings.Contains(err.Error(), "output.type must be json or nats") {
		t.Errorf("ExecuteContext() error = %v, want unsupported output.type error", err)
	}
}

func TestValidateCommandAcceptsNATSConfig(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "nats.yaml")
	configData := `output:
  type: nats
  nats:
    url: nats://localhost:4222
    stream: METRICS
    subject_prefix: metrics
metrics:
  - name: requests_total
    type: counter
    help: Total requests
    value:
      type: increment
`
	if err := os.WriteFile(configFile, []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}
	root := NewRootCmd()
	root.SetArgs([]string{"validate", "--config", configFile})
	if err := root.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
}

func TestGenerateCommandWritesJSONDocumentAndReplacesFile(t *testing.T) {
	dir := t.TempDir()
	outputFile := filepath.Join(dir, "generated.json")
	configFile := filepath.Join(dir, "config.yaml")
	configData := `output:
  type: json
  json:
    file: ` + outputFile + `
metrics:
  - name: requests_total
    type: counter
    help: Total requests
    value:
      type: increment
      min: 1
      max: 5
generator:
  rate: 1000/s
`
	if err := os.WriteFile(configFile, []byte(configData), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outputFile, []byte("old data"), 0o600); err != nil {
		t.Fatal(err)
	}

	output := new(bytes.Buffer)
	root := NewRootCmd()
	root.SetOut(output)
	root.SetErr(output)
	root.SetArgs([]string{"generate", "--config", configFile, "--timeout", "20ms"})

	if err := root.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", outputFile, err)
	}
	var document struct {
		Batches [][]struct {
			Name    string `json:"name"`
			Samples []struct {
				Value float64 `json:"value"`
			} `json:"samples"`
		} `json:"batches"`
	}
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatalf("Unmarshal generated JSON: %v", err)
	}
	if len(document.Batches) < 2 {
		t.Fatalf("generated %d batches, want at least 2", len(document.Batches))
	}
	previous := 0.0
	for _, metrics := range document.Batches {
		if len(metrics) != 1 || metrics[0].Name != "requests_total" {
			t.Fatalf("generated metrics = %+v, want requests_total", metrics)
		}
		if len(metrics[0].Samples) != 1 {
			t.Fatalf("generated samples = %+v, want exactly one", metrics[0].Samples)
		}
		value := metrics[0].Samples[0].Value
		if delta := value - previous; delta < 1 || delta > 5 {
			t.Errorf("counter increased by %v, want increment in [1, 5]", delta)
		}
		previous = value
	}
}
