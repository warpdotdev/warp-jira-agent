package cmd

import (
	"errors"
	"os"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var rootCmd = &cobra.Command{
	Use:   "warp-jira-agent",
	Short: "Driver for running Warp agents in response to Jira events",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		viper.SetEnvPrefix("WARP_JIRA")
		viper.SetEnvKeyReplacer(strings.NewReplacer(".", "*", "-", "*"))
		viper.AutomaticEnv()

		if err := viper.ReadInConfig(); err != nil {
			var notFound viper.ConfigFileNotFoundError
			if !errors.As(err, &notFound) {
				return err
			}
		}

		return viper.BindPFlags(cmd.Flags())
	},
}

func init() {
	rootCmd.PersistentFlags().String("host", "", "URL of your Jira instance")
	rootCmd.PersistentFlags().String("email", "", "Email address of the Jira account to log into")
	rootCmd.PersistentFlags().String("token", "", "API token associated with the Jira account")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Error().Err(err).Send()
		os.Exit(1)
	}
}
