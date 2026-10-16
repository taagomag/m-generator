package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/taagomag/m-generator/internal/config"
)

func NewValidateCmd(v *viper.Viper) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the configuration file",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := loadConfig(v, cmd); err != nil {
				return err
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Configuration is valid: %s\n", v.ConfigFileUsed()); err != nil {
				return err
			}
			return nil
		},
	}
}

func loadConfig(v *viper.Viper, cmd *cobra.Command) (config.Config, error) {
	if err := v.BindPFlags(cmd.InheritedFlags()); err != nil {
		return config.Config{}, fmt.Errorf("bind config flag: %w", err)
	}
	v.SetConfigFile(v.GetString("config"))
	if err := v.ReadInConfig(); err != nil {
		return config.Config{}, fmt.Errorf("read config file: %w", err)
	}
	var cfg config.Config
	if err := v.Unmarshal(&cfg); err != nil {
		return config.Config{}, fmt.Errorf("decode config file: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return config.Config{}, fmt.Errorf("validate config file: %w", err)
	}
	return cfg, nil
}
