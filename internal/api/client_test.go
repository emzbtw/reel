package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/emzbtw/reel/internal/config"
)

func testClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(&config.Config{SeerrURL: srv.URL, SeerrAPIKey: "test-key"})
}

func TestDo_SendsAPIKeyHeader(t *testing.T) {
	var gotKey string
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Api-Key")
		w.WriteHeader(http.StatusOK)
	})

	if _, err := c.Search(context.Background(), "dune", 0); err != nil {
		t.Fatalf("Search() returned error: %v", err)
	}
	if gotKey != "test-key" {
		t.Errorf("X-Api-Key header = %q, want %q", gotKey, "test-key")
	}
}

func TestDo_ErrorMapping(t *testing.T) {
	tests := []struct {
		status  int
		wantErr error
	}{
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusNotFound, ErrNotFound},
		{http.StatusTooManyRequests, ErrRateLimited},
	}

	for _, tt := range tests {
		c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tt.status)
			w.Write([]byte(`{"message":"nope"}`))
		})

		_, err := c.Search(context.Background(), "dune", 0)
		if err == nil {
			t.Fatalf("status %d: Search() returned nil error", tt.status)
		}
		if !errors.Is(err, tt.wantErr) {
			t.Errorf("status %d: errors.Is(err, %v) = false, err = %v", tt.status, tt.wantErr, err)
		}

		var apiErr *APIError
		if !errors.As(err, &apiErr) {
			t.Fatalf("status %d: errors.As(err, *APIError) = false", tt.status)
		}
		if apiErr.StatusCode != tt.status {
			t.Errorf("APIError.StatusCode = %d, want %d", apiErr.StatusCode, tt.status)
		}
	}
}
