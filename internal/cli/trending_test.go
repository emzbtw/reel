package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/emzbtw/reel/internal/models"
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

func TestTrendingCmd_JSON(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"page": 1, "totalPages": 1, "totalResults": 2,
			"results": [
				{"id": 1, "mediaType": "movie", "title": "Dune", "releaseDate": "2021-10-22"},
				{"id": 2, "mediaType": "tv", "name": "Dune: Prophecy", "firstAirDate": "2024-11-17"}
			]
		}`))
	})

	out, err := execute(t, "", "trending", "--json")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}

	// See TestSearchCmd_JSON for why this mirror struct exists rather than
	// decoding into models.SearchResult directly.
	type rawSearchResult struct {
		MediaType models.MediaType
		Movie     *models.MovieResult
		TV        *models.TvResult
	}

	var results []rawSearchResult
	if unmarshalErr := json.Unmarshal([]byte(out), &results); unmarshalErr != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", unmarshalErr, out)
	}
	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}
	if results[0].Movie == nil || results[0].Movie.Title != "Dune" {
		t.Errorf("results[0] = %+v, want movie %q", results[0], "Dune")
	}
	if results[1].TV == nil || results[1].TV.Name != "Dune: Prophecy" {
		t.Errorf("results[1] = %+v, want tv %q", results[1], "Dune: Prophecy")
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
