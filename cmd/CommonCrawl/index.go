package main

import (
	"CommonCrawl"
	"CommonCrawl/OpenSearch"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/joeguo/tldextract"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var extractor *tldextract.TLDExtract

func processIndex(ccindex, local string, q chan map[string]string) {
	log.Info().Str("file", local).Msg("processing local index file")

	data, err := os.ReadFile(local)
	if err != nil {
		log.Error().Err(err).Str("data", local).Msg("could not read file")
		return
	}
	data, err = CommonCrawl.DecompressGZIP(data)
	if err != nil {
		log.Error().Err(err).Str("data", local).Msg("could not decompress data")
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		startIdx := strings.IndexAny(line, "{")
		if startIdx >= 0 {
			jsonBlob := line[startIdx:]
			itemData := make(map[string]string)
			json.Unmarshal([]byte(jsonBlob), &itemData)

			u := itemData["url"]
			itemData[viper.GetString("Index.FieldNames.CCIndex")] = ccindex
			tldInfo := extractor.Extract(u)
			itemData[viper.GetString("Index.FieldNames.Tld")] = tldInfo.Tld
			itemData[viper.GetString("Index.FieldNames.DomainRoot")] = tldInfo.Root
			itemData[viper.GetString("Index.FieldNames.SubDomain")] = tldInfo.Sub
			u2, _ := url.Parse(u)
			itemData[viper.GetString("Index.FieldNames.Host")] = u2.Host
			itemData[viper.GetString("Index.FieldNames.Path")] = u2.Path

			q <- itemData
		}
	}

}

func buildLocalIndex(q chan map[string]string) {
	index := viper.GetString("CCIndex")
	reprocess := viper.GetBool("Index.ReprocessExisting")

	log.Info().Str("Index", index).Msg("building local index")

	baseUrl := viper.GetString("BaseUrl")
	output_folder := filepath.Join(viper.GetString("DataFolder"), index, viper.GetString("Index.CacheFolderName"))
	if err := os.MkdirAll(filepath.Dir(output_folder), 0755); err != nil {
		log.Fatal().Err(err).Msg("Could not create output folder")
	}

	index_url := fmt.Sprintf(viper.GetString("IndexPathFormat"), baseUrl, index)
	log.Info().Str("Url", index_url).Msg("gathering index index")
	index_content, err := CommonCrawl.DownloadAndUnGZip(http.MethodGet, index_url)
	if err != nil {
		var httpErr *CommonCrawl.HTTPError
		if errors.As(err, &httpErr) {
			log.Error().Str("status", httpErr.Status).Str("body", httpErr.Body).Msg("could not download index of index")
		} else {
			log.Fatal().Err(err).Msg("Could not download index of index")
		}
	}
	index_list := strings.Split(string(index_content), "\n")

	for _, item := range index_list {
		index_url := fmt.Sprintf("%s/%s", baseUrl, item)
		u, err := url.Parse(index_url)
		if err != nil {
			log.Error().Err(err).Msg("could not parse index url")
			continue
		}

		filename := path.Base(u.Path)
		index_local := filepath.Join(output_folder, filename)
		exists, err := CommonCrawl.FileExists(index_local)
		if err != nil {
			log.Error().Err(err).Msg("could not check if file exists")
			continue
		}
		downloaded := false
		if !exists {
			log.Info().Str("local", index_local).Msg("Downloading")
			CommonCrawl.DownloadToFile(http.MethodGet, index_url, index_local)
			err = CommonCrawl.DownloadToFile(http.MethodGet, index_url, index_local)
			if err != nil {
				var httpErr *CommonCrawl.HTTPError
				if errors.As(err, &httpErr) {
					log.Error().Str("status", httpErr.Status).Str("body", httpErr.Body).Msg("could not download WARC")
				} else {
					log.Error().Err(err).Msg("could not download WARC")
				}
				downloaded = false
			}
			downloaded = true
		}
		exists, err = CommonCrawl.FileExists(index_local)
		if err != nil {
			log.Error().Err(err).Msg("could not check if file exists")
			continue
		}
		if exists && (downloaded || reprocess) {
			processIndex(index, index_local, q)
		}
	}

}

func index() {

	workers := viper.GetInt("Index.BulkInserters")
	index_name := viper.GetString("Index.InsertionIndex")
	extr, err := tldextract.New(viper.GetString("TLDCacheFile"), false)
	if err != nil {
		log.Fatal().Err(err).Msg("could not create TLDExtract cache")
	}
	extractor = extr

	docQ := make(chan map[string]string)
	client := OpenSearch.Client{
		BaseURL: viper.GetString("Index.OpenSearchBaseUrl"),
		Client:  &http.Client{Timeout: 2 * time.Minute},
	}

	index := viper.GetString("CCIndex")
	output_folder := filepath.Join(viper.GetString("DataFolder"), index)
	if err := os.MkdirAll(filepath.Dir(output_folder), 0755); err != nil {
		log.Fatal().Err(err).Msg("Could not create output folder")
	}

	go func() {
		defer close(docQ)
		buildLocalIndex(docQ)
	}()

	stats := &OpenSearch.BulkStats{}
	deadLetter, err := os.Create(viper.GetString("Index.FailedFile"))
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create dead letter queue file")
	}
	defer deadLetter.Close()

	handler := func(e OpenSearch.BulkItemError) {

		record := map[string]any{
			"status":   e.Status,
			"type":     e.Type,
			"reason":   e.Reason,
			"document": e.Document,
		}

		b, _ := json.Marshal(record)

		deadLetter.Write(b)
		deadLetter.Write([]byte("\n"))
	}

	go func() {

		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			s := stats.Snapshot()
			log.Debug().Uint64("indexed", s.Indexed).Uint64("failed", s.Failed).Uint64("batches", s.Batches).Msg("snapshot")
		}
	}()

	err = client.BulkInsert(
		context.Background(),
		index_name, // alias
		docQ,
		workers, // workers
		stats,
		handler,
	)
	if err != nil {
		log.Error().Err(err).Msg("failure to insert")
	}
}

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "build local index",
	Run: func(cmd *cobra.Command, args []string) {
		index()
	},
}

func init() {
	indexCmd.Flags().Bool("reprocess", false, "reprocess existing gz files")
	_ = viper.BindPFlag("Index.ReprocessExisting", indexCmd.Flags().Lookup("reprocess"))

	rootCmd.AddCommand(indexCmd)
}
