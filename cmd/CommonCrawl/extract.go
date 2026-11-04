package main

//TODO:
// tld/domain/path filter
// ip/iprange/cidr filter
// allow for custom configuration of output saving folder

import (
	"CommonCrawl"
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	warc "github.com/internetarchive/gowarc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type processJob struct {
	record        *warc.Record
	output_folder string
}

func process_worker(id int, job processJob, outQ chan string) {
	record := job.record
	warcType := record.Header.Get("WARC-Type")
	targetURL := record.Header.Get("WARC-Target-URI")
	if warcType != "response" {
		return
	}

	log.Info().Int("P Worker", id).Str("url", targetURL).Msg("processing")

	// Parse the embedded HTTP response.
	resp, err := http.ReadResponse(
		bufio.NewReader(record.Content),
		nil,
	)
	if err != nil {
		log.Error().Int("P Worker", id).Err(err).Msg("could not extract response")
		return
	}

	mime := strings.ToLower(resp.Header.Get("Content-Type"))

	if CommonCrawl.ContainsAnySub(mime, viper.GetStringSlice("Extract.MimeFilter")) {

		//Convert url to local path
		local, err := CommonCrawl.URLToLocalPath_Robust(job.output_folder, targetURL, mime)
		if err != nil {
			log.Error().Int("P Worker", id).Err(err).Msg("could not extract local path")
			return
		}
		local_metadata := local + viper.GetString("MetadataExtension")

		// ensure local path is ready
		if err := os.MkdirAll(filepath.Dir(local), 0755); err != nil {
			log.Error().Int("P Worker", id).Err(err).Msg("could not create local path")
			return
		}

		exists, err := CommonCrawl.FileExists(local)
		if err != nil {
			log.Error().Int("PWorker", id).Err(err).Msg("could not check if file exists")
			return
		}
		if !exists {
			// create local file
			file, err := os.Create(local)
			if err != nil {
				log.Error().Int("P Worker", id).Err(err).Msg("could not create local file")
				return
			}
			defer file.Close()

			//write body to local file
			_, err = io.Copy(file, resp.Body)
			if err != nil {
				log.Error().Int("P Worker", id).Err(err).Msg("could not stream to disk")
				return
			}

			//write metadata to file
			data, err := json.MarshalIndent(record.Header, "", "  ")
			if err != nil {
				log.Error().Int("P Worker", id).Err(err).Msg("could not marshal metadata")
				return
			}
			if err := os.WriteFile(local_metadata, data, 0644); err != nil {
				log.Error().Int("P Worker", id).Err(err).Msg("could not write metadata to disk")
				return
			}
		} else {
			//TODO: do we want to merge the metadata or no?
			log.Warn().Int("P Worker", id).Str("File", local).Msg("File already exists")
		}
	}

}

func ProcessLocalWarc(local string, output_folder string, workQueue chan processJob) error {
	f, err := os.Open(local)
	if err != nil {
		return err
	}
	defer f.Close()

	reader, err := warc.NewReader(f)
	if err != nil {
		return err
	}

	for {
		record, err := reader.ReadRecord()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		work := processJob{
			record:        record,
			output_folder: output_folder,
		}
		workQueue <- work

	}
	return nil
}

type downloaderJob struct {
	url            string
	output_folder  string
	content_folder string
	download       bool
	process        bool
}

func download_worker(id int, job downloaderJob, workQ chan processJob) {
	log.Info().Int("DLWorker", id).Str("url", job.url).Msg("Processing")

	u, err := url.Parse(job.url)
	if err != nil {
		log.Error().Int("DLWorker", id).Err(err).Msg("could not parse url")
		return
	}
	filename := path.Base(u.Path)

	local_file := filepath.Join(job.output_folder, filename)

	exists, err := CommonCrawl.FileExists(local_file)
	if err != nil {
		log.Error().Int("DLWorker", id).Err(err).Msg("could not check if file exists")
		return
	}

	if !exists && job.download {
		var httpErr *CommonCrawl.HTTPError
		err = CommonCrawl.DownloadToFile(http.MethodGet, job.url, local_file)
		if err != nil {
			if errors.As(err, &httpErr) {
				log.Error().Int("DLWorker", id).Str("status", httpErr.Status).Str("body", httpErr.Body).Msg("could not download WARC")
			} else {
				log.Error().Int("DLWorker", id).Err(err).Msg("could not download WARC")
			}
			return
		}
		exists = true
	}

	if job.process && exists {
		err = ProcessLocalWarc(local_file, job.content_folder, workQ)
		if err != nil {
			log.Error().Int("DLWorker", id).Err(err).Msg("could not process WARC")
			return

		}
	}
}

