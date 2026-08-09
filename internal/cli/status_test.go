package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/emzbtw/reel/internal/models"
)

func TestStatusCmd_PrintsTable(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/request" {
			t.Errorf("path = %q, want /api/v1/request", r.URL.Path)
		}
		w.Write([]byte(`{
			"pageInfo": {"page": 1, "pages": 1, "results": 1},
			"results": [{"id": 5, "status": 2, "media": {"id": 10, "tmdbId": 1234, "status": 4}, "createdAt": "2026-08-01T10:00:00.000Z"}]
		}`))
	})

	out, err := execute(t, "", "status")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	for _, want := range []string{"1234", "Approved", "Partially available"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

func TestStatusCmd_JSON(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"pageInfo": {"page": 1, "pages": 1, "results": 1},
			"results": [{"id": 5, "status": 2, "media": {"id": 10, "tmdbId": 1234, "status": 4}, "createdAt": "2026-08-01T10:00:00.000Z"}]
		}`))
	})

	out, err := execute(t, "", "status", "--json")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}

	var results []models.MediaRequest
	if unmarshalErr := json.Unmarshal([]byte(out), &results); unmarshalErr != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", unmarshalErr, out)
	}
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if results[0].ID != 5 || results[0].Media.TmdbID != 1234 {
		t.Errorf("results[0] = %+v, want id=5 tmdbId=1234", results[0])
	}
}

func TestStatusCmd_NoRequests(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"pageInfo": {"page": 1, "pages": 0, "results": 0}, "results": []}`))
	})

	out, err := execute(t, "", "status")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if !strings.Contains(out, "No requests found.") {
		t.Errorf("output = %q, want no-requests message", out)
	}
}
