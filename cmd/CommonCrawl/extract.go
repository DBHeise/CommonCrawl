package main

//TODO:
// tld/domain/path filter
// ip/iprange/cidr filter
// allow for custom configuration of output saving folder

import (
	"CommonCrawl"
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	warc "github.com/internetarchive/gowarc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type processJob struct {
	record        *warc.Record
	output_folder string
}

func process_worker(id int, work <-chan processJob, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range work {
		record := job.record
		warcType := record.Header.Get("WARC-Type")
		targetURL := record.Header.Get("WARC-Target-URI")
		if warcType != "response" {
			continue
		}

		log.Info().Int("P Worker", id).Str("url", targetURL).Msg("processing")

		// Parse the embedded HTTP response.
		resp, err := http.ReadResponse(
			bufio.NewReader(record.Content),
			nil,
		)
		if err != nil {
			log.Error().Int("P Worker", id).Err(err).Msg("could not extract response")
			continue
		}

		mime := strings.ToLower(resp.Header.Get("Content-Type"))

		if CommonCrawl.ContainsAnySub(mime, viper.GetStringSlice("MimeFilter")) {

			//Convert url to local path
			local, err := CommonCrawl.URLToLocalPath_Robust(job.output_folder, targetURL, mime)
			if err != nil {
				log.Error().Int("P Worker", id).Err(err).Msg("could not extract local path")
				continue
			}
			local_metadata := local + viper.GetString("MetadataExtension")

			// ensure local path is ready
			if err := os.MkdirAll(filepath.Dir(local), 0755); err != nil {
				log.Error().Int("P Worker", id).Err(err).Msg("could not create local path")
				continue
			}

			exists, err := CommonCrawl.FileExists(local)
			if err != nil {
				log.Error().Int("PWorker", id).Err(err).Msg("could not check if file exists")
				continue
			}
			if !exists {
				// create local file
				file, err := os.Create(local)
				if err != nil {
					log.Error().Int("P Worker", id).Err(err).Msg("could not create local file")
					continue
				}
				defer file.Close()

				//write body to local file
				_, err = io.Copy(file, resp.Body)
				if err != nil {
					log.Error().Int("P Worker", id).Err(err).Msg("could not stream to disk")
					continue
				}

				//write metadata to file
				data, err := json.MarshalIndent(record.Header, "", "  ")
				if err != nil {
					log.Error().Int("P Worker", id).Err(err).Msg("could not marshal metadata")
					continue
				}
				if err := os.WriteFile(local_metadata, data, 0644); err != nil {
					log.Error().Int("P Worker", id).Err(err).Msg("could not write metadata to disk")
					continue
				}
			} else {
				//TODO: do we want to merge the metadata or no?
				log.Warn().Int("P Worker", id).Str("File", local).Msg("File already exists")
			}
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

func download_worker(id int, jobs <-chan downloaderJob, work chan processJob, wg *sync.WaitGroup) {
	defer wg.Done()
	for job := range jobs {

		log.Info().Int("DLWorker", id).Str("url", job.url).Msg("Processing")

		u, err := url.Parse(job.url)
		if err != nil {
			log.Error().Int("DLWorker", id).Err(err).Msg("could not parse url")
			continue
		}
		filename := path.Base(u.Path)

		local_file := filepath.Join(job.output_folder, filename)

		exists, err := CommonCrawl.FileExists(local_file)
		if err != nil {
			log.Error().Int("DLWorker", id).Err(err).Msg("could not check if file exists")
			continue
		}

		if !exists && job.download {
			err = CommonCrawl.DownloadToFile(job.url, local_file)
			if err != nil {
				log.Error().Int("DLWorker", id).Err(err).Msg("could not download WARC")
				continue
			}
			exists = true
		}

		if job.process && exists {
			err = ProcessLocalWarc(local_file, job.content_folder, work)
			if err != nil {
				log.Error().Int("DLWorker", id).Err(err).Msg("could not process WARC")
				continue
			}
		}
	}
}

func extract() {

	index := viper.GetString("Index")
	baseUrl := viper.GetString("BaseUrl")
	output_folder := filepath.Join(viper.GetString("DataFolder"), index)
	if err := os.MkdirAll(filepath.Dir(output_folder), 0755); err != nil {
		log.Fatal().Err(err).Msg("Could not create output folder")
	}
	data_output := filepath.Join(output_folder, viper.GetString("ContentFolderName"))
	if err := os.MkdirAll(filepath.Dir(output_folder), 0755); err != nil {
		log.Fatal().Err(err).Msg("Could not create content folder")
	}

	warc_url := fmt.Sprintf(viper.GetString("WarcPathFormat"), baseUrl, index)

	decompressed, err := CommonCrawl.DownloadAndUnGZip(warc_url)
	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	lines := strings.Split(string(decompressed), "\n")
	max_downloads := viper.GetInt("MaxDownloads")
	downloadWarc := viper.GetBool("DownloadWarc")
	processWarc := viper.GetBool("ProcessWarc")

	if viper.GetBool("RandomizeWarcOrder") {
		rand.Shuffle(len(lines), func(i, j int) {
			lines[i], lines[j] = lines[j], lines[i]
		})
	}

	numJobs := len(lines)
	if max_downloads > 0 {
		numJobs = max_downloads
	}
	var wgJobs sync.WaitGroup
	var wgWork sync.WaitGroup

	jobs := make(chan downloaderJob, numJobs)
	workQ := make(chan processJob)

	// Start Workers
	numWorkers := viper.GetInt("DownloadWorkers")
	for w := 1; w <= numWorkers; w++ {
		wgJobs.Add(1)
		go download_worker(w, jobs, workQ, &wgJobs)
	}
	numWorkers = viper.GetInt("ProcessWorkers")
	for p := 1; p <= numWorkers; p++ {
		wgWork.Add(1)
		go process_worker(p, workQ, &wgWork)
	}

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
			output_folder:  output_folder,
			content_folder: data_output,
			download:       downloadWarc,
			process:        processWarc,
		}
		jobs <- dl
	}

	// Close jobs channel
	close(jobs)
	// Wait for jobs to finish
	wgJobs.Wait()

	// Close the work channel
	close(workQ)
	// Wait for the work to finish
	wgWork.Wait()
}

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extracts files from CommonCrawl",
	Run: func(cmd *cobra.Command, args []string) {
		extract()
	},
}

func init() {

	extractCmd.Flags().String("index", "CC-MAIN-2026-43", "CommonCrawl Index to use")
	_ = viper.BindPFlag("Index", extractCmd.Flags().Lookup("index"))

	extractCmd.Flags().String("data", ".\\data", "Local folder to store data")
	_ = viper.BindPFlag("DataFolder", extractCmd.Flags().Lookup("data"))

	extractCmd.Flags().Bool("shuffle", false, "Shuffle WARC download/processing order")
	_ = viper.BindPFlag("RandomizeWarcOrder", extractCmd.Flags().Lookup("shuffle"))

	extractCmd.Flags().Bool("download", true, "Download WARC files")
	_ = viper.BindPFlag("DownloadWarc", extractCmd.Flags().Lookup("download"))

	extractCmd.Flags().Bool("process", true, "Process WARC files")
	_ = viper.BindPFlag("ProcessWarc", extractCmd.Flags().Lookup("process"))

	extractCmd.Flags().Int("max", -1, "Maximum number of WARC files to download/process")
	_ = viper.BindPFlag("MaxDownloads", extractCmd.Flags().Lookup("max"))

	rootCmd.AddCommand(extractCmd)
}
