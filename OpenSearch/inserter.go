package OpenSearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultWorkers   = 8
	DefaultBatchDocs = 5000
	DefaultBatchSize = 15 * 1024 * 1024 // 15MB
)

type Client struct {
	BaseURL string
	Client  *http.Client
}

type BulkStats struct {
	Indexed uint64
	Failed  uint64
	Batches uint64
}

type BulkItemError struct {
	Document map[string]string
	Status   int
	Type     string
	Reason   string
}

type ErrorHandler func(BulkItemError)

type bulkDoc struct {
	Document map[string]string
}

type bulkResponse struct {
	Errors bool `json:"errors"`

	Items []struct {
		Index struct {
			Status int `json:"status"`

			Error struct {
				Type   string `json:"type"`
				Reason string `json:"reason"`
			} `json:"error"`
		} `json:"index"`
	} `json:"items"`
}

func (s *BulkStats) Snapshot() BulkStats {
	return BulkStats{
		Indexed: atomic.LoadUint64(&s.Indexed),
		Failed:  atomic.LoadUint64(&s.Failed),
		Batches: atomic.LoadUint64(&s.Batches),
	}
}

func (c *Client) BulkInsert(ctx context.Context, index string, docs <-chan map[string]string, workers int, stats *BulkStats, handler ErrorHandler) error {

	if workers <= 0 {
		workers = DefaultWorkers
	}

	if handler == nil {
		handler = func(BulkItemError) {}
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, workers)

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			if err := c.bulkWorker(ctx, index, docs, stats, handler); err != nil {
				select {
				case errCh <- err:
					cancel()
				default:
				}
			}
		}()
	}

	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (c *Client) doBulkRequest(ctx context.Context, payload []byte) (*http.Response, error) {

	var lastErr error

	for attempt := 0; attempt < 5; attempt++ {

		reqCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)

		req, err := http.NewRequestWithContext(
			reqCtx,
			http.MethodPost,
			strings.TrimRight(c.BaseURL, "/")+"/_bulk",
			bytes.NewReader(payload),
		)
		if err != nil {
			cancel()
			return nil, err
		}

		req.Header.Set("Content-Type", "application/x-ndjson")

		resp, err := c.Client.Do(req)

		if err == nil {
			// The caller must cancel only after consuming and closing resp.Body.
			resp.Body = &cancelOnCloseBody{ReadCloser: resp.Body, cancel: cancel}
			return resp, nil
		}
		cancel()

		lastErr = err
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		timer := time.NewTimer(time.Second * time.Duration(attempt+1))
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}

	return nil, lastErr
}

type cancelOnCloseBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelOnCloseBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

func (c *Client) bulkWorker(ctx context.Context, index string, docs <-chan map[string]string, stats *BulkStats, handler ErrorHandler) error {

	var (
		buf      bytes.Buffer
		docCount int
		batch    []bulkDoc
	)

	flush := func() error {

		if docCount == 0 {
			return nil
		}

		resp, err := c.doBulkRequest(ctx, buf.Bytes())
		if err != nil {
			return err
		}

		body, err := io.ReadAll(resp.Body)
		closeErr := resp.Body.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}

		if resp.StatusCode >= 300 {
			return fmt.Errorf(
				"bulk request failed: status=%d body=%s",
				resp.StatusCode,
				string(body),
			)
		}

		var result bulkResponse

		if err := json.Unmarshal(body, &result); err != nil {
			return err
		}

		successCount := 0

		if result.Errors {

			for i, item := range result.Items {

				if item.Index.Status < 300 {
					successCount++
					continue
				}

				atomic.AddUint64(&stats.Failed, 1)

				handler(BulkItemError{
					Document: batch[i].Document,
					Status:   item.Index.Status,
					Type:     item.Index.Error.Type,
					Reason:   item.Index.Error.Reason,
				})
			}
		} else {
			successCount = len(batch)
		}

		atomic.AddUint64(&stats.Indexed, uint64(successCount))
		atomic.AddUint64(&stats.Batches, 1)

		buf.Reset()
		docCount = 0

		batch = batch[:0]

		return nil
	}

	for {
		select {

		case <-ctx.Done():
			return ctx.Err()

		case doc, ok := <-docs:

			if !ok {
				return flush()
			}

			meta := map[string]any{
				"index": map[string]string{
					"_index": index,
				},
			}

			if id, ok := doc["digest"]; ok {
				meta["index"].(map[string]string)["_id"] = id
			}

			metaJSON, err := json.Marshal(meta)
			if err != nil {
				return err
			}

			docJSON, err := json.Marshal(doc)
			if err != nil {
				return err
			}

			buf.Write(metaJSON)
			buf.WriteByte('\n')

			buf.Write(docJSON)
			buf.WriteByte('\n')

			batch = append(batch, bulkDoc{
				Document: doc,
			})

			docCount++

			if docCount >= DefaultBatchDocs ||
				buf.Len() >= DefaultBatchSize {

				if err := flush(); err != nil {

					return err
				}
			}
		}
	}
}
