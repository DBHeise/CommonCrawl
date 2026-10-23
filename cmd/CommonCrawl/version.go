package main

import (
	"github.com/spf13/cobra"
)

var (
	version   string
	commit    string
	builddate string
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "gets the version",
	Run: func(cmd *cobra.Command, args []string) {
		log.Info().Str("Version", version).Str("Commit", commit).Str("BuildDate", builddate).Msg("Version Information")
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
