package CommonCrawl

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
	viper.SetDefault("Log.Level", "info")

	viper.SetDefault("DataFolder", ".\\data")
	viper.SetDefault("ContentFolderName", "content")

	viper.SetDefault("CCIndex", "CC-MAIN-2026-43")
	viper.SetDefault("BaseUrl", "https://data.commoncrawl.org")
	viper.SetDefault("MetadataExtention", ".metadata.json")
	viper.SetDefault("TLDCacheFile", "./tld.cache")

	viper.SetDefault("Http.UserAgent", "CommonCrawl CLI/1.0 (https://github.com/dbheise/commoncrawlcli)")
	viper.SetDefault("Http.ErrorBodyLimit", 8192)
	viper.SetDefault("Http.DefaultTimeoutSeconds", 30)

	viper.SetDefault("MacroChar", ":")
	viper.SetDefault("SavePathFormat", ":header.warc-identified-payload-type:\\:tld.tld:\\:urlhash.3_1:\\:urlhash.3_2:\\:url.hash:")

	viper.SetDefault("WarcPathFormat", "%s/crawl-data/%s/warc.paths.gz")
	viper.SetDefault("Extract.WarcCacheFolderName", "warc_cache")
	viper.SetDefault("Extract.RandomizeWarcOrder", false)
	viper.SetDefault("Extract.DownloadWarc", true)
	viper.SetDefault("Extract.ProcessWarc", true)
	viper.SetDefault("Extract.MaxDownloads", -1)
	viper.SetDefault("Extract.DownloadWorkers", 3)
	viper.SetDefault("Extract.ProcessWorkers", 6)
	viper.SetDefault("Extract.MimeFilter", []string{""})

	viper.SetDefault("IndexPathFormat", "%s/crawl-data/%s/cc-index.paths.gz")
	viper.SetDefault("Index.CacheFolderName", "index_cache")
	viper.SetDefault("Index.ReprocessExisting", false)
	viper.SetDefault("Index.FailedFile", "failed.jsonl")
	viper.SetDefault("Index.BulkInserters", 8)
	viper.SetDefault("Index.OpenSearchBaseUrl", "http://localhost:9200")
	viper.SetDefault("Index.InsertionIndex", "commoncrawl")
	viper.SetDefault("Index.FieldNames.CCIndex", "ccindex")
	viper.SetDefault("Index.FieldNames.Tld", "tld")
	viper.SetDefault("Index.FieldNames.DomainRoot", "domainRoot")
	viper.SetDefault("Index.FieldNames.SubDomain", "subdomain")
	viper.SetDefault("Index.FieldNames.Host", "host")
	viper.SetDefault("Index.FieldNames.Path", "path")

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
