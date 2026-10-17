package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
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
			if err := v.BindPFlag("generator.rate", cmd.Flags().Lookup("rate")); err != nil {
				return fmt.Errorf("bind rate flag: %w", err)
			}
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
			case "stdout":
				return generateStdout(cmd, cfg, timeout)
			case "nats":
				return generateNATS(cmd, cfg, timeout)
			default:
				return fmt.Errorf("unsupported output type %q", cfg.Output.Type)
			}
		},
	}
	cmd.Flags().Duration("timeout", time.Minute, "Stop generation after this duration")
	cmd.Flags().String("rate", config.DefaultRate, "Batches per second or minute, e.g. 100/s; overrides generator.rate")
	return cmd
}

func generateStdout(cmd *cobra.Command, cfg config.Config, timeout time.Duration) error {
	encoder := json.NewEncoder(cmd.OutOrStdout())
	return runGeneration(cmd.Context(), cfg, timeout, func(metrics []generator.Metric) error {
		return encoder.Encode(metrics)
	})
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
	interval, err := config.RateInterval(cfg.Generator.Rate)
	if err != nil {
		return err
	}
	gen := generator.New(cfg.Metrics, rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())))
	generateBatch := func() error {
		metrics, err := gen.Next(time.Now())
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
