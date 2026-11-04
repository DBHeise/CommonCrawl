package main

import (
	"CommonCrawl"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
)

var rootCmd = &cobra.Command{
	Use:   "commoncrawl",
	Short: "CommonCrawl Helper",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Error().Err(err).Msg("An Unhandled Error Occured!")
		os.Exit(-1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootFlags := rootCmd.PersistentFlags()

	rootFlags.String("index", "CC-MAIN-2026-43", "CommonCrawl Index to use")
	_ = viper.BindPFlag("CCIndex", rootFlags.Lookup("index"))

	rootFlags.String("data", ".\\data", "Local folder to store data")
	_ = viper.BindPFlag("DataFolder", rootFlags.Lookup("data"))

	rootFlags.StringVar(&cfgFile, "config", "", "config file (default is $HOME/config.json)")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	CommonCrawl.InitializeConfig(cfgFile)
	log = CommonCrawl.InitializeLogger("CommonCrawl")
}
