package webcache

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

func init() {

	viper.SetDefault("Log.ReportCaller", false)
	viper.SetDefault("Log.Output", "console")
	viper.SetDefault("Log.Level", "trace")

	viper.SetDefault("DataFolder", ".\\data")
	viper.SetDefault("ContentFolderName", "content")

	viper.SetDefault("Index", "CC-MAIN-2026-43")
	viper.SetDefault("RandomizeWarcOrder", false)
	viper.SetDefault("DownloadWarc", true)
	viper.SetDefault("ProcessWarc", true)
	viper.SetDefault("MaxDownloads", -1)

	viper.SetDefault("MimeFilter", []string{"javascript", "css", "html", "svg"})

	viper.SetDefault("BaseUrl", "https://data.commoncrawl.org")
	viper.SetDefault("UserAgent", "CommonCrawl CLI/1.0 (https://github.com/dbheise/commoncrawlcli)")

	viper.SetDefault("MetadataExtention", ".metadata.json")

	viper.SetDefault("WarcPathFormat", "%s/crawl-data/%s/warc.paths.gz")
}

// InitializeConfig - Setup viper & config
func InitializeConfig(cfgFile string) {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {

		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
		viper.AddConfigPath(home)

		currentPath, err := filepath.Abs(".")
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
		viper.AddConfigPath(currentPath)

		parentPath, err := filepath.Abs("..")
		if err != nil {
			fmt.Println(err)
			os.Exit(-1)
		}
		viper.AddConfigPath(parentPath)

		viper.SetConfigName("config")
	}

	viper.SetEnvPrefix("commoncrawl")
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		//fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else {
		panic(err)
	}

}
func GenerateDefaultConfig(outfile string) error {
	return viper.WriteConfigAs(outfile)
}
