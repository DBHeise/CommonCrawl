package CommonCrawl

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewCache(t *testing.T) {
	tests := []struct {
		name        string
		files       []string
		directories []string
		wantFiles   []string
	}{
		{
			name:      "start with an empty folder",
			wantFiles: []string{},
		},
		{
			name:        "load files and ignore subdirectories",
			files:       []string{"page.html", "archive.gz"},
			directories: []string{"nested"},
			wantFiles:   []string{"page.html", "archive.gz"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			folder := t.TempDir()
			for _, name := range tt.files {
				if err := os.WriteFile(filepath.Join(folder, name), []byte(name), 0600); err != nil {
					t.Fatalf("create cache file: %v", err)
				}
			}
			for _, name := range tt.directories {
				if err := os.Mkdir(filepath.Join(folder, name), 0700); err != nil {
					t.Fatalf("create cache directory: %v", err)
				}
			}

			cache := NewCache(folder)
			if assert.NotNil(t, cache) {
				assert.Equal(t, stringSet(tt.wantFiles...), cache.cache)
			}
		})
	}
}

func TestCacheRebuild(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(t *testing.T, folder string, cache *Cache)
		wantFiles map[string]struct{}
		wantErr   bool
	}{
		{
			name: "refresh files and skip directories",
			setup: func(t *testing.T, folder string, _ *Cache) {
				if err := os.WriteFile(filepath.Join(folder, "new.txt"), []byte("new"), 0600); err != nil {
					t.Fatalf("create file: %v", err)
				}
				if err := os.Mkdir(filepath.Join(folder, "nested"), 0700); err != nil {
					t.Fatalf("create directory: %v", err)
				}
			},
			wantFiles: stringSet("new.txt"),
		},
		{
			name: "replace the old cache contents",
			setup: func(t *testing.T, folder string, cache *Cache) {
				if err := os.WriteFile(filepath.Join(folder, "replacement.txt"), []byte("new"), 0600); err != nil {
					t.Fatalf("create file: %v", err)
				}
				cache.cache["stale.txt"] = struct{}{}
			},
			wantFiles: stringSet("replacement.txt"),
		},
		{
			name: "return an error when the cache folder is a file",
			setup: func(t *testing.T, _ string, cache *Cache) {
				file := filepath.Join(t.TempDir(), "not-a-folder")
				if err := os.WriteFile(file, []byte("file"), 0600); err != nil {
					t.Fatalf("create file: %v", err)
				}
				cache.localFolder = file
				cache.cache["existing.txt"] = struct{}{}
			},
			wantFiles: stringSet("existing.txt"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			folder := t.TempDir()
			cache := NewCache(folder)
			tt.setup(t, folder, cache)

			err := cache.Rebuild()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tt.wantFiles, cache.cache)
		})
	}
}

