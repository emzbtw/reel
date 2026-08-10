package obsidian

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- round-trip: parse then rejoin must reproduce the input exactly ---

func TestParseBytes_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{"empty file", ""},
		{"LF only", "# Movies\n- [ ] Arrival\n- [ ] Heat\n"},
		{"CRLF only", "# Movies\r\n- [ ] Arrival\r\n- [ ] Heat\r\n"},
		{"mixed line endings", "# Movies\r\n- [ ] Arrival\n- [ ] Heat\r\n"},
		{"no trailing newline", "# Movies\n- [ ] Arrival\n- [ ] Heat"},
		{"no trailing newline, CRLF", "# Movies\r\n- [ ] Arrival\r\n- [ ] Heat"},
		{"single line no newline", "- [ ] Arrival"},
		{"only a newline", "\n"},
		{"blank lines", "# Movies\n\n- [ ] Arrival\n\n"},
		{"trailing blank line", "- [ ] Arrival\n\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			note := ParseBytes("note.md", []byte(tt.data))
			got := strings.Join(note.Lines, "")
			if got != tt.data {
				t.Errorf("round trip = %q, want %q", got, tt.data)
			}
		})
	}
}

// --- frontmatter is untouched and not parsed as tasks ---

func TestParseBytes_Frontmatter(t *testing.T) {
	data := "---\n" +
		"tags: [movies]\n" +
		"created: 2024-01-01\n" +
		"---\n" +
		"# Movies\n" +
		"- [ ] Arrival\n"
	note := ParseBytes("note.md", []byte(data))

	if got := strings.Join(note.Lines, ""); got != data {
		t.Fatalf("round trip = %q, want %q", got, data)
	}
	if len(note.Tasks) != 1 {
		t.Fatalf("Tasks = %+v, want exactly 1 task (frontmatter must not parse as tasks)", note.Tasks)
	}
	if note.Tasks[0].Title != "Arrival" {
		t.Errorf("Tasks[0].Title = %q, want %q", note.Tasks[0].Title, "Arrival")
	}
}

// --- indented / nested task lists ---

func TestParseBytes_IndentedTasks(t *testing.T) {
	data := "- [ ] Arrival\n" +
		"  - [ ] Heat\n" +
		"\t- [x] Se7en\n" +
		"    - [ ] Alien\n"
	note := ParseBytes("note.md", []byte(data))

	want := []string{"Arrival", "Heat", "Se7en", "Alien"}
	if len(note.Tasks) != len(want) {
		t.Fatalf("Tasks = %+v, want %d tasks", note.Tasks, len(want))
	}
	for i, title := range want {
		if note.Tasks[i].Title != title {
			t.Errorf("Tasks[%d].Title = %q, want %q", i, note.Tasks[i].Title, title)
		}
	}
}

// --- fenced code blocks are not scanned for tasks ---

func TestParseBytes_FencedCodeBlockNotTasks(t *testing.T) {
	data := "- [ ] Arrival\n" +
		"```\n" +
		"- [ ] not a task\n" +
		"```\n" +
		"- [ ] Heat\n" +
		"~~~\n" +
		"- [ ] also not a task\n" +
		"~~~\n" +
		"- [ ] Alien\n"
	note := ParseBytes("note.md", []byte(data))

	want := []string{"Arrival", "Heat", "Alien"}
	if len(note.Tasks) != len(want) {
		var got []string
		for _, tk := range note.Tasks {
			got = append(got, tk.Title)
		}
		t.Fatalf("Tasks titles = %v, want %v", got, want)
	}
	for i, title := range want {
		if note.Tasks[i].Title != title {
			t.Errorf("Tasks[%d].Title = %q, want %q", i, note.Tasks[i].Title, title)
		}
	}
}

func TestParseBytes_FencedCodeBlockWithLanguage(t *testing.T) {
	data := "```go\n" +
		"- [ ] not a task\n" +
		"```\n" +
		"- [ ] Arrival\n"
	note := ParseBytes("note.md", []byte(data))
	if len(note.Tasks) != 1 || note.Tasks[0].Title != "Arrival" {
		t.Fatalf("Tasks = %+v, want exactly one task titled Arrival", note.Tasks)
	}
}

