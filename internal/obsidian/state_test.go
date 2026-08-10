package obsidian

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pelletier/go-toml/v2"
)

// makeVault creates dir/.obsidian as a directory, marking dir as a vault
// root.
func makeVault(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, ".obsidian"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestFindVaultRoot(t *testing.T) {
	t.Run("note directly in vault root", func(t *testing.T) {
		vault := t.TempDir()
		makeVault(t, vault)
		note := filepath.Join(vault, "Note.md")

		if got := FindVaultRoot(note); got != vault {
			t.Errorf("FindVaultRoot(%q) = %q, want %q", note, got, vault)
		}
	})

	t.Run("note nested several directories deep", func(t *testing.T) {
		vault := t.TempDir()
		makeVault(t, vault)
		notedir := filepath.Join(vault, "a", "b", "c")
		if err := os.MkdirAll(notedir, 0o755); err != nil {
			t.Fatal(err)
		}
		note := filepath.Join(notedir, "Note.md")

		if got := FindVaultRoot(note); got != vault {
			t.Errorf("FindVaultRoot(%q) = %q, want %q", note, got, vault)
		}
	})

	t.Run("no vault anywhere", func(t *testing.T) {
		dir := t.TempDir()
		note := filepath.Join(dir, "Note.md")

		if got := FindVaultRoot(note); got != "" {
			t.Errorf("FindVaultRoot(%q) = %q, want \"\"", note, got)
		}
	})

	t.Run(".obsidian as a file does not count", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, ".obsidian"), []byte("not a dir"), 0o644); err != nil {
			t.Fatal(err)
		}
		note := filepath.Join(dir, "Note.md")

		if got := FindVaultRoot(note); got != "" {
			t.Errorf("FindVaultRoot(%q) = %q, want \"\" (a file .obsidian must not count)", note, got)
		}
	})

	// "reel sync Movies.md" run from inside the vault must find the vault.
	// Walking a relative path without resolving it first stops immediately,
	// because filepath.Dir(".") is ".", and the note silently falls back to
	// XDG state instead of the in-vault store.
	t.Run("relative note path from a vault subdirectory", func(t *testing.T) {
		vault := t.TempDir()
		makeVault(t, vault)
		notedir := filepath.Join(vault, "Media")
		if err := os.MkdirAll(notedir, 0o755); err != nil {
			t.Fatal(err)
		}

		wd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chdir(wd) })
		if err := os.Chdir(notedir); err != nil {
			t.Fatal(err)
		}

		got := FindVaultRoot("Movies.md")
		// The vault path itself may be a symlink (macOS /var -> /private/var),
		// so compare resolved forms.
		wantResolved, _ := filepath.EvalSymlinks(vault)
		gotResolved, _ := filepath.EvalSymlinks(got)
		if got == "" || gotResolved != wantResolved {
			t.Errorf("FindVaultRoot(\"Movies.md\") from %s = %q, want %q", notedir, got, vault)
		}
	})

	t.Run("does not walk past the vault root into real directories", func(t *testing.T) {
		// The vault root here is t.TempDir() itself, which sits under the
		// system temp directory. If the walk kept going past the root it
		// found, it would climb into real ancestor directories on the test
		// machine; assert it stops as soon as it finds .obsidian.
		vault := t.TempDir()
		makeVault(t, vault)
		note := filepath.Join(vault, "Note.md")

		if got := FindVaultRoot(note); got != vault {
			t.Errorf("FindVaultRoot(%q) = %q, want %q", note, got, vault)
		}
	})
}

