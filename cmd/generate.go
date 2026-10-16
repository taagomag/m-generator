package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/taagomag/m-generator/internal/config"
	"github.com/taagomag/m-generator/internal/generator"
)

func NewGenerateCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate metrics continuously and write one JSON document",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadConfig(v, cmd)
			if err != nil {
				return err
			}
			timeout, err := cmd.Flags().GetDuration("timeout")
			if err != nil {
				return fmt.Errorf("read timeout flag: %w", err)
			}
			if timeout <= 0 {
				return fmt.Errorf("timeout must be greater than zero")
			}
			batches, err := runGeneration(cmd.Context(), cfg, timeout)
			if err != nil {
				return err
			}
			document := struct {
				Batches [][]generator.Metric `json:"batches"`
			}{Batches: batches}
			data, err := json.MarshalIndent(document, "", "  ")
			if err != nil {
				return fmt.Errorf("encode metrics as JSON: %w", err)
			}
			data = append(data, '\n')
			if err := os.WriteFile(cfg.Output.JSON.File, data, 0o644); err != nil {
				return fmt.Errorf("write JSON output %q: %w", cfg.Output.JSON.File, err)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "Wrote metrics to %s\n", cfg.Output.JSON.File)
			return err
		},
	}
	cmd.Flags().Duration("timeout", time.Minute, "Stop generation after this duration")
	return cmd
}

func runGeneration(ctx context.Context, cfg config.Config, timeout time.Duration) ([][]generator.Metric, error) {
	if timeout <= 0 {
		return nil, fmt.Errorf("timeout must be greater than zero")
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	rate := cfg.Generator.Rate
	if rate == "" {
		rate = "1/s"
	}
	interval, err := config.RateInterval(rate)
	if err != nil {
		return nil, err
	}
	batches := make([][]generator.Metric, 0)
	generateBatch := func() error {
		metrics, err := generator.Generate(cfg, time.Now())
		if err != nil {
			return fmt.Errorf("generate metrics: %w", err)
		}
		batches = append(batches, metrics)
		return nil
	}
	if err := ctx.Err(); err != nil {
		return batches, nil
	}
	if err := generateBatch(); err != nil {
		return nil, err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return batches, nil
		case <-timer.C:
			return batches, nil
		case <-ticker.C:
			if err := generateBatch(); err != nil {
				return nil, err
			}
		}
	}
}
