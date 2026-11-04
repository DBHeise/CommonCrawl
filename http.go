package CommonCrawl

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
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

	//TODO: make this customizable
	return filepath.Join(
		baseDir,
		SanitizeMime(mime),
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

	host := SanitizeComponent(u.Hostname())
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

		dirs = append(dirs, SanitizeComponent(part))
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

func DownloadPage(method, url string) ([]byte, error) {
	client, req := NewHttpRequest(method, url)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, NewHTTPError(resp)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return content, nil
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

func DownloadAndUnGZip(method, url string) ([]byte, error) {
	compressed, err := DownloadPage(method, url)
	if err != nil {
		return nil, err
	}

	decompressed, err := DecompressGZIP(compressed)
	if err != nil {
		return nil, err
	}

	return decompressed, nil
}