// --- markers, including multibyte, with byte-accurate MarkerByte ---

func TestParseBytes_Markers(t *testing.T) {
	tests := []struct {
		name   string
		marker string // the literal rune(s) between [ and ]
		want   Marker
	}{
		{"unsynced", " ", Unsynced},
		{"requested", "🎬", Requested},
		{"downloading", "↓", Downloading},
		{"available", "✓", Available},
		{"failed", "✗", Failed},
		{"done lowercase", "x", Done},
		{"done uppercase", "X", Done},
		{"foreign cancelled", "-", Foreign},
		{"foreign in progress", "/", Foreign},
		{"foreign forwarded", ">", Foreign},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := "- [" + tt.marker + "] Arrival\n"
			note := ParseBytes("note.md", []byte(data))
			if len(note.Tasks) != 1 {
				t.Fatalf("Tasks = %+v, want exactly 1", note.Tasks)
			}
			task := note.Tasks[0]
			if task.Marker != tt.want {
				t.Errorf("Marker = %v, want %v", task.Marker, tt.want)
			}
			// MarkerByte must point exactly at the marker rune within Raw.
			gotRune := task.Raw[task.MarkerByte : task.MarkerByte+len(tt.marker)]
			if gotRune != tt.marker {
				t.Errorf("Raw[MarkerByte:...] = %q, want %q (Raw=%q, MarkerByte=%d)", gotRune, tt.marker, task.Raw, task.MarkerByte)
			}
		})
	}
}

// --- write-back ---

// setMTime sets both atime and mtime of path to t (in the past, by
// default, well outside the 3s recently-modified window), returning the
// value set so callers can assert against it.
func setMTime(t *testing.T, path string, when time.Time) {
	t.Helper()
	if err := os.Chtimes(path, when, when); err != nil {
		t.Fatalf("Chtimes(%s): %v", path, err)
	}
}

// old is a timestamp safely outside WriteEdits' 3s recently-modified
// window, used by tests that don't care about mtime behavior itself.
var old = time.Now().Add(-1 * time.Hour)

func writeTestNote(t *testing.T, data string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "Movies.md")
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	setMTime(t, path, old)
	return path
}

