package CommonCrawl

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestNewHttpClient(t *testing.T) {
	client := NewHttpClient()

	assert.NotNil(t, client)
	assert.Equal(t, 30*time.Second, client.Timeout)
}

func TestNewHttpRequest(t *testing.T) {
	const method = http.MethodPost
	const requestURL = "https://example.com/resource"
	const viperConfigUserAgent = "Http.UserAgent"

	previousUserAgent := viper.GetString(viperConfigUserAgent)
	viper.Set(viperConfigUserAgent, "test-agent")
	t.Cleanup(func() {
		viper.Set(viperConfigUserAgent, previousUserAgent)
	})

	client, req := NewHttpRequest(method, requestURL)

	assert.NotNil(t, client)
	if assert.NotNil(t, req) {
		assert.Equal(t, method, req.Method)
		assert.Equal(t, requestURL, req.URL.String())
		assert.Equal(t, "test-agent", req.Header.Get("User-Agent"))
	}
}

func TestURLToLocalPath(t *testing.T) {
	const savePathFormatKey = "SavePathFormat"
	const tldCacheFileKey = "TLDCacheFile"

	previousSavePathFormat := viper.GetString(savePathFormatKey)
	previousTLDCacheFile := viper.GetString(tldCacheFileKey)
	previousExtractor := extractor
	t.Cleanup(func() {
		viper.Set(savePathFormatKey, previousSavePathFormat)
		viper.Set(tldCacheFileKey, previousTLDCacheFile)
		extractor = previousExtractor
	})

	tldCacheFile := filepath.Join(t.TempDir(), "tld.cache")
	if err := os.WriteFile(tldCacheFile, []byte("com\norg\nnet\n"), 0600); err != nil {
		t.Fatalf("write TLD cache: %v", err)
	}
	viper.Set(tldCacheFileKey, tldCacheFile)
	extractor = nil

	tests := []struct {
		name    string
		format  string
		rawURL  string
		headers map[string]string
		baseDir string
		want    string
		wantErr string
	}{
		{
			name:    "expand domain and URL path macros",
			format:  ":tld.domainroot:/:tld.tld:/:url.hostname::url.path:",
			rawURL:  "https://www.example.com/articles/a%3Fb.html",
			baseDir: "downloads",
			want:    filepath.Join("downloads", "example", "com", "www.example.com", "articles", "a_b.html"),
		},
		{
			name:    "expand and sanitize header and URL macros",
			format:  ":header.x-test:/:url.scheme:/:url.host:",
			rawURL:  "https://example.com:8443/page",
			headers: map[string]string{"x-test": "release:v1"},
			baseDir: "downloads",
			want:    filepath.Join("downloads", "release_v1", "https", "example.com_8443"),
		},
		{
			name:    "sanitize an unexpanded missing header macro",
			format:  ":header.missing:/:url.hostname:",
			rawURL:  "https://example.com/page",
			headers: map[string]string{"x-test": "release:v1"},
			baseDir: "downloads",
			want:    filepath.Join("downloads", "_header.missing_", "example.com"),
		},
		{
			name:    "sanitize an empty header macro",
			format:  ":header.empty:/:url.hostname:",
			rawURL:  "https://example.com/page",
			headers: map[string]string{"empty": ""},
			baseDir: "downloads",
			want:    filepath.Join("downloads", "example.com"),
		},
		{
			name:    "normalize parent components in the URL path",
			format:  ":url.hostname::url.path:",
			rawURL:  "https://example.com/section/../private",
			baseDir: "downloads",
			want:    filepath.Join("downloads", "example.com", "private"),
		},
		{
			name:    "reject an empty format and base directory",
			format:  "",
			rawURL:  "https://example.com/page",
			baseDir: "",
			wantErr: "empty path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			viper.Set(savePathFormatKey, tt.format)

			got, err := URLToLocalPath(tt.baseDir, tt.rawURL, tt.headers)
			if tt.wantErr != "" {
				if assert.EqualError(t, err, tt.wantErr) {
					assert.Empty(t, got)
				}
				return
			}

			if assert.NoError(t, err) {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestDownloadToFile(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantErr    string
	}{
		{
			name:       "download and create parent directories",
			statusCode: http.StatusOK,
			body:       "file content",
		},
		{
			name:       "reject non-OK success status",
			statusCode: http.StatusCreated,
			body:       "not written",
			wantErr:    "http error: 201 Created",
		},
		{
			name:       "reject server error",
			statusCode: http.StatusInternalServerError,
			body:       "not written",
			wantErr:    "http error: 500 Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()

			filePath := filepath.Join(t.TempDir(), "nested", "page.txt")
			err := DownloadToFile(http.MethodGet, server.URL, filePath)
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.EqualError(t, err, tt.wantErr)
				_, statErr := os.Stat(filePath)
				assert.ErrorIs(t, statErr, os.ErrNotExist)
				return
			}

			if assert.NoError(t, err) {
				content, readErr := os.ReadFile(filePath)
				assert.NoError(t, readErr)
				assert.Equal(t, tt.body, string(content))
			}
		})
	}

	t.Run("fail when parent path is a file", func(t *testing.T) {
		parentFile := filepath.Join(t.TempDir(), "parent")
		assert.NoError(t, os.WriteFile(parentFile, []byte("not a directory"), 0600))

		err := DownloadToFile(http.MethodGet, "http://example.com", filepath.Join(parentFile, "page.txt"))
		assert.Error(t, err)
	})
}

func TestDecompressGZIP(t *testing.T) {
	validGZIP := gzipData(t, []byte("decompressed content"))
	emptyGZIP := gzipData(t, nil)

	tests := []struct {
		name    string
		data    []byte
		want    []byte
		wantErr string
	}{
		{
			name: "decompress content",
			data: validGZIP,
			want: []byte("decompressed content"),
		},
		{
			name: "decompress empty content",
			data: emptyGZIP,
			want: []byte{},
		},
		{
			name:    "reject invalid gzip header",
			data:    []byte("not gzip"),
			wantErr: "open gzip data",
		},
		{
			name:    "reject truncated gzip stream",
			data:    validGZIP[:len(validGZIP)-4],
			wantErr: "decompress gzip data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DecompressGZIP(tt.data)
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, got)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func gzipData(t *testing.T, data []byte) []byte {
	t.Helper()

	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(data); err != nil {
		t.Fatalf("write gzip test data: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close gzip test data: %v", err)
	}
	return compressed.Bytes()
}
