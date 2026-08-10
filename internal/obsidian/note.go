package obsidian

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// taskLineRe matches a markdown task list item: optional leading
// whitespace, a bullet, one or more spaces, a single-rune checkbox, then
// whitespace and the rest of the line. It's applied to a line's content
// with its terminator already stripped, so "$" anchors to the true end of
// the line rather than requiring a literal newline in the input.
var taskLineRe = regexp.MustCompile(`^[ \t]*[-*+] +\[(.)\][ \t]+(.*)$`)

// TaskLine is one parsed "- [ ] Title" line of a Note.
type TaskLine struct {
	LineIndex  int    // index into Note.Lines
	Raw        string // the complete original line INCLUDING its line ending
	Marker     Marker
	MarkerByte int // byte offset of the marker rune within Raw
	Title      string
	Year       string
	Ignored    bool
}

// Note is a parsed markdown file: every line verbatim (each carrying its
// own terminator, or none for a final unterminated line), plus the subset
// recognized as task lines. strings.Join(note.Lines, "") always reproduces
// the source file byte-for-byte.
type Note struct {
	Path  string
	Lines []string
	Tasks []TaskLine
}

// ParseNote reads and parses the note at path.
func ParseNote(path string) (*Note, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("obsidian: reading %s: %w", path, err)
	}
	return ParseBytes(path, data), nil
}

// ParseBytes parses note content already in memory. It's split out from
// ParseNote so tests can exercise parsing without touching the filesystem;
// path is stored on the result but otherwise unused.
func ParseBytes(path string, data []byte) *Note {
	note := &Note{Path: path, Lines: splitLines(data)}

	// Fence state persists across lines: once inside a ``` or ~~~ block,
	// nothing is a task line until a matching close, no matter what it
	// looks like.
	var inFence bool
	var fenceChar byte
	var fenceLen int

	for i, line := range note.Lines {
		content := stripLineEnding(line)
		trimmed := strings.TrimLeft(content, " \t")

		if inFence {
			if ch, n := fenceRun(trimmed); n > 0 && ch == fenceChar && n >= fenceLen {
				inFence = false
			}
			continue
		}
		if ch, n := fenceRun(trimmed); n > 0 {
			inFence, fenceChar, fenceLen = true, ch, n
			continue
		}

		m := taskLineRe.FindStringSubmatchIndex(content)
		if m == nil {
			continue
		}
		markerStart, markerEnd := m[2], m[3]
		restStart, restEnd := m[4], m[5]

		r, _ := utf8.DecodeRuneInString(content[markerStart:markerEnd])
		title := ParseTitle(content[restStart:restEnd])
		note.Tasks = append(note.Tasks, TaskLine{
			LineIndex:  i,
			Raw:        line,
			Marker:     ParseMarker(r),
			MarkerByte: markerStart,
			Title:      title.Title,
			Year:       title.Year,
			Ignored:    title.Ignored,
		})
	}
	return note
}

// splitLines splits data into lines, each retaining its own terminator
// ("\n", "\r\n", or none for a trailing partial line). strings.Split
// discards this information, but write-back needs it to reproduce the
// file exactly, so this is done by hand.
func splitLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	var lines []string
	start := 0
	for i, b := range data {
		if b == '\n' {
			lines = append(lines, string(data[start:i+1]))
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, string(data[start:]))
	}
	return lines
}

// stripLineEnding returns line with any trailing "\r\n" or "\n" removed.
func stripLineEnding(line string) string {
	if strings.HasSuffix(line, "\r\n") {
		return line[:len(line)-2]
	}
	return strings.TrimSuffix(line, "\n")
}

// fenceRun reports the fence character and run length at the start of
// trimmed (leading whitespace already removed), if any. A run shorter
// than 3 isn't a fence delimiter per CommonMark.
func fenceRun(trimmed string) (ch byte, n int) {
	if trimmed == "" {
		return 0, 0
	}
	c := trimmed[0]
	if c != '`' && c != '~' {
		return 0, 0
	}
	for n < len(trimmed) && trimmed[n] == c {
		n++
	}
	if n < 3 {
		return 0, 0
	}
	return c, n
}

// Edit is a request to change one task line's marker.
type Edit struct {
	OriginalLine string // must equal the TaskLine.Raw it came from
	NewMarker    Marker
}

// WriteResult reports what WriteEdits actually did, since some edits may
// legitimately not apply (the note changed under us, or the target line
// turned out to be read-only).
type WriteResult struct {
	Applied []Edit
	Skipped []Edit
}

// ErrRecentlyModified is returned by WriteEdits when the note's mtime is
// too fresh to write safely. Obsidian autosaves a couple of seconds after
// the user stops typing, so a write landing right after an on-disk change
// risks racing Obsidian's own in-memory buffer and being clobbered (or
// clobbering it). Skipping narrows that window; it can't close it
// entirely.
var ErrRecentlyModified = errors.New("obsidian: note modified within the last 3s, skipping to avoid racing an editor")

// recentThreshold is how fresh a file's mtime must be for WriteEdits to
// refuse to write.
const recentThreshold = 3 * time.Second

