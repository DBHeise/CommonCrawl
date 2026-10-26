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
	previousUserAgent := viper.GetString("UserAgent")
	viper.Set("UserAgent", "test-agent")
	t.Cleanup(func() {
		viper.Set("UserAgent", previousUserAgent)
	})

	client, req := NewHttpRequest(method, requestURL)

	assert.NotNil(t, client)
	if assert.NotNil(t, req) {
		assert.Equal(t, method, req.Method)
		assert.Equal(t, requestURL, req.URL.String())
		assert.Equal(t, "test-agent", req.Header.Get("User-Agent"))
	}
}

func TestURLToLocalPathRobust(t *testing.T) {
	tests := []struct {
		name    string
		baseDir string
		rawURL  string
		mime    string
		want    string
		wantErr bool
	}{
		{
			name:    "hash URL and partition by MIME and top-level domain",
			baseDir: "archive",
			rawURL:  "https://www.example.com/a/b.html?x=1",
			mime:    "text/html; charset=utf-8",
			want: filepath.Join(
				"archive",
				"text_html",
				"com",
				"353",
				"225",
				"0ed",
				"3532250edb2a93f9d894c6a9dbace024bae2ecd7bacb8399171c662006e9dcac",
			),
		},
		{
			name:    "use unknown domain for host without a dot",
			baseDir: "archive",
			rawURL:  "http://localhost/resource",
			mime:    "application/json",
			want: filepath.Join(
				"archive",
				"application_json",
				"unknown",
				"621",
				"56b",
				"29e",
				"62156b29e9378b20861a3d9e14307269a414ae1cd7bdddfaf5ccd476e88e0c99",
			),
		},
		{
			name:    "reject malformed URL escape",
			baseDir: "archive",
			rawURL:  "https://example.com/%zz",
			mime:    "text/html",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := URLToLocalPath_Robust(tt.baseDir, tt.rawURL, tt.mime)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, got)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestURLToLocalPath(t *testing.T) {
	tests := []struct {
		name    string
		baseDir string
		rawURL  string
		want    string
		wantErr bool
	}{
		{
			name:    "preserve directories and use URL hash as filename",
			baseDir: "archive",
			rawURL:  "https://www.example.com/a/b.html?x=1",
			want:    filepath.Join("archive", "www.example.com", "a", "3532250edb2a93f9.html"),
		},
		{
			name:    "use HTML extension when URL has none",
			baseDir: "archive",
			rawURL:  "https://example.com",
			want:    filepath.Join("archive", "example.com", "100680ad546ce6a5.html"),
		},
		{
			name:    "URL query affects generated filename",
			baseDir: "archive",
			rawURL:  "https://example.com/path?one=1",
			want:    filepath.Join("archive", "example.com", "93d282acfb8b2dcd.html"),
		},
		{
			name:    "reject malformed URL",
			baseDir: "archive",
			rawURL:  "https://[::1",
			wantErr: true,
		},
		{
			name:    "sanitize empty hostname",
			baseDir: "archive",
			rawURL:  "mailto:user@example.com",
			want:    filepath.Join("archive", "_", "57c9bca033401834.html"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := URLToLocalPath(tt.baseDir, tt.rawURL)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Empty(t, got)
				return
			}
			if assert.NoError(t, err) {
				baseAbs, absErr := filepath.Abs(tt.want)
				assert.NoError(t, absErr)
				assert.Equal(t, baseAbs, got)
			}
		})
	}
}

func TestDownloadPage(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		want       []byte
		wantErr    string
	}{
		{
			name:       "download successful response",
			statusCode: http.StatusOK,
			body:       "<html>content</html>",
			want:       []byte("<html>content</html>"),
		},
		{
			name:       "accept created response",
			statusCode: http.StatusCreated,
			body:       "created",
			want:       []byte("created"),
		},
		{
			name:       "accept empty response body",
			statusCode: http.StatusOK,
			body:       "",
			want:       []byte{},
		},
		{
			name:       "reject client error",
			statusCode: http.StatusNotFound,
			body:       "missing",
			wantErr:    "404 Not Found",
		},
		{
			name:       "reject server error",
			statusCode: http.StatusInternalServerError,
			body:       "failed",
			wantErr:    "500 Internal Server Error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = io.WriteString(w, tt.body)
			}))
			defer server.Close()

			got, err := DownloadPage(server.URL)
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
			wantErr:    "unexpected status: 201 Created",
		},
		{
			name:       "reject server error",
			statusCode: http.StatusInternalServerError,
			body:       "not written",
			wantErr:    "unexpected status: 500 Internal Server Error",
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
			err := DownloadToFile(server.URL, filePath)
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

		err := DownloadToFile("http://example.com", filepath.Join(parentFile, "page.txt"))
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

func TestDownloadAndUnGZip(t *testing.T) {
	validGZIP := gzipData(t, []byte("downloaded and decompressed"))

	tests := []struct {
		name       string
		statusCode int
		body       []byte
		want       []byte
		wantErr    string
	}{
		{
			name:       "download and decompress successful response",
			statusCode: http.StatusOK,
			body:       validGZIP,
			want:       []byte("downloaded and decompressed"),
		},
		{
			name:       "propagate download status error",
			statusCode: http.StatusNotFound,
			body:       []byte("missing"),
			wantErr:    "404 Not Found",
		},
		{
			name:       "report invalid gzip response",
			statusCode: http.StatusOK,
			body:       []byte("not gzip"),
			wantErr:    "open gzip data",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.statusCode)
				_, _ = w.Write(tt.body)
			}))
			defer server.Close()

			got, err := DownloadAndUnGZip(server.URL)
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
