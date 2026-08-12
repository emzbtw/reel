package cli

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/emzbtw/reel/internal/obsidian"
)

func TestHasChanges(t *testing.T) {
	tests := []struct {
		name    string
		actions []obsidian.Action
		want    bool
	}{
		{"empty plan", nil, false},
		{"all up to date", []obsidian.Action{obsidian.ActionNone, obsidian.ActionNone}, false},
		{"skips only", []obsidian.Action{obsidian.ActionSkip, obsidian.ActionNone}, false},
		{"a request", []obsidian.Action{obsidian.ActionNone, obsidian.ActionRequest}, true},
		{"a marker write", []obsidian.Action{obsidian.ActionNone, obsidian.ActionMarker}, true},
		{"an ambiguous line", []obsidian.Action{obsidian.ActionNone, obsidian.ActionAmbiguous}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var items []obsidian.Item
			for _, a := range tt.actions {
				items = append(items, obsidian.Item{Action: a})
			}
			plan := &obsidian.Plan{Items: items}
			if got := hasChanges(plan); got != tt.want {
				t.Errorf("hasChanges() = %v, want %v", got, tt.want)
			}
		})
	}
}

func writeNote(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "movies.md")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSyncCmd_QuietSuppressesNothingToDo(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/request":
			w.Write([]byte(`{"results": []}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	note := writeNote(t, "- [x] Heat\n")

	out, err := execute(t, "", "sync", note, "--quiet")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out != "" {
		t.Errorf("expected no output, got: %q", out)
	}
}

func TestSyncCmd_QuietStillPrintsAmbiguous(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/request":
			w.Write([]byte(`{"results": []}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/search":
			w.Write([]byte(`{"results": [
				{"mediaType": "movie", "id": 841, "title": "Dune", "releaseDate": "1984-12-14", "voteCount": 2700, "popularity": 31.0},
				{"mediaType": "movie", "id": 438631, "title": "Dune", "releaseDate": "2021-09-15", "voteCount": 2600, "popularity": 30.0}
			]}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	note := writeNote(t, "- [ ] Dune\n")

	out, err := execute(t, "", "sync", note, "--quiet", "--dry-run")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "matched several results") {
		t.Errorf("expected ambiguous match to still print under --quiet, got: %q", out)
	}
}

func TestSyncCmd_QuietMultiNoteSuppressesSilentHeaders(t *testing.T) {
	newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/request":
			w.Write([]byte(`{"results": []}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/search":
			w.Write([]byte(`{"results": [
				{"mediaType": "movie", "id": 841, "title": "Dune", "releaseDate": "1984-12-14", "voteCount": 2700, "popularity": 31.0},
				{"mediaType": "movie", "id": 438631, "title": "Dune", "releaseDate": "2021-09-15", "voteCount": 2600, "popularity": 30.0}
			]}`))
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	})
	silent := writeNote(t, "- [x] Heat\n")
	noisy := writeNote(t, "- [ ] Dune\n")

	// Silent note first: its header must not print, and the noisy note's
	// header must not carry a leading blank line since nothing printed
	// before it.
	out, err := execute(t, "", "sync", silent, noisy, "--quiet", "--dry-run")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if strings.Contains(out, silent) {
		t.Errorf("expected no header for the silent note, got: %q", out)
	}
	want := "== " + noisy + "\n"
	if !strings.HasPrefix(out, want) {
		t.Errorf("expected output to start with %q, got: %q", want, out)
	}

	// All-silent run: no output at all, across every note.
	out, err = execute(t, "", "sync", silent, silent, "--quiet", "--dry-run")
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if out != "" {
		t.Errorf("expected no output when every note is silent, got: %q", out)
	}
}