func TestNormalizeTitle(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"mixed case", "Arrival", "arrival"},
		{"padding", "  Arrival  ", "arrival"},
		{"internal whitespace run", "The   Matrix", "the matrix"},
		{"tabs and newlines", "The\tMatrix\nReloaded", "the matrix reloaded"},
		{"already normalized", "the matrix", "the matrix"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizeTitle(tc.in); got != tc.want {
				t.Errorf("NormalizeTitle(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestLoadStore_MissingFileYieldsEmptyStore(t *testing.T) {
	vault := t.TempDir()
	makeVault(t, vault)
	note := filepath.Join(vault, "Note.md")

	s, err := LoadStore(note)
	if err != nil {
		t.Fatalf("LoadStore() returned error: %v", err)
	}
	if _, ok := s.Get(note, "arrival"); ok {
		t.Errorf("Get() on empty store found a binding")
	}
}

func TestLoadStore_NotePathInVault_StoresRelativeNote(t *testing.T) {
	vault := t.TempDir()
	makeVault(t, vault)
	notedir := filepath.Join(vault, "Media")
	if err := os.MkdirAll(notedir, 0o755); err != nil {
		t.Fatal(err)
	}
	note := filepath.Join(notedir, "Movies.md")

	s, err := LoadStore(note)
	if err != nil {
		t.Fatalf("LoadStore() returned error: %v", err)
	}
	s.Put(Binding{Note: note, Title: "Arrival", TmdbID: 329865, MediaType: "movie"})
	if err := s.Save(); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	got := readBindingNote(t, s.Path())
	if got != "Media/Movies.md" {
		t.Errorf("saved note = %q, want %q (vault-relative)", got, "Media/Movies.md")
	}
}

func TestLoadStore_NotePathOutsideVault_StoresAbsoluteNoteAndXDGPath(t *testing.T) {
	dir := t.TempDir() // not a vault
	note := filepath.Join(dir, "Note.md")

	stateHome := t.TempDir()
	t.Setenv("XDG_STATE_HOME", stateHome)

	s, err := LoadStore(note)
	if err != nil {
		t.Fatalf("LoadStore() returned error: %v", err)
	}
	wantPath := filepath.Join(stateHome, "reel", "obsidian.toml")
	if s.Path() != wantPath {
		t.Errorf("Path() = %q, want %q", s.Path(), wantPath)
	}

	s.Put(Binding{Note: note, Title: "Arrival", TmdbID: 329865, MediaType: "movie"})
	if err := s.Save(); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	got := readBindingNote(t, s.Path())
	if got != note {
		t.Errorf("saved note = %q, want %q (absolute)", got, note)
	}
}

// readBindingNote reads back the single binding's note field from a saved
// state file, independent of whatever quoting style the TOML encoder chose.
func readBindingNote(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading saved state file: %v", err)
	}
	var file stateFile
	if err := toml.Unmarshal(data, &file); err != nil {
		t.Fatalf("parsing saved state file: %v", err)
	}
	if len(file.Binding) != 1 {
		t.Fatalf("saved file has %d bindings, want 1", len(file.Binding))
	}
	return file.Binding[0].Note
}

func TestStore_RoundTrip(t *testing.T) {
	vault := t.TempDir()
	makeVault(t, vault)
	noteA := filepath.Join(vault, "Movies.md")
	noteB := filepath.Join(vault, "Shows.md")

	bindings := []Binding{
		{Note: noteA, Title: "Arrival", TmdbID: 329865, MediaType: "movie", RequestID: 42, LastStatus: "available", LastMarker: "✓"},
		{Note: noteA, Title: "Alien", TmdbID: 348, MediaType: "movie", RequestID: 7, LastStatus: "pending", LastMarker: "~"},
		{Note: noteB, Title: "Severance", TmdbID: 95396, MediaType: "tv", RequestID: 0, LastStatus: "", LastMarker: " "},
	}

	s, err := LoadStore(noteA)
	if err != nil {
		t.Fatalf("LoadStore() returned error: %v", err)
	}
	for _, b := range bindings {
		s.Put(b)
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	reloaded, err := LoadStore(noteA)
	if err != nil {
		t.Fatalf("second LoadStore() returned error: %v", err)
	}
	for _, want := range bindings {
		got, ok := reloaded.Get(want.Note, want.Title)
		if !ok {
			t.Errorf("Get(%q, %q) not found after reload", want.Note, want.Title)
			continue
		}
		wantNormalized := want
		wantNormalized.Note = reloaded.noteKey(want.Note)
		wantNormalized.Title = NormalizeTitle(want.Title)
		if got != wantNormalized {
			t.Errorf("Get(%q, %q) = %+v, want %+v", want.Note, want.Title, got, wantNormalized)
		}
	}
}

func TestStore_Delete(t *testing.T) {
	vault := t.TempDir()
	makeVault(t, vault)
	note := filepath.Join(vault, "Movies.md")

	s, err := LoadStore(note)
	if err != nil {
		t.Fatalf("LoadStore() returned error: %v", err)
	}
	s.Put(Binding{Note: note, Title: "Arrival", TmdbID: 329865, MediaType: "movie"})
	s.Delete(note, "Arrival")

	if _, ok := s.Get(note, "Arrival"); ok {
		t.Errorf("Get() found a binding after Delete()")
	}
}

// TestStore_DeterministicOrder is the test that protects the property the
// design relies on for Syncthing: bindings must serialize in the same
// order regardless of the order they were inserted in, so that a save with
// no real content change produces byte-identical output. If it didn't, a
// Go map's randomized iteration order would rewrite the whole file on
// every sync, and Syncthing would treat that as a genuine change on every
// machine — multiplying the chance of two machines saving concurrently and
// one being sidelined as a sync-conflict file.
func TestStore_DeterministicOrder(t *testing.T) {
	orders := [][]int{
		{0, 1, 2, 3},
		{3, 2, 1, 0},
		{2, 0, 3, 1},
		{1, 3, 0, 2},
	}

	var results [][]byte
	for i, order := range orders {
		// Each order gets its own vault (its own save location), per the
		// requirement to save to separate locations and compare bytes.
		vault := t.TempDir()
		makeVault(t, vault)
		noteA := filepath.Join(vault, "Movies.md")
		noteB := filepath.Join(vault, "Shows.md")
		bindings := []Binding{
			{Note: noteA, Title: "Arrival", TmdbID: 329865, MediaType: "movie"},
			{Note: noteA, Title: "Alien", TmdbID: 348, MediaType: "movie"},
			{Note: noteB, Title: "Severance", TmdbID: 95396, MediaType: "tv"},
			{Note: noteA, Title: "Alien Romulus", TmdbID: 1156593, MediaType: "movie"},
		}

		s, err := LoadStore(noteA)
		if err != nil {
			t.Fatalf("LoadStore() returned error: %v", err)
		}
		for _, idx := range order {
			s.Put(bindings[idx])
		}
		if err := s.Save(); err != nil {
			t.Fatalf("Save() returned error: %v", err)
		}
		data, err := os.ReadFile(s.Path())
		if err != nil {
			t.Fatalf("order %d: reading saved file: %v", i, err)
		}
		results = append(results, data)
	}

	for i := 1; i < len(results); i++ {
		if string(results[i]) != string(results[0]) {
			t.Errorf("save order %v produced different bytes than order %v:\n--- got ---\n%s\n--- want ---\n%s",
				orders[i], orders[0], results[i], results[0])
		}
	}
}

// A sync where nothing moved re-records every line it saw. That must not
// rewrite the file, or every run would touch its mtime and set every
// Syncthing peer rescanning for nothing.
func TestStore_PutIdenticalBindingIsNotAChange(t *testing.T) {
	vault := t.TempDir()
	makeVault(t, vault)
	note := filepath.Join(vault, "Movies.md")
	b := Binding{Note: note, Title: "Arrival", TmdbID: 329865, MediaType: "movie", LastMarker: "✓"}

	s, err := LoadStore(note)
	if err != nil {
		t.Fatalf("LoadStore() returned error: %v", err)
	}
	s.Put(b)
	if err := s.Save(); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(s.Path(), old, old); err != nil {
		t.Fatal(err)
	}

	s2, err := LoadStore(note)
	if err != nil {
		t.Fatalf("LoadStore() returned error: %v", err)
	}
	s2.Put(b)
	if err := s2.Save(); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	info, err := os.Stat(s2.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Errorf("re-putting an identical binding rewrote the state file (mtime %v, want %v)", info.ModTime(), old)
	}
}

func TestStore_SaveNoopWhenUnchanged(t *testing.T) {
	vault := t.TempDir()
	makeVault(t, vault)
	note := filepath.Join(vault, "Movies.md")

	s, err := LoadStore(note)
	if err != nil {
		t.Fatalf("LoadStore() returned error: %v", err)
	}
	s.Put(Binding{Note: note, Title: "Arrival", TmdbID: 329865, MediaType: "movie"})
	if err := s.Save(); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	old := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := os.Chtimes(s.Path(), old, old); err != nil {
		t.Fatal(err)
	}

	// Load fresh (dirty=false) and save again: nothing changed, so the
	// file must not be touched.
	reloaded, err := LoadStore(note)
	if err != nil {
		t.Fatalf("second LoadStore() returned error: %v", err)
	}
	if err := reloaded.Save(); err != nil {
		t.Fatalf("no-op Save() returned error: %v", err)
	}

	info, err := os.Stat(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Errorf("mtime = %v, want unchanged %v (Save() should have been a no-op)", info.ModTime(), old)
	}
}

func TestStore_ReelDirCreatedOnDemand(t *testing.T) {
	vault := t.TempDir()
	makeVault(t, vault)
	note := filepath.Join(vault, "Movies.md")

	reelDir := filepath.Join(vault, ".reel")
	if _, err := os.Stat(reelDir); !os.IsNotExist(err) {
		t.Fatalf(".reel already exists before Save(): %v", err)
	}

	s, err := LoadStore(note)
	if err != nil {
		t.Fatalf("LoadStore() returned error: %v", err)
	}
	s.Put(Binding{Note: note, Title: "Arrival", TmdbID: 329865, MediaType: "movie"})
	if err := s.Save(); err != nil {
		t.Fatalf("Save() returned error: %v", err)
	}

	if info, err := os.Stat(reelDir); err != nil || !info.IsDir() {
		t.Errorf(".reel directory was not created by Save(): %v", err)
	}
}

func TestLoadStore_IgnoresSyncConflictFiles(t *testing.T) {
	vault := t.TempDir()
	makeVault(t, vault)
	note := filepath.Join(vault, "Movies.md")

	reelDir := filepath.Join(vault, ".reel")
	if err := os.MkdirAll(reelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Real sync.toml: one binding.
	realContent := "[[binding]]\nnote = \"Movies.md\"\ntitle = \"arrival\"\ntmdb_id = 329865\nmedia_type = \"movie\"\nrequest_id = 0\nlast_status = \"\"\nlast_marker = \"\"\n"
	if err := os.WriteFile(filepath.Join(reelDir, "sync.toml"), []byte(realContent), 0o644); err != nil {
		t.Fatal(err)
	}
	// A Syncthing conflict copy with a different, misleading binding that
	// must never be read.
	conflictContent := "[[binding]]\nnote = \"Movies.md\"\ntitle = \"arrival\"\ntmdb_id = 999999\nmedia_type = \"movie\"\nrequest_id = 0\nlast_status = \"\"\nlast_marker = \"\"\n"
	conflictName := "sync.sync-conflict-20260101-120000-ABCDEFG.toml"
	if err := os.WriteFile(filepath.Join(reelDir, conflictName), []byte(conflictContent), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := LoadStore(note)
	if err != nil {
		t.Fatalf("LoadStore() returned error: %v", err)
	}
	got, ok := s.Get(note, "arrival")
	if !ok {
		t.Fatalf("Get() did not find binding from sync.toml")
	}
	if got.TmdbID != 329865 {
		t.Errorf("TmdbID = %d, want 329865 (conflict file must be ignored)", got.TmdbID)
	}
}
