package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestTrendingCmd_PrintsTable(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/discover/trending" {
			t.Errorf("path = %q, want /api/v1/discover/trending", r.URL.Path)
		}
		w.Write([]byte(`{
			"page": 1, "totalPages": 1, "totalResults": 2,
			"results": [
				{"id": 1, "mediaType": "movie", "title": "Dune", "releaseDate": "2021-10-22"},
				{"id": 2, "mediaType": "tv", "name": "Dune: Prophecy", "firstAirDate": "2024-11-17"}
			]
		}`))
	})

	out, err := execute(t, "", "trending")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if !strings.Contains(out, "Dune") || !strings.Contains(out, "Dune: Prophecy") {
		t.Errorf("output missing expected titles: %s", out)
	}
}

func TestTrendingCmd_NoResults(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"page": 1, "totalPages": 0, "totalResults": 0, "results": []}`))
	})

	out, err := execute(t, "", "trending")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if !strings.Contains(out, "No trending results.") {
		t.Errorf("output = %q, want no-results message", out)
	}
}
