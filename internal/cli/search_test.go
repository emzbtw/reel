package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/emzbtw/reel/internal/models"
)

func TestSearchCmd_FiltersPersonAndPrintsTable(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"page": 1, "totalPages": 1, "totalResults": 3,
			"results": [
				{"id": 1, "mediaType": "movie", "title": "Dune", "releaseDate": "2021-10-22"},
				{"id": 2, "mediaType": "tv", "name": "Dune: Prophecy", "firstAirDate": "2024-11-17"},
				{"id": 3, "mediaType": "person", "profilePath": "/p.jpg"}
			]
		}`))
	})

	out, err := execute(t, "", "search", "dune")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if !strings.Contains(out, "Dune") || !strings.Contains(out, "Dune: Prophecy") {
		t.Errorf("output missing expected titles: %s", out)
	}
	if strings.Contains(out, "person") {
		t.Errorf("output should not mention person results: %s", out)
	}
}

func TestSearchCmd_NoResults(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"page": 1, "totalPages": 0, "totalResults": 0, "results": []}`))
	})

	out, err := execute(t, "", "search", "nonexistent")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if !strings.Contains(out, `No movies or TV shows found for "nonexistent"`) {
		t.Errorf("output = %q, want no-results message", out)
	}
}

func TestSearchCmd_JSON(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"page": 1, "totalPages": 1, "totalResults": 3,
			"results": [
				{"id": 1, "mediaType": "movie", "title": "Dune", "releaseDate": "2021-10-22"},
				{"id": 2, "mediaType": "tv", "name": "Dune: Prophecy", "firstAirDate": "2024-11-17"},
				{"id": 3, "mediaType": "person", "profilePath": "/p.jpg"}
			]
		}`))
	})

	out, err := execute(t, "", "search", "dune", "--json")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}

	// models.SearchResult has a custom UnmarshalJSON expecting the API's
	// flat shape, but no MarshalJSON, so json.Marshal instead emits its Go
	// field names (MediaType/Movie/TV/Person) nesting the sub-results.
	// Decode with a plain mirror struct rather than models.SearchResult
	// itself to check what actually comes out on the wire.
	type rawSearchResult struct {
		MediaType models.MediaType
		Movie     *models.MovieResult
		TV        *models.TvResult
		Person    *models.PersonResult
	}

	var results []rawSearchResult
	if unmarshalErr := json.Unmarshal([]byte(out), &results); unmarshalErr != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", unmarshalErr, out)
	}
	if len(results) != 3 {
		t.Fatalf("len(results) = %d, want 3 (unfiltered, including person)", len(results))
	}
	if results[0].Movie == nil || results[0].Movie.Title != "Dune" {
		t.Errorf("results[0] = %+v, want movie %q", results[0], "Dune")
	}
	if results[1].TV == nil || results[1].TV.Name != "Dune: Prophecy" {
		t.Errorf("results[1] = %+v, want tv %q", results[1], "Dune: Prophecy")
	}
	if results[2].Person == nil {
		t.Errorf("results[2] = %+v, want a person result", results[2])
	}
}

func TestSearchCmd_Unauthorized(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"unauthorized"}`))
	})

	_, err := execute(t, "", "search", "dune")
	if err == nil {
		t.Fatal("execute() returned nil error, want unauthorized error")
	}
	if !strings.Contains(FormatError(err), "API key") {
		t.Errorf("FormatError(err) = %q, want it to mention the API key", FormatError(err))
	}
}
