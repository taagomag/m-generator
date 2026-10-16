package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var version = "0.0.1"

func ExecuteContext(ctx context.Context) error {
	return NewRootCmd().ExecuteContext(ctx)
}

func NewRootCmd() *cobra.Command {
	v := viper.New()
	rootCmd := &cobra.Command{
		Use:     "metrics-generator",
		Short:   "Generate synthetic metrics and write them to an output",
		Version: version,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	rootCmd.PersistentFlags().StringP("config", "c", "config.yaml", "config file path")
	rootCmd.AddCommand(NewGenerateCmd(v))
	rootCmd.AddCommand(NewValidateCmd(v))
	return rootCmd
}
