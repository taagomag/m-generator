package cmd

import (
	"context"
	"runtime/debug"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var version = "dev"

func Version() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return version
}

func ExecuteContext(ctx context.Context) error {
	return NewRootCmd().ExecuteContext(ctx)
}

func NewRootCmd() *cobra.Command {
	v := viper.New()
	rootCmd := &cobra.Command{
		Use:     "metrics-generator",
		Short:   "Generate synthetic metrics and write them to an output",
		Version: Version(),
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
