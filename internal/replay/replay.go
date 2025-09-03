package replay

import (
	"bytes"
	"context"
	"httpeek/internal/storage"
	"io"
	"net/http"
	"time"
)

type Result struct {
	Status     int         `json:"status"`
	DurationMs int64       `json:"durationMs"`
	Body       []byte      `json:"body,omitempty"`
	Headers    http.Header `json:"headers"`
}

func Replay(store *storage.Store, id string, ctx context.Context) (*Result, error) {
	e, err := store.Get(id)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, e.Method, e.URL, bytes.NewReader(e.ReqBody))
	if err != nil {
		return nil, err
	}
	h := make(http.Header)
	for _, kv := range e.ReqHeaders {
		h.Add(kv.Key, kv.Value)
	}
	req.Header = h

	client := &http.Client{}
	start := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		return &Result{Status: 0, DurationMs: time.Since(start).Milliseconds()}, nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return &Result{
		Status:     resp.StatusCode,
		DurationMs: time.Since(start).Milliseconds(),
		Body:       body,
		Headers:    resp.Header.Clone(),
	}, nil
}