func extract() {

	index := viper.GetString("CCIndex")
	baseUrl := viper.GetString("BaseUrl")
	output_folder := filepath.Join(viper.GetString("DataFolder"), index)
	if err := os.MkdirAll(filepath.Dir(output_folder), 0755); err != nil {
		log.Fatal().Err(err).Msg("Could not create output folder")
	}
	warc_cache := filepath.Join(output_folder, viper.GetString("Extract.WarcCacheFolderName"))
	if err := os.MkdirAll(filepath.Dir(warc_cache), 0755); err != nil {
		log.Fatal().Err(err).Msg("Could not create warc cache folder")
	}
	content_output := filepath.Join(output_folder, viper.GetString("ContentFolderName"))
	if err := os.MkdirAll(filepath.Dir(content_output), 0755); err != nil {
		log.Fatal().Err(err).Msg("Could not create content folder")
	}

	warc_url := fmt.Sprintf(viper.GetString("WarcPathFormat"), baseUrl, index)

	decompressed, err := CommonCrawl.DownloadAndUnGZip(http.MethodGet, warc_url)
	if err != nil {
		var httpErr *CommonCrawl.HTTPError
		if errors.As(err, &httpErr) {
			log.Error().Str("status", httpErr.Status).Str("body", httpErr.Body).Msg("could not download/ungzip")
		} else {
			log.Fatal().Err(err).Msg("could not download/ungzip")
		}
	}

	lines := strings.Split(string(decompressed), "\n")
	max_downloads := viper.GetInt("Extract.MaxDownloads")
	downloadWarc := viper.GetBool("Extract.DownloadWarc")
	processWarc := viper.GetBool("Extract.ProcessWarc")

	if viper.GetBool("Extract.RandomizeWarcOrder") {
		rand.Shuffle(len(lines), func(i, j int) {
			lines[i], lines[j] = lines[j], lines[i]
		})
	}

	dlPool := CommonCrawl.NewChainedWorker(
		viper.GetInt("Extract.DownloadWorkers"),
		download_worker,
	)
	procPool := CommonCrawl.NewChainedWorkerFrom(
		dlPool.Output(),
		viper.GetInt("Extract.ProcessWorkers"),
		process_worker,
	)

	// Enqueue Jobs
	count := 0
	for _, line := range lines {
		count++
		if count > max_downloads && max_downloads > 0 {
			break
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		dl := downloaderJob{
			url:            fmt.Sprintf("%s/%s", baseUrl, line),
			output_folder:  warc_cache,
			content_folder: content_output,
			download:       downloadWarc,
			process:        processWarc,
		}
		dlPool.Enqueue(dl)
	}

	dlPool.CloseAndWait()
	<-procPool.Done()
}

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extracts files from CommonCrawl",
	Run: func(cmd *cobra.Command, args []string) {
		extract()
	},
}

func init() {

	extractCmd.Flags().Bool("shuffle", false, "Shuffle WARC download/processing order")
	_ = viper.BindPFlag("Extract.RandomizeWarcOrder", extractCmd.Flags().Lookup("shuffle"))

	extractCmd.Flags().Bool("download", true, "Download WARC files")
	_ = viper.BindPFlag("Extract.DownloadWarc", extractCmd.Flags().Lookup("download"))

	extractCmd.Flags().Bool("process", true, "Process WARC files")
	_ = viper.BindPFlag("Extract.ProcessWarc", extractCmd.Flags().Lookup("process"))

	extractCmd.Flags().Int("max", -1, "Maximum number of WARC files to download/process")
	_ = viper.BindPFlag("Extract.MaxDownloads", extractCmd.Flags().Lookup("max"))

	rootCmd.AddCommand(extractCmd)
}
