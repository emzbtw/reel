package cli

import (
	"net/http"
	"strings"
	"testing"
)

func TestBrowseMoviesCmd_PrintsTable(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/discover/movies" {
			t.Errorf("path = %q, want /api/v1/discover/movies", r.URL.Path)
		}
		if got := r.URL.Query().Get("page"); got != "1" {
			t.Errorf("page param = %q, want %q", got, "1")
		}
		w.Write([]byte(`{
			"page": 1, "totalPages": 100, "totalResults": 2000,
			"results": [
				{"id": 1, "mediaType": "movie", "title": "Dune", "releaseDate": "2021-10-22"},
				{"id": 2, "mediaType": "movie", "title": "Dune: Part Two", "releaseDate": "2024-03-01"}
			]
		}`))
	})

	out, err := execute(t, "", "browse", "movies")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	for _, want := range []string{"Dune", "Dune: Part Two", "Movie"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

func TestBrowseMoviesCmd_NoResults(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"page": 1, "totalPages": 0, "totalResults": 0, "results": []}`))
	})

	out, err := execute(t, "", "browse", "movies")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if !strings.Contains(out, "No movies found.") {
		t.Errorf("output = %q, want no-results message", out)
	}
}

func TestBrowseTVCmd_PrintsTable(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/discover/tv" {
			t.Errorf("path = %q, want /api/v1/discover/tv", r.URL.Path)
		}
		w.Write([]byte(`{
			"page": 1, "totalPages": 100, "totalResults": 2000,
			"results": [
				{"id": 1, "mediaType": "tv", "name": "Dune: Prophecy", "firstAirDate": "2024-11-17"}
			]
		}`))
	})

	out, err := execute(t, "", "browse", "tv")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	for _, want := range []string{"Dune: Prophecy", "TV"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

func TestBrowseTVCmd_NoResults(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"page": 1, "totalPages": 0, "totalResults": 0, "results": []}`))
	})

	out, err := execute(t, "", "browse", "tv")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if !strings.Contains(out, "No TV shows found.") {
		t.Errorf("output = %q, want no-results message", out)
	}
}
