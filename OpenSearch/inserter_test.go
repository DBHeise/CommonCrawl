package OpenSearch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBulkInsertReadsResponseBeforeCancelingRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/_bulk", r.URL.Path)
		assert.Equal(t, "application/x-ndjson", r.Header.Get("Content-Type"))

		payload, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Contains(t, string(payload), `"url":"https://example.com"`)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		time.Sleep(25 * time.Millisecond)
		_, _ = fmt.Fprint(w, `{"errors":false,"items":[{"index":{"status":201}}]}`)
	}))
	defer server.Close()

	docs := make(chan map[string]string, 1)
	docs <- map[string]string{"url": "https://example.com"}
	close(docs)

	stats := &BulkStats{}
	client := &Client{
		BaseURL: server.URL,
		Client:  server.Client(),
	}

	err := client.BulkInsert(context.Background(), "test-index", docs, 1, stats, nil)

	assert.NoError(t, err)
	assert.Equal(t, uint64(1), stats.Snapshot().Indexed)
	assert.Equal(t, uint64(1), stats.Snapshot().Batches)
}

func TestBulkInsertHandlesCanceledContext(t *testing.T) {
	docs := make(chan map[string]string)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := &Client{
		BaseURL: "http://127.0.0.1:9200",
		Client:  &http.Client{},
	}

	err := client.BulkInsert(ctx, "test-index", docs, 1, &BulkStats{}, nil)

	assert.ErrorIs(t, err, context.Canceled)
}