func TestCacheDownload(t *testing.T) {
	tests := []struct {
		name               string
		method             string
		urlPath            string
		statusCode         int
		body               string
		preexistingFile    string
		preexistingContent string
		wantContent        string
		wantRequests       int32
		wantErr            string
	}{
		{
			name:         "download to the URL basename and create the folder",
			method:       http.MethodGet,
			urlPath:      "/nested/page.html?version=2",
			statusCode:   http.StatusOK,
			body:         "downloaded",
			wantContent:  "downloaded",
			wantRequests: 1,
		},
		{
			name:               "reuse an existing cached file without a request",
			method:             http.MethodGet,
			urlPath:            "/page.html",
			statusCode:         http.StatusInternalServerError,
			preexistingFile:    "page.html",
			preexistingContent: "cached",
			wantContent:        "cached",
			wantRequests:       0,
		},
		{
			name:         "return an HTTP error and do not cache a failed response",
			method:       http.MethodGet,
			urlPath:      "/failed.txt",
			statusCode:   http.StatusNotFound,
			body:         "not found",
			wantRequests: 1,
			wantErr:      "http error: 404 Not Found",
		},
		{
			name:         "preserve the request method",
			method:       http.MethodPost,
			urlPath:      "/head.txt",
			statusCode:   http.StatusOK,
			body:         "head response",
			wantContent:  "head response",
			wantRequests: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				assert.Equal(t, tt.method, r.Method)
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			folder := filepath.Join(t.TempDir(), "cache")
			if err := os.Mkdir(folder, 0700); err != nil {
				t.Fatalf("create cache folder: %v", err)
			}
			if tt.preexistingFile != "" {
				if err := os.WriteFile(filepath.Join(folder, tt.preexistingFile), []byte(tt.preexistingContent), 0600); err != nil {
					t.Fatalf("create existing cache file: %v", err)
				}
			}

			cache := NewCache(folder)
			got, err := cache.Download(tt.method, server.URL+tt.urlPath)
			assert.EqualValues(t, tt.wantRequests, requests.Load())
			requestURL, parseErr := url.Parse(tt.urlPath)
			if !assert.NoError(t, parseErr) {
				return
			}
			filename := path.Base(requestURL.Path)
			if tt.wantErr != "" {
				if assert.EqualError(t, err, tt.wantErr) {
					assert.Empty(t, got)
				}
				_, statErr := os.Stat(filepath.Join(folder, filename))
				assert.ErrorIs(t, statErr, os.ErrNotExist)
				return
			}

			wantPath := filepath.Join(folder, filename)
			if assert.NoError(t, err) {
				assert.Equal(t, wantPath, got)
				content, readErr := os.ReadFile(got)
				if assert.NoError(t, readErr) {
					assert.Equal(t, tt.wantContent, string(content))
				}
			}
		})
	}
}

func TestCacheDownloadPage(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		body         string
		missingFile  bool
		want         []byte
		wantErr      string
		wantNotExist bool
	}{
		{
			name:       "download and read page contents",
			statusCode: http.StatusOK,
			body:       "page content",
			want:       []byte("page content"),
		},
		{
			name:       "return download errors",
			statusCode: http.StatusBadGateway,
			body:       "gateway error",
			wantErr:    "http error: 502 Bad Gateway",
		},
		{
			name:         "return a file read error for a missing cached file",
			statusCode:   http.StatusOK,
			missingFile:  true,
			wantErr:      "file does not exist",
			wantNotExist: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			folder := t.TempDir()
			cache := NewCache(folder)
			if tt.missingFile {
				cache.cache["page.txt"] = struct{}{}
			}

			got, err := cache.DownloadPage(http.MethodGet, server.URL+"/page.txt")
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Nil(t, got)
				if tt.wantNotExist {
					assert.ErrorIs(t, err, os.ErrNotExist)
				} else {
					assert.EqualError(t, err, tt.wantErr)
				}
				return
			}

			if assert.NoError(t, err) {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestCacheDownloadAndUnGZip(t *testing.T) {
	compressed := gzipData(t, []byte("decompressed cache content"))
	tests := []struct {
		name       string
		statusCode int
		body       []byte
		want       []byte
		wantErr    string
	}{
		{
			name:       "download and decompress gzip content",
			statusCode: http.StatusOK,
			body:       compressed,
			want:       []byte("decompressed cache content"),
		},
		{
			name:       "decompress an empty gzip stream",
			statusCode: http.StatusOK,
			body:       gzipData(t, nil),
			want:       []byte{},
		},
		{
			name:       "return a decompression error for invalid content",
			statusCode: http.StatusOK,
			body:       []byte("not gzip"),
			wantErr:    "open gzip data",
		},
		{
			name:       "return an HTTP error without decompressing",
			statusCode: http.StatusServiceUnavailable,
			body:       []byte("unavailable"),
			wantErr:    "http error: 503 Service Unavailable",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write(tt.body)
			}))
			defer server.Close()

			cache := NewCache(t.TempDir())
			got, err := cache.DownloadAndUnGZip(http.MethodGet, server.URL+"/page.gz")
			if tt.wantErr != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				assert.Nil(t, got)
				return
			}

			if assert.NoError(t, err) {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func stringSet(values ...string) map[string]struct{} {
	set := make(map[string]struct{}, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}
