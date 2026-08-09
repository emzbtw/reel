package cli

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/emzbtw/reel/internal/models"
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
	for _, want := range []string{"Dune", "Dune: Part Two", "Movie", "Page 1 of 100"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

func TestBrowseMoviesCmd_PageFlag(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("page"); got != "3" {
			t.Errorf("page param = %q, want %q", got, "3")
		}
		w.Write([]byte(`{
			"page": 3, "totalPages": 100, "totalResults": 2000,
			"results": [{"id": 1, "mediaType": "movie", "title": "Dune", "releaseDate": "2021-10-22"}]
		}`))
	})

	out, err := execute(t, "", "browse", "movies", "--page", "3")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if !strings.Contains(out, "Page 3 of 100") {
		t.Errorf("output = %q, want it to mention Page 3 of 100", out)
	}
}

func TestBrowseMoviesCmd_InvalidPage(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request should have been made for an invalid --page")
	})

	_, err := execute(t, "", "browse", "movies", "--page", "0")
	if err == nil {
		t.Fatal("execute() returned nil error, want an error for --page 0")
	}
	if !strings.Contains(err.Error(), "--page") {
		t.Errorf("err = %q, want it to mention --page", err)
	}
}

func TestBrowseMoviesCmd_JSON(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"page": 1, "totalPages": 100, "totalResults": 2000,
			"results": [
				{"id": 1, "mediaType": "movie", "title": "Dune", "releaseDate": "2021-10-22"}
			]
		}`))
	})

	out, err := execute(t, "", "browse", "movies", "--json")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}

	var results []models.MovieResult
	if unmarshalErr := json.Unmarshal([]byte(out), &results); unmarshalErr != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", unmarshalErr, out)
	}
	if len(results) != 1 || results[0].Title != "Dune" {
		t.Errorf("results = %+v, want a single Dune result", results)
	}
	if strings.Contains(out, "Page") {
		t.Errorf("output = %q, should not include the Page X of Y line in JSON mode", out)
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
	for _, want := range []string{"Dune: Prophecy", "TV", "Page 1 of 100"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q: %s", want, out)
		}
	}
}

func TestBrowseTVCmd_PageFlag(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("page"); got != "3" {
			t.Errorf("page param = %q, want %q", got, "3")
		}
		w.Write([]byte(`{
			"page": 3, "totalPages": 100, "totalResults": 2000,
			"results": [{"id": 1, "mediaType": "tv", "name": "Dune: Prophecy", "firstAirDate": "2024-11-17"}]
		}`))
	})

	out, err := execute(t, "", "browse", "tv", "--page", "3")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if !strings.Contains(out, "Page 3 of 100") {
		t.Errorf("output = %q, want it to mention Page 3 of 100", out)
	}
}

func TestBrowseTVCmd_InvalidPage(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request should have been made for an invalid --page")
	})

	_, err := execute(t, "", "browse", "tv", "--page", "-1")
	if err == nil {
		t.Fatal("execute() returned nil error, want an error for --page -1")
	}
	if !strings.Contains(err.Error(), "--page") {
		t.Errorf("err = %q, want it to mention --page", err)
	}
}

func TestBrowseTVCmd_JSON(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"page": 1, "totalPages": 100, "totalResults": 2000,
			"results": [
				{"id": 1, "mediaType": "tv", "name": "Dune: Prophecy", "firstAirDate": "2024-11-17"}
			]
		}`))
	})

	out, err := execute(t, "", "browse", "tv", "--json")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}

	var results []models.TvResult
	if unmarshalErr := json.Unmarshal([]byte(out), &results); unmarshalErr != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", unmarshalErr, out)
	}
	if len(results) != 1 || results[0].Name != "Dune: Prophecy" {
		t.Errorf("results = %+v, want a single Dune: Prophecy result", results)
	}
	if strings.Contains(out, "Page") {
		t.Errorf("output = %q, should not include the Page X of Y line in JSON mode", out)
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
