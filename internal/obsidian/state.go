// Package obsidian syncs a checklist of films/shows in an Obsidian vault
// note with Seerr media requests.
package obsidian

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Binding remembers which Seerr media a single checklist line refers to, so
// that reel does not have to re-search (and possibly re-ask the user to
// disambiguate) on every sync. It is the on-disk row of an array-of-tables
// TOML document; see stateFile.
type Binding struct {
	Note       string `toml:"note"`
	Title      string `toml:"title"`
	TmdbID     int    `toml:"tmdb_id"`
	MediaType  string `toml:"media_type"` // "movie" or "tv"
	RequestID  int    `toml:"request_id"` // 0 when reel has no request record for the item
	LastStatus string `toml:"last_status"`
	LastMarker string `toml:"last_marker"`
}

// stateFile is the on-disk shape of sync.toml: a flat array of tables
// rather than a table keyed by note/title, so the file stays line-oriented
// and diffs cleanly.
type stateFile struct {
	Binding []Binding `toml:"binding"`
}

// NormalizeTitle produces the lookup key for a title as written in a note:
// lowercased, trimmed, with internal whitespace runs collapsed to a single
// space. It does nothing cleverer than that — callers are expected to have
// already stripped Markdown links/tags and any trailing year before a title
// reaches this function.
func NormalizeTitle(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// FindVaultRoot walks up from the directory containing path looking for an
// Obsidian vault, identified by a .obsidian directory — the only marker
// Obsidian itself guarantees, and the only one this package trusts. It
// returns "" if no such directory is found before reaching the filesystem
// root. A .obsidian that exists but is a regular file does not count.
//
// path is resolved against the working directory first. Walking a relative
// path directly would stop dead at ".", since filepath.Dir(".") is "." — so
// "reel sync Movies.md" run from a subdirectory of a vault would fail to
// find the vault at all and silently fall back to XDG state.
func FindVaultRoot(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return ""
	}
	dir := filepath.Dir(abs)
	for {
		if info, err := os.Stat(filepath.Join(dir, ".obsidian")); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// bindKey identifies a binding in memory: the note it belongs to (already
// normalized to the stored form) and the normalized title.
type bindKey struct {
	note  string
	title string
}

// Store holds the bindings for a single state file (one Obsidian vault, or
// the XDG fallback for notes outside any vault).
type Store struct {
	path      string
	vaultRoot string // "" for the XDG fallback; note keys are absolute paths in that case
	bindings  map[bindKey]Binding
	dirty     bool
}

// statePath returns where the state file for vaultRoot lives, or the XDG
// state fallback when vaultRoot is "" (the note is not inside any vault).
func statePath(vaultRoot string) (string, error) {
	if vaultRoot != "" {
		return filepath.Join(vaultRoot, ".reel", "sync.toml"), nil
	}
	stateHome := os.Getenv("XDG_STATE_HOME")
	if stateHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		stateHome = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateHome, "reel", "obsidian.toml"), nil
}

// LoadStore locates the state file covering notePath — inside notePath's
// Obsidian vault if it has one, otherwise the XDG state fallback — and
// reads it. A missing file is not an error; it yields an empty Store whose
// directory is created lazily on the first Save.
//
// Only sync.toml itself is read. Syncthing (which this vault is synced
// with) does not merge conflicting writes from two machines: it keeps one
// version and sidelines the other as
// sync.sync-conflict-<date>-<time>-<device>.toml. Such files are never ours
// to interpret — reading one could silently rebind a line to stale data,
// whereas ignoring it just means reel re-resolves the binding, which is
// benign.
func LoadStore(notePath string) (*Store, error) {
	vaultRoot := FindVaultRoot(notePath)
	path, err := statePath(vaultRoot)
	if err != nil {
		return nil, err
	}

	s := &Store{
		path:      path,
		vaultRoot: vaultRoot,
		bindings:  make(map[bindKey]Binding),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return nil, fmt.Errorf("obsidian: reading state file %s: %w", path, err)
	}

	var file stateFile
	if err := toml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("obsidian: parsing state file %s: %w", path, err)
	}
	for _, b := range file.Binding {
		s.bindings[bindKey{b.Note, b.Title}] = b
	}
	return s, nil
}

