package CommonCrawl

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

type HTTPError struct {
	StatusCode int
	Status     string
	Body       string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("http error: %s", e.Status)
}

func NewHTTPError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, viper.GetInt64("Http.ErrorBodyLimit")))

	return &HTTPError{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Body:       string(body),
	}
}

func NewHttpClient() *http.Client {
	return &http.Client{
		Timeout: time.Duration(viper.GetInt("Http.DefaultTimeoutSeconds")) * time.Second,
	}
}

func NewHttpRequest(method, url string) (*http.Client, *http.Request) {
	client := NewHttpClient()
	req, _ := http.NewRequest(method, url, nil)
	req.Header.Set("User-Agent", viper.GetString("Http.UserAgent"))
	return client, req
}

func URLToLocalPath(baseDir, rawURL string, headers map[string]string) (string, error) {
	macros := BuildMacroMap(rawURL, headers)
	format := viper.GetString("SavePathFormat")
	return SanitizePath(filepath.Join(baseDir, ApplyMacros(format, macros)), '_')

}

func DownloadToFile(method, url, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	client, req := NewHttpRequest(method, url)

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return NewHTTPError(resp)
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
