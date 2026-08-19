package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/emzbtw/reel/internal/models"
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

func TestRequestCmd_PickMovieJSON(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/search":
			w.Write([]byte(requestSearchResults))
		case r.URL.Path == "/api/v1/request" && r.Method == http.MethodPost:
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": 99, "status": 1, "media": {"id": 1, "tmdbId": 1234, "status": 2}}`))
		}
	})

	out, err := execute(t, "1\n", "request", "dune", "--json")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}

	// The picker table and prompt still print as plain text even with
	// --json; only the final created-request output changes shape.
	if !strings.Contains(out, "Dune") || !strings.Contains(out, "Pick a result to request") {
		t.Errorf("output = %q, want the picker table and prompt to still print", out)
	}
	if strings.Contains(out, `Requested "Dune"`) {
		t.Errorf("output = %q, should not print the plain-text confirmation under --json", out)
	}

	jsonStart := strings.Index(out, "{")
	if jsonStart == -1 {
		t.Fatalf("output has no JSON object: %s", out)
	}

	var req models.MediaRequest
	if unmarshalErr := json.Unmarshal([]byte(out[jsonStart:]), &req); unmarshalErr != nil {
		t.Fatalf("final output is not valid JSON: %v\noutput: %s", unmarshalErr, out)
	}
	if req.ID != 99 || req.Media.TmdbID != 1234 {
		t.Errorf("req = %+v, want id=99 tmdbId=1234", req)
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

func TestRequestCmd_SelectMovie(t *testing.T) {
	var searched, posted bool
	var gotBody map[string]any
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/search":
			searched = true
		case r.URL.Path == "/api/v1/request" && r.Method == http.MethodPost:
			posted = true
			b, _ := io.ReadAll(r.Body)
			json.Unmarshal(b, &gotBody)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": 99, "status": 1}`))
		}
	})

	out, err := execute(t, "", "request", "--select", "438631", "--type", "movie")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if searched {
		t.Error("--select should skip search entirely")
	}
	if !posted {
		t.Fatal("expected a POST /request, got none")
	}
	if gotBody["mediaType"] != "movie" || gotBody["mediaId"] != float64(438631) {
		t.Errorf("request body = %+v", gotBody)
	}
	if !strings.Contains(out, "Requested tmdbId 438631 (request #99)") {
		t.Errorf("output = %q, want a tmdbId-based confirmation", out)
	}
}

func TestRequestCmd_SelectTVSendsAllSeasons(t *testing.T) {
	var gotBody map[string]any
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/request" && r.Method == http.MethodPost {
			b, _ := io.ReadAll(r.Body)
			json.Unmarshal(b, &gotBody)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": 100, "status": 1}`))
		}
	})

	// --type is accepted case-insensitively.
	if _, err := execute(t, "", "request", "--select", "1", "--type", "TV"); err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if gotBody["mediaType"] != "tv" || gotBody["seasons"] != "all" {
		t.Errorf("request body = %+v, want mediaType=tv seasons=all", gotBody)
	}
}

func TestRequestCmd_SelectJSON(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id": 99, "status": 1, "media": {"id": 1, "tmdbId": 438631, "status": 2}}`))
	})

	out, err := execute(t, "", "request", "--select", "438631", "--type", "movie", "--json")
	if err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}

	// Fully non-interactive: no table, no prompt, just the JSON object.
	if strings.Contains(out, "Pick a result") || strings.Contains(out, "#") {
		t.Errorf("output = %q, want no picker output at all", out)
	}

	var req models.MediaRequest
	if unmarshalErr := json.Unmarshal([]byte(out), &req); unmarshalErr != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", unmarshalErr, out)
	}
	if req.ID != 99 || req.Media.TmdbID != 438631 {
		t.Errorf("req = %+v, want id=99 tmdbId=438631", req)
	}
}

