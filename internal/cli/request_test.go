package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

const requestSearchResults = `{
	"page": 1, "totalPages": 1, "totalResults": 2,
	"results": [
		{"id": 1, "mediaType": "movie", "title": "Dune", "releaseDate": "2021-10-22"},
		{"id": 2, "mediaType": "tv", "name": "Dune: Prophecy", "firstAirDate": "2024-11-17"}
	]
}`

func TestRequestCmd_PickMovie(t *testing.T) {
	var posted bool
	var gotBody map[string]any
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/search":
			w.Write([]byte(requestSearchResults))
		case r.URL.Path == "/api/v1/request" && r.Method == http.MethodPost:
			posted = true
			b, _ := io.ReadAll(r.Body)
			json.Unmarshal(b, &gotBody)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": 99, "status": 1}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	out, err := execute(t, "1\n", "request", "dune")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if !posted {
		t.Fatal("expected a POST /request, got none")
	}
	if gotBody["mediaType"] != "movie" || gotBody["mediaId"] != float64(1) {
		t.Errorf("request body = %+v", gotBody)
	}
	if _, ok := gotBody["seasons"]; ok {
		t.Errorf("movie request body should not include seasons: %+v", gotBody)
	}
	if !strings.Contains(out, `Requested "Dune" (request #99)`) {
		t.Errorf("output = %q, want confirmation message", out)
	}
}

func TestRequestCmd_PickTVSendsAllSeasons(t *testing.T) {
	var gotBody map[string]any
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/search":
			w.Write([]byte(requestSearchResults))
		case r.URL.Path == "/api/v1/request" && r.Method == http.MethodPost:
			b, _ := io.ReadAll(r.Body)
			json.Unmarshal(b, &gotBody)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": 100, "status": 1}`))
		}
	})

	if _, err := execute(t, "2\n", "request", "dune"); err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if gotBody["mediaType"] != "tv" || gotBody["seasons"] != "all" {
		t.Errorf("request body = %+v, want mediaType=tv seasons=all", gotBody)
	}
}

func TestRequestCmd_Cancel(t *testing.T) {
	var posted bool
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/search" {
			w.Write([]byte(requestSearchResults))
			return
		}
		posted = true
	})

	for _, input := range []string{"q\n", "\n"} {
		out, err := execute(t, input, "request", "dune")
		if err != nil {
			t.Fatalf("execute(%q) returned error: %v", input, err)
		}
		if !strings.Contains(out, "Cancelled.") {
			t.Errorf("execute(%q) output = %q, want Cancelled.", input, out)
		}
		if posted {
			t.Errorf("execute(%q) should not have posted a request", input)
		}
	}
}

func TestRequestCmd_InvalidSelection(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(requestSearchResults))
	})

	_, err := execute(t, "99\n", "request", "dune")
	if err == nil {
		t.Fatal("execute() returned nil error, want invalid-selection error")
	}
	if !strings.Contains(err.Error(), "invalid selection") {
		t.Errorf("err = %q, want it to mention invalid selection", err)
	}
}

func TestRequestCmd_NoResults(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"page": 1, "totalPages": 0, "totalResults": 0, "results": []}`))
	})

	out, err := execute(t, "", "request", "nonexistent")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if !strings.Contains(out, `No movies or TV shows found for "nonexistent"`) {
		t.Errorf("output = %q, want no-results message", out)
	}
}