// noteKey converts a note path as the caller knows it (typically absolute)
// into the form a Binding stores it in: relative to the vault root when
// this store belongs to a vault — so the file stays portable across
// machines that have the vault checked out at different paths — or an
// absolute path in the XDG fallback case.
// It resolves notePath against the working directory for the same reason
// FindVaultRoot does: vaultRoot is absolute, so filepath.Rel would fail on a
// relative note path and fall through to storing an unstable, cwd-dependent
// key.
func (s *Store) noteKey(notePath string) string {
	abs, err := filepath.Abs(notePath)
	if err != nil {
		abs = filepath.Clean(notePath)
	}
	if s.vaultRoot == "" {
		return abs
	}
	if rel, err := filepath.Rel(s.vaultRoot, abs); err == nil {
		return rel
	}
	return abs
}

// Get returns the binding recorded for the given note and title, if any.
// notePath and title are taken as the caller knows/wrote them and are
// normalized internally.
func (s *Store) Get(notePath, title string) (Binding, bool) {
	b, ok := s.bindings[bindKey{s.noteKey(notePath), NormalizeTitle(title)}]
	return b, ok
}

// Put records or replaces the binding for b.Note/b.Title. Those two fields
// are taken as the caller knows/wrote them (not yet normalized) and are
// rewritten in place to their stored form before b is kept.
//
// Storing a binding identical to the one already held is not a change, and
// deliberately does not mark the store dirty: a sync where nothing moved
// re-records every line it saw, and rewriting the file for that would touch
// its mtime on every run and set every Syncthing peer rescanning for
// nothing.
func (s *Store) Put(b Binding) {
	b.Note = s.noteKey(b.Note)
	b.Title = NormalizeTitle(b.Title)
	key := bindKey{b.Note, b.Title}
	if old, ok := s.bindings[key]; ok && old == b {
		return
	}
	s.bindings[key] = b
	s.dirty = true
}

// Delete removes the binding for the given note and title, if one exists.
func (s *Store) Delete(notePath, title string) {
	key := bindKey{s.noteKey(notePath), NormalizeTitle(title)}
	if _, ok := s.bindings[key]; ok {
		delete(s.bindings, key)
		s.dirty = true
	}
}

// Path returns the file this Store will write to on Save.
func (s *Store) Path() string {
	return s.path
}

// Save writes the store to disk, atomically, unless nothing has changed
// since it was loaded (in which case it does nothing).
//
// Bindings are sorted by note then title before marshaling. Go map
// iteration order is randomized, and this state file lives inside a vault
// synced by Syncthing, which does not merge conflicting writes — it keeps
// one file and sidelines the other as
// sync.sync-conflict-<date>-<time>-<device>.toml. Marshaling bindings in
// random order would rewrite the whole file on every save and propagate
// that as a full-file change to every synced machine, widening the window
// in which two machines save concurrently and collide. Deterministic
// ordering keeps writes limited to the lines that actually changed.
func (s *Store) Save() error {
	if !s.dirty {
		return nil
	}

	bindings := make([]Binding, 0, len(s.bindings))
	for _, b := range s.bindings {
		bindings = append(bindings, b)
	}
	sort.Slice(bindings, func(i, j int) bool {
		if bindings[i].Note != bindings[j].Note {
			return bindings[i].Note < bindings[j].Note
		}
		return bindings[i].Title < bindings[j].Title
	})

	data, err := toml.Marshal(stateFile{Binding: bindings})
	if err != nil {
		return fmt.Errorf("obsidian: encoding state file: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("obsidian: creating %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".sync.toml.tmp-*")
	if err != nil {
		return fmt.Errorf("obsidian: creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op once the rename below succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("obsidian: writing temp file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("obsidian: syncing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("obsidian: closing temp file: %w", err)
	}
	if err := os.Rename(tmpPath, s.path); err != nil {
		return fmt.Errorf("obsidian: renaming temp file into place: %w", err)
	}

	s.dirty = false
	return nil
}