func TestWriteEdits_HappyPath_ChangesOnlyMarkerBytes(t *testing.T) {
	before := "# Movies\n- [ ] Arrival\n- [ ] Heat\n"
	path := writeTestNote(t, before)

	result, err := WriteEdits(path, []Edit{
		{OriginalLine: "- [ ] Arrival\n", NewMarker: Available},
	})
	if err != nil {
		t.Fatalf("WriteEdits() returned error: %v", err)
	}
	if len(result.Applied) != 1 || len(result.Skipped) != 0 {
		t.Fatalf("result = %+v, want 1 applied, 0 skipped", result)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "# Movies\n- [✓] Arrival\n- [ ] Heat\n"
	if string(after) != want {
		t.Fatalf("file after write = %q, want %q", after, want)
	}

	// Exactly the marker bytes changed: " " (1 byte) -> "✓" (3 bytes), a
	// net +2 bytes, and nothing else in the file moved.
	if len(after)-len(before) != 2 {
		t.Errorf("byte length delta = %d, want 2", len(after)-len(before))
	}
}

func TestWriteEdits_LineChangedOnDisk_IsSkipped(t *testing.T) {
	path := writeTestNote(t, "- [ ] Arrival\n- [ ] Heat\n")

	// Simulate an edit made to the note (by the user, in another
	// process) after WriteEdits' caller originally parsed it: the
	// Arrival line's text on disk no longer matches what the edit
	// expects.
	if err := os.WriteFile(path, []byte("- [ ] Arrival (extra)\n- [ ] Heat\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	setMTime(t, path, old)

	result, err := WriteEdits(path, []Edit{
		{OriginalLine: "- [ ] Arrival\n", NewMarker: Available},
		{OriginalLine: "- [ ] Heat\n", NewMarker: Downloading},
	})
	if err != nil {
		t.Fatalf("WriteEdits() returned error: %v", err)
	}
	if len(result.Applied) != 1 || len(result.Skipped) != 1 {
		t.Fatalf("result = %+v, want 1 applied, 1 skipped", result)
	}
	if result.Skipped[0].OriginalLine != "- [ ] Arrival\n" {
		t.Errorf("Skipped[0] = %+v, want the Arrival edit", result.Skipped[0])
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "- [ ] Arrival (extra)\n- [↓] Heat\n"
	if string(after) != want {
		t.Fatalf("file after write = %q, want %q", after, want)
	}
}

func TestWriteEdits_DuplicateLinesMatchPositionally(t *testing.T) {
	path := writeTestNote(t, "- [ ] Heat\n- [ ] Heat\n- [ ] Heat\n")

	result, err := WriteEdits(path, []Edit{
		{OriginalLine: "- [ ] Heat\n", NewMarker: Available},
		{OriginalLine: "- [ ] Heat\n", NewMarker: Downloading},
	})
	if err != nil {
		t.Fatalf("WriteEdits() returned error: %v", err)
	}
	if len(result.Applied) != 2 || len(result.Skipped) != 0 {
		t.Fatalf("result = %+v, want 2 applied, 0 skipped", result)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "- [✓] Heat\n- [↓] Heat\n- [ ] Heat\n"
	if string(after) != want {
		t.Fatalf("file after write = %q, want %q", after, want)
	}
}

func TestWriteEdits_DuplicateLines_ExcessEditIsSkipped(t *testing.T) {
	path := writeTestNote(t, "- [ ] Heat\n")

	result, err := WriteEdits(path, []Edit{
		{OriginalLine: "- [ ] Heat\n", NewMarker: Available},
		{OriginalLine: "- [ ] Heat\n", NewMarker: Downloading},
	})
	if err != nil {
		t.Fatalf("WriteEdits() returned error: %v", err)
	}
	if len(result.Applied) != 1 || len(result.Skipped) != 1 {
		t.Fatalf("result = %+v, want 1 applied, 1 skipped", result)
	}
}

func TestWriteEdits_RefusesDoneAndForeign(t *testing.T) {
	path := writeTestNote(t, "- [x] Arrival\n- [-] Heat\n- [ ] Alien\n")

	result, err := WriteEdits(path, []Edit{
		{OriginalLine: "- [x] Arrival\n", NewMarker: Available},
		{OriginalLine: "- [-] Heat\n", NewMarker: Available},
		{OriginalLine: "- [ ] Alien\n", NewMarker: Available},
	})
	if err != nil {
		t.Fatalf("WriteEdits() returned error: %v", err)
	}
	if len(result.Applied) != 1 || len(result.Skipped) != 2 {
		t.Fatalf("result = %+v, want 1 applied, 2 skipped", result)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	want := "- [x] Arrival\n- [-] Heat\n- [✓] Alien\n"
	if string(after) != want {
		t.Fatalf("file after write = %q, want %q", after, want)
	}
}

func TestWriteEdits_RecentlyModified(t *testing.T) {
	path := writeTestNote(t, "- [ ] Arrival\n")
	setMTime(t, path, time.Now().Add(-1*time.Second))

	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	_, err = WriteEdits(path, []Edit{{OriginalLine: "- [ ] Arrival\n", NewMarker: Available}})
	if err != ErrRecentlyModified {
		t.Fatalf("err = %v, want ErrRecentlyModified", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(after) != string(before) {
		t.Fatalf("file was modified despite ErrRecentlyModified: before=%q after=%q", before, after)
	}
}

// A future mtime means the clock is wrong, not that an editor is mid-save —
// plausible when Syncthing carries file times over from a machine with a
// skewed clock. Blocking on it would refuse every write until wall-clock
// time caught up, silently stalling sync indefinitely.
func TestWriteEdits_FutureMTimeStillWrites(t *testing.T) {
	path := writeTestNote(t, "- [ ] Arrival\n")
	setMTime(t, path, time.Now().Add(1*time.Hour))

	result, err := WriteEdits(path, []Edit{{OriginalLine: "- [ ] Arrival\n", NewMarker: Available}})
	if err != nil {
		t.Fatalf("WriteEdits() returned error: %v", err)
	}
	if len(result.Applied) != 1 {
		t.Fatalf("result = %+v, want 1 applied", result)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(after) != "- [✓] Arrival\n" {
		t.Errorf("file after write = %q, want the marker updated", after)
	}
}

// Marker.Rune panics on Done/Foreign, so WriteEdits must reject an edit
// carrying one rather than let a caller crash the process.
func TestWriteEdits_NonWritableNewMarkerIsSkipped(t *testing.T) {
	path := writeTestNote(t, "- [ ] Arrival\n- [ ] Heat\n")
	setMTime(t, path, time.Now().Add(-1*time.Hour))

	result, err := WriteEdits(path, []Edit{
		{OriginalLine: "- [ ] Arrival\n", NewMarker: Done},
		{OriginalLine: "- [ ] Heat\n", NewMarker: Foreign},
	})
	if err != nil {
		t.Fatalf("WriteEdits() returned error: %v", err)
	}
	if len(result.Applied) != 0 || len(result.Skipped) != 2 {
		t.Fatalf("result = %+v, want 0 applied, 2 skipped", result)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(after) != "- [ ] Arrival\n- [ ] Heat\n" {
		t.Errorf("file = %q, want it untouched", after)
	}
}

func TestWriteEdits_EmptyEdits_FileUntouched(t *testing.T) {
	path := writeTestNote(t, "- [ ] Arrival\n")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	wantMTime := info.ModTime()

	result, err := WriteEdits(path, nil)
	if err != nil {
		t.Fatalf("WriteEdits() returned error: %v", err)
	}
	if len(result.Applied) != 0 || len(result.Skipped) != 0 {
		t.Fatalf("result = %+v, want empty", result)
	}

	info, err = os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.ModTime().Equal(wantMTime) {
		t.Errorf("mtime changed: before=%v after=%v", wantMTime, info.ModTime())
	}
}

func TestWriteEdits_AllSkipped_FileUntouched(t *testing.T) {
	path := writeTestNote(t, "- [x] Arrival\n")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	wantMTime := info.ModTime()

	result, err := WriteEdits(path, []Edit{
		{OriginalLine: "- [x] Arrival\n", NewMarker: Available},
	})
	if err != nil {
		t.Fatalf("WriteEdits() returned error: %v", err)
	}
	if len(result.Applied) != 0 || len(result.Skipped) != 1 {
		t.Fatalf("result = %+v, want 0 applied, 1 skipped", result)
	}

	info, err = os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if !info.ModTime().Equal(wantMTime) {
		t.Errorf("mtime changed despite no applied edits: before=%v after=%v", wantMTime, info.ModTime())
	}
}

func TestWriteEdits_PreservesFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Movies.md")
	if err := os.WriteFile(path, []byte("- [ ] Arrival\n"), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	setMTime(t, path, old)

	if _, err := WriteEdits(path, []Edit{{OriginalLine: "- [ ] Arrival\n", NewMarker: Available}}); err != nil {
		t.Fatalf("WriteEdits() returned error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestWriteEdits_AtomicWrite_NoTempFileLeftBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Movies.md")
	if err := os.WriteFile(path, []byte("- [ ] Arrival\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	setMTime(t, path, old)

	if _, err := WriteEdits(path, []Edit{{OriginalLine: "- [ ] Arrival\n", NewMarker: Available}}); err != nil {
		t.Fatalf("WriteEdits() returned error: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "Movies.md" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("dir entries = %v, want exactly [Movies.md]", names)
	}
}

func TestWriteEdits_NoEditsDoesNotOpenFile(t *testing.T) {
	// A nonexistent path must not produce an error when edits is empty:
	// WriteEdits must return before ever touching the filesystem.
	result, err := WriteEdits(filepath.Join(t.TempDir(), "does-not-exist.md"), nil)
	if err != nil {
		t.Fatalf("WriteEdits() returned error: %v", err)
	}
	if len(result.Applied) != 0 || len(result.Skipped) != 0 {
		t.Fatalf("result = %+v, want empty", result)
	}
}