func TestRequestCmd_SelectValidation(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request should have been made for invalid --select/--type usage")
	})

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"missing --type", []string{"request", "--select", "1"}, "--type is required"},
		{"missing --select (only --type given)", []string{"request", "--type", "movie"}, "--select must be a positive TMDB ID"},
		{"no query and no flags", []string{"request"}, "request requires a <query> argument"},
		{"invalid --type", []string{"request", "--select", "1", "--type", "album"}, "--type must be movie or tv"},
		{"select explicitly zero", []string{"request", "--select", "0", "--type", "movie"}, "--select must be a positive TMDB ID"},
		{"negative --select", []string{"request", "--select", "-1", "--type", "movie"}, "--select must be a positive TMDB ID"},
		{"query combined with --select", []string{"request", "dune", "--select", "1", "--type", "movie"}, "cannot be combined with --select"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := execute(t, "", tt.args...)
			if err == nil {
				t.Fatalf("execute(%v) returned nil error, want an error containing %q", tt.args, tt.want)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("execute(%v) err = %q, want it to contain %q", tt.args, err, tt.want)
			}
		})
	}
}

const sonarrServersResponse = `[
	{"id": 0, "name": "Sonarr", "isDefault": true},
	{"id": 1, "name": "Sonarr Anime", "isDefault": false}
]`

func TestRequestCmd_SelectTVWithServer(t *testing.T) {
	var gotBody map[string]any
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/service/sonarr":
			w.Write([]byte(sonarrServersResponse))
		case r.URL.Path == "/api/v1/request" && r.Method == http.MethodPost:
			b, _ := io.ReadAll(r.Body)
			json.Unmarshal(b, &gotBody)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": 101, "status": 1}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	// Matching is case-insensitive.
	if _, err := execute(t, "", "request", "--select", "1", "--type", "tv", "--server", "sonarr anime"); err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	if gotBody["serverId"] != float64(1) {
		t.Errorf("request body serverId = %+v, want 1", gotBody["serverId"])
	}
}

// TestRequestCmd_SelectTVWithServerIDZero guards the same edge case as
// TestCreateRequest_ServerIDZero one layer down: sonarrServersResponse's
// "Sonarr" entry has id 0, and that must reach the POST body as
// "serverId":0, not be silently dropped as if unset.
func TestRequestCmd_SelectTVWithServerIDZero(t *testing.T) {
	var gotBody map[string]any
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/service/sonarr":
			w.Write([]byte(sonarrServersResponse))
		case r.URL.Path == "/api/v1/request" && r.Method == http.MethodPost:
			b, _ := io.ReadAll(r.Body)
			json.Unmarshal(b, &gotBody)
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": 102, "status": 1}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	if _, err := execute(t, "", "request", "--select", "1", "--type", "tv", "--server", "Sonarr"); err != nil {
		t.Fatalf("execute() returned error: %v", err)
	}
	got, ok := gotBody["serverId"]
	if !ok {
		t.Fatal("request body omits serverId entirely, want serverId:0")
	}
	if got != float64(0) {
		t.Errorf("request body serverId = %+v, want 0", got)
	}
}

func TestRequestCmd_SelectServerNotFound(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/v1/service/sonarr":
			w.Write([]byte(sonarrServersResponse))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})

	_, err := execute(t, "", "request", "--select", "1", "--type", "tv", "--server", "nope")
	if err == nil {
		t.Fatal("execute() returned nil error, want an unknown-server error")
	}
	for _, want := range []string{`server "nope" not found`, "Sonarr", "Sonarr Anime"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %q, want it to contain %q", err, want)
		}
	}
}

func TestRequestCmd_SelectServerMovieErrors(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no request should have been made: %s %s", r.Method, r.URL.Path)
	})

	_, err := execute(t, "", "request", "--select", "1", "--type", "movie", "--server", "Sonarr Anime")
	if err == nil {
		t.Fatal("execute() returned nil error, want --server-on-movie error")
	}
	if !strings.Contains(err.Error(), "--server is only supported for TV requests") {
		t.Errorf("err = %q, want the TV-only message", err)
	}
}
