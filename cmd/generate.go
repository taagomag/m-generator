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
	"github.com/taagomag/m-generator/internal/publisher"
)

func NewGenerateCmd(v *viper.Viper) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate metrics continuously and send them to the configured output",
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
			switch cfg.Output.Type {
			case "json":
				return generateJSON(cmd, cfg, timeout)
			case "nats":
				return generateNATS(cmd, cfg, timeout)
			default:
				return fmt.Errorf("unsupported output type %q", cfg.Output.Type)
			}
		},
	}
	cmd.Flags().Duration("timeout", time.Minute, "Stop generation after this duration")
	return cmd
}

func generateJSON(cmd *cobra.Command, cfg config.Config, timeout time.Duration) error {
	batches := make([][]generator.Metric, 0)
	err := runGeneration(cmd.Context(), cfg, timeout, func(metrics []generator.Metric) error {
		batches = append(batches, metrics)
		return nil
	})
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
}

func generateNATS(cmd *cobra.Command, cfg config.Config, timeout time.Duration) error {
	if err := cmd.Context().Err(); err != nil {
		return nil
	}
	natsPublisher, err := publisher.NewNATSPublisher(cmd.Context(), cfg.Output.NATS)
	if err != nil {
		return err
	}
	defer natsPublisher.Close()
	if err := runGeneration(cmd.Context(), cfg, timeout, func(metrics []generator.Metric) error {
		return natsPublisher.Publish(cmd.Context(), metrics)
	}); err != nil {
		return err
	}
	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Published metrics to NATS stream %s\n", cfg.Output.NATS.Stream)
	return err
}

func runGeneration(ctx context.Context, cfg config.Config, timeout time.Duration, publish func([]generator.Metric) error) error {
	if timeout <= 0 {
		return fmt.Errorf("timeout must be greater than zero")
	}
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	rate := cfg.Generator.Rate
	if rate == "" {
		rate = "1/s"
	}
	interval, err := config.RateInterval(rate)
	if err != nil {
		return err
	}
	generateBatch := func() error {
		metrics, err := generator.Generate(cfg, time.Now())
		if err != nil {
			return fmt.Errorf("generate metrics: %w", err)
		}
		if err := publish(metrics); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return fmt.Errorf("publish generated metrics: %w", err)
		}
		return nil
	}
	if err := ctx.Err(); err != nil {
		return nil
	}
	if err := generateBatch(); err != nil {
		return err
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-timer.C:
			return nil
		case <-ticker.C:
			if err := generateBatch(); err != nil {
				return err
			}
		}
	}
}
