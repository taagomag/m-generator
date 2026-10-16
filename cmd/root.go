package cmd

import (
	"github.com/spf13/cobra"
)

var version = "0.0.1"

func Execute() error {
	return NewRootCmd().Execute()
}

func NewRootCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "metrics-generator",
		Short:   "Generate synthetic metrics and publish them to NATS",
		Version: version,
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}
