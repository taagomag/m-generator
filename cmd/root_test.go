package cmd

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestRootCommand(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "help",
			args: []string{"--help"},
			want: "Usage:",
		},
		{
			name: "version",
			args: []string{"--version"},
			want: "metrics-generator version 0.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := new(bytes.Buffer)
			root := NewRootCmd()
			root.SetOut(output)
			root.SetErr(output)
			root.SetArgs(tt.args)

			if err := root.ExecuteContext(t.Context()); err != nil {
				t.Fatalf("ExecuteContext() error = %v", err)
			}
			if !strings.Contains(output.String(), tt.want) {
				t.Errorf("output = %q, want it to contain %q", output.String(), tt.want)
			}
		})
	}
}

func TestGenerateTimeoutDefaultsToOneMinute(t *testing.T) {
	root := NewRootCmd()
	generate, _, err := root.Find([]string{"generate"})
	if err != nil {
		t.Fatalf("Find(generate) error = %v", err)
	}
	timeout, err := generate.Flags().GetDuration("timeout")
	if err != nil {
		t.Fatalf("GetDuration(timeout) error = %v", err)
	}
	if timeout != time.Minute {
		t.Errorf("timeout = %s, want %s", timeout, time.Minute)
	}
}
