package main

//TODO:
// tld/domain/path filter
// ip/iprange/cidr filter
// allow for custom configuration of output saving folder

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
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
	"time"

	warc "github.com/internetarchive/gowarc"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func ContainsAnySub(str string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(str, sub) {
			return true // Short-circuit as soon as a match is found
		}
	}
	return false
}

func URLToLocalPath_Robust(baseDir, rawURL, mime string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	// Hash the entire original URL.
	sum := sha256.Sum256([]byte(rawURL))
	hash := hex.EncodeToString(sum[:])

	// Extract TLD.
	tld := "unknown"

	host := strings.ToLower(u.Hostname())
	if parts := strings.Split(host, "."); len(parts) > 1 {
		tld = parts[len(parts)-1]
	}

	return filepath.Join(
		baseDir,
		sanitizeMime(mime),
		tld,
		hash[0:3],
		hash[3:6],
		hash[6:9],
		hash,
	), nil

}

func URLToLocalPath(baseDir, rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	host := sanitizeComponent(u.Hostname())
	if host == "" {
		return "", fmt.Errorf("url has no hostname")
	}

	// Decode URL path.
	urlPath := u.Path

	// Normalize.
	urlPath = path.Clean("/" + urlPath)

	// Split into path components.
	var dirs []string

	for _, part := range strings.Split(strings.Trim(urlPath, "/"), "/") {
		if part == "" || part == "." || part == ".." {
			continue
		}

		dirs = append(dirs, sanitizeComponent(part))
	}

	// Determine extension.
	ext := filepath.Ext(urlPath)
	if ext == "" {
		ext = ".html"
	}

	// Use URL hash as filename.
	sum := sha256.Sum256([]byte(rawURL))
	hash := hex.EncodeToString(sum[:8])

	filename := hash + ext

	parts := []string{baseDir, host}
	parts = append(parts, dirs...)

	// Replace final path component with hashed filename.
	if len(dirs) > 0 {
		parts = parts[:len(parts)-1]
	}

	fullPath := filepath.Join(parts...)
	fullPath = filepath.Join(fullPath, filename)

	// Verify no escape from baseDir.
	baseAbs, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}

	pathAbs, err := filepath.Abs(fullPath)
	if err != nil {
		return "", err
	}

	rel, err := filepath.Rel(baseAbs, pathAbs)
	if err != nil {
		return "", err
	}

	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes base directory")
	}

	return pathAbs, nil
}

func sanitizeMime(mime string) string {
	if strings.ContainsRune(mime, ';') {
		mime = strings.Split(mime, ";")[0]
	}
	return sanitizeComponent(mime)
}

func sanitizeComponent(s string) string {
	s = strings.TrimSpace(s)

	// Windows-invalid characters.
	replacer := strings.NewReplacer(
		"<", "_",
		">", "_",
		":", "_",
		"\"", "_",
		"/", "_",
		"\\", "_",
		"|", "_",
		"?", "_",
		"*", "_",
	)

	s = replacer.Replace(s)

	// Remove control chars.
	var b strings.Builder
	for _, r := range s {
		if r < 32 {
			continue
		}
		b.WriteRune(r)
	}

	s = b.String()

	// Windows doesn't like trailing dots/spaces.
	s = strings.TrimRight(s, ". ")

	if s == "" {
		s = "_"
	}

	return s
}

func DownloadPage(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", viper.GetString("UserAgent"))

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("request failed with status %s", resp.Status)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return content, nil
}

func DownloadToFile(url, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	client := &http.Client{Timeout: 30 * time.Second}

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", viper.GetString("UserAgent"))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	return file.Sync()
}

func DecompressGZIP(data []byte) ([]byte, error) {
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("open gzip data: %w", err)
	}
	defer reader.Close()

	uncompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("decompress gzip data: %w", err)
	}

	return uncompressed, nil
}

func DownloadAndUnGZip(url string) ([]byte, error) {
	compressed, err := DownloadPage(url)
	if err != nil {
		log.Error().Err(err).Msg("download failed")
		return nil, err
	}

	decompressed, err := DecompressGZIP(compressed)
	if err != nil {
		log.Error().Err(err).Msg("decompression failed")
		return nil, err
	}

	return decompressed, nil
}

func FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

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

		if ContainsAnySub(mime, viper.GetStringSlice("MimeFilter")) {

			//Convert url to local path
			local, err := URLToLocalPath_Robust(job.output_folder, targetURL, mime)
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

			//TODO: handle local file already exists
			//	have to read in metadata and merge??

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

		exists, err := FileExists(local_file)
		if err != nil {
			log.Error().Int("DLWorker", id).Err(err).Msg("could not check if file exists")
			continue
		}

		if !exists && job.download {
			err = DownloadToFile(job.url, local_file)
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

	decompressed, err := DownloadAndUnGZip(warc_url)
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