// WriteEdits applies edits to the note at path, changing only the marker
// byte(s) of each targeted line and leaving every other byte of the file
// untouched.
//
// Edits are matched against a fresh read of the file, not anything an
// earlier ParseNote call saw: network calls happen between parsing and
// writing, and the user may have edited the note in the meantime. Matching
// is by full line text rather than line number, so unrelated insertions,
// deletions, or reordering elsewhere in the file don't cause misapplied
// edits. If OriginalLine isn't found, or its target line's current marker
// is Done or Foreign, that edit is skipped and the rest still proceed —
// partial application is expected, not an error.
//
// If nothing ends up applied, the file is not written at all (no mtime
// churn). Otherwise the write is atomic: a temp file in the same
// directory is written, synced, chmod'd to match the original, and
// renamed over it.
func WriteEdits(path string, edits []Edit) (WriteResult, error) {
	if len(edits) == 0 {
		return WriteResult{}, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return WriteResult{}, fmt.Errorf("obsidian: stat %s: %w", path, err)
	}
	// Only a recently-*past* mtime means an editor may be mid-save. A future
	// mtime means the clock is wrong — plausible here, since the vault is
	// synced across machines by Syncthing and file times come with it. The
	// heuristic tells us nothing in that case, and treating it as "too
	// fresh" would refuse every write until wall-clock time caught up,
	// turning a race-narrowing guard into a silent, permanent outage.
	if age := time.Since(info.ModTime()); age >= 0 && age < recentThreshold {
		return WriteResult{}, ErrRecentlyModified
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return WriteResult{}, fmt.Errorf("obsidian: reading %s: %w", path, err)
	}
	fresh := ParseBytes(path, data)

	// Index task occurrences by their exact raw text, in file order, so
	// duplicate lines can be matched positionally: the Nth edit for a
	// given text claims the Nth occurrence in the file.
	occurrences := map[string][]int{} // raw line text -> indices into fresh.Tasks
	for i, t := range fresh.Tasks {
		occurrences[t.Raw] = append(occurrences[t.Raw], i)
	}

	type pendingWrite struct {
		markerByte int
		oldLen     int // byte length of the marker rune currently on disk
		newRune    rune
	}
	pending := map[int]pendingWrite{} // Note.Lines index -> replacement

	var result WriteResult
	claimed := map[string]int{} // raw line text -> how many occurrences consumed so far
	for _, e := range edits {
		occ := occurrences[e.OriginalLine]
		n := claimed[e.OriginalLine]
		claimed[e.OriginalLine]++
		// Guard the marker being written, not just the one already on the
		// line: Marker.Rune panics on Done and Foreign, and skipping is the
		// right answer for an edit reel would never legitimately make.
		// Checked after the claim counter advances so positional matching of
		// duplicate lines stays stable regardless of outcome.
		if !e.NewMarker.Writable() {
			result.Skipped = append(result.Skipped, e)
			continue
		}
		if n >= len(occ) {
			result.Skipped = append(result.Skipped, e)
			continue
		}
		task := fresh.Tasks[occ[n]]
		if !task.Marker.Writable() {
			result.Skipped = append(result.Skipped, e)
			continue
		}
		_, oldLen := utf8.DecodeRuneInString(task.Raw[task.MarkerByte:])
		pending[task.LineIndex] = pendingWrite{
			markerByte: task.MarkerByte,
			oldLen:     oldLen,
			newRune:    e.NewMarker.Rune(),
		}
		result.Applied = append(result.Applied, e)
	}

	if len(result.Applied) == 0 {
		return result, nil
	}

	var buf bytes.Buffer
	for i, line := range fresh.Lines {
		pw, ok := pending[i]
		if !ok {
			buf.WriteString(line)
			continue
		}
		buf.WriteString(line[:pw.markerByte])
		buf.WriteRune(pw.newRune)
		buf.WriteString(line[pw.markerByte+pw.oldLen:])
	}

	if err := writeFileAtomic(path, buf.Bytes(), info.Mode()); err != nil {
		return result, err
	}
	return result, nil
}

// writeFileAtomic writes data to path by way of a temp file in the same
// directory (required for os.Rename to be atomic on the same filesystem),
// so a crash or power loss mid-write can never leave path truncated or
// half-written. The temp name is dot-prefixed so it stays hidden from
// Obsidian's file list if a crash leaves it behind.
func writeFileAtomic(path string, data []byte, mode os.FileMode) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".reel-tmp-*")
	if err != nil {
		return fmt.Errorf("obsidian: creating temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		if err != nil {
			os.Remove(tmpPath)
		}
	}()

	if _, err = tmp.Write(data); err != nil {
		tmp.Close()
		return fmt.Errorf("obsidian: writing temp file: %w", err)
	}
	if err = tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("obsidian: syncing temp file: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("obsidian: closing temp file: %w", err)
	}
	if err = os.Chmod(tmpPath, mode); err != nil {
		return fmt.Errorf("obsidian: chmod temp file: %w", err)
	}
	if err = os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("obsidian: renaming temp file: %w", err)
	}
	return nil
}
