package main

import (
	"CommonCrawl"
	"os"

	"github.com/spf13/cobra"
)

var (
	cfgFile string
)

var rootCmd = &cobra.Command{
	Use:   "webcache",
	Short: "webcache Service",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Error().Err(err).Msg("An Unhandled Error Occured!")
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/config.json)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	CommonCrawl.InitializeConfig(cfgFile)
	log = CommonCrawl.InitializeLogger("CommonCrawl")
}
