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
  type: stdout
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
	if err == nil || !strings.Contains(err.Error(), "output.type must be stdout or nats") {
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

func TestGenerateCommandWritesOneJSONLinePerBatchToStdout(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	configData := `output:
  type: stdout
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

	output := new(bytes.Buffer)
	root := NewRootCmd()
	root.SetOut(output)
	root.SetErr(output)
	root.SetArgs([]string{"generate", "--config", configFile, "--timeout", "20ms"})

	if err := root.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) < 2 {
		t.Fatalf("generated %d batches, want at least 2", len(lines))
	}
	previous := 0.0
	for _, line := range lines {
		var metrics []struct {
			Name    string `json:"name"`
			Samples []struct {
				Value float64 `json:"value"`
			} `json:"samples"`
		}
		if err := json.Unmarshal([]byte(line), &metrics); err != nil {
			t.Fatalf("Unmarshal line %q: %v", line, err)
		}
		if len(metrics) != 1 || metrics[0].Name != "requests_total" || len(metrics[0].Samples) != 1 {
			t.Fatalf("generated metrics = %+v, want one requests_total sample", metrics)
		}
		value := metrics[0].Samples[0].Value
		if delta := value - previous; delta < 1 || delta > 5 {
			t.Errorf("counter increased by %v, want increment in [1, 5]", delta)
		}
		previous = value
	}
}

func TestGenerateDefaultsToStdoutAndRateFlagOverridesConfig(t *testing.T) {
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	configData := `generator:
  rate: 1/m
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
	root.SetArgs([]string{"generate", "--config", configFile, "--timeout", "50ms", "--rate", "1000/s"})

	if err := root.ExecuteContext(t.Context()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
	if lines := strings.Count(output.String(), "\n"); lines < 2 {
		t.Errorf("generated %d batches on stdout, want at least 2 (--rate should override 1/m)", lines)
	}
}
