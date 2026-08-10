package obsidian

import "strings"

// Marker is reel's understanding of the rune between "[" and "]" on a task
// line. The mapping to/from that literal rune is explicit and total: every
// rune not in the table below still parses to Foreign (see ParseMarker),
// so there is always a Marker to reason about, and it defaults to the
// safest, read-only choice.
type Marker int

const (
	// Unsynced marks a task reel has not yet requested ("[ ]").
	Unsynced Marker = iota
	// Requested marks a task reel has asked Seerr for, not yet available
	// ("[🎬]").
	Requested
	// Downloading marks a task that is downloading or partially available
	// ("[↓]").
	Downloading
	// Available marks a task whose media is fully available in the
	// library ("[✓]").
	Available
	// Failed marks a task whose request was declined, or whose media was
	// since deleted ("[✗]").
	Failed
	// Done marks a task the user completed/opted out of by hand ("[x]" or
	// "[X]", the vanilla Obsidian "checked" state). reel must never write
	// to a Done line.
	Done
	// Foreign marks any marker rune reel doesn't own — cancelled/forwarded
	// states from plugins like Obsidian Tasks, or anything else a user's
	// vocabulary might use. reel must never write to a Foreign line.
	Foreign
)

// runeToMarker holds the known, single-rune mappings. Markers that parse to
// more than one rune (Done) or that match by exclusion (Foreign) are
// handled separately in ParseMarker.
var runeToMarker = map[rune]Marker{
	' ': Unsynced,
	'🎬': Requested,
	'↓': Downloading,
	'✓': Available,
	'✗': Failed,
	'x': Done,
	'X': Done,
}

// markerToRune is the inverse of runeToMarker, used when reel writes a
// marker back into a note. It only needs entries for markers reel actually
// writes; Done and Foreign are never written (see Writable), so they have
// no entry here.
var markerToRune = map[Marker]rune{
	Unsynced:    ' ',
	Requested:   '🎬',
	Downloading: '↓',
	Available:   '✓',
	Failed:      '✗',
}

// ParseMarker maps the literal rune found between "[" and "]" on a task
// line to a Marker. Any rune not in reel's own vocabulary parses to
// Foreign, so unrecognized conventions (from other plugins or users) are
// always treated as read-only rather than silently adopted.
func ParseMarker(r rune) Marker {
	if m, ok := runeToMarker[r]; ok {
		return m
	}
	return Foreign
}

// Rune returns the literal byte(s) reel would write for m. It panics if m
// is Done or Foreign, since reel never writes either — callers must check
// Writable first.
func (m Marker) Rune() rune {
	r, ok := markerToRune[m]
	if !ok {
		panic("obsidian: Rune called on a non-writable Marker")
	}
	return r
}

// Writable reports whether reel is permitted to overwrite a line carrying
// this marker. Done (user-completed) and Foreign (someone else's
// vocabulary) are permanently read-only.
func (m Marker) Writable() bool {
	return m != Done && m != Foreign
}

// String returns a human-readable name for m, used in logs/output.
func (m Marker) String() string {
	switch m {
	case Unsynced:
		return "Unsynced"
	case Requested:
		return "Requested"
	case Downloading:
		return "Downloading"
	case Available:
		return "Available"
	case Failed:
		return "Failed"
	case Done:
		return "Done"
	default:
		return "Foreign"
	}
}

// reelIgnoreToken is the user's permanent per-line opt-out, an Obsidian
// comment so it renders invisibly in reading view.
const reelIgnoreToken = "%%reel:ignore%%"

// TitleInfo is the result of cleaning up the free text that follows a task
// line's marker.
type TitleInfo struct {
	Title   string // cleaned title
	Year    string // "" when no year hint, else 4 digits e.g. "1979"
	Ignored bool   // %%reel:ignore%% was present
}

// ParseTitle extracts a title, optional year hint, and ignore flag from the
// raw text following "] " on a task line. It understands enough Obsidian
// and markdown syntax to recover a clean search title: wikilinks, markdown
// links, trailing hashtags, trailing block IDs, and a trailing
// parenthesized year.
//
// Hashtags and block IDs are stripped before the link is unwrapped, not
// after: both are line-trailing annotations that come *after* a link in
// Obsidian's own syntax ("[[Alien (1979)]] #film ^abc123"), so a wikilink
// or markdown link is only ever found at the very end of the text once its
// trailing annotations are gone. The year, in turn, is only meaningful
// once any wrapping link syntax has been peeled away.
func ParseTitle(text string) TitleInfo {
	var info TitleInfo

	text, info.Ignored = stripIgnoreToken(text)
	text = strings.TrimSpace(text)

	// Hashtags and block IDs can both be present on one line
	// ("Arrival #film ^abc123"), in either order, so strip trailing
	// tokens repeatedly until nothing more comes off.
	for {
		stripped, ok := stripTrailingHashtags(text)
		if !ok {
			stripped, ok = stripTrailingBlockID(text)
		}
		if !ok {
			break
		}
		text = strings.TrimSpace(stripped)
	}

	text = unwrapLink(text)
	text = strings.TrimSpace(text)

	text, info.Year = peelTrailingYear(text)

	// Unwrap once more: a year hint written *outside* the link
	// ("[[Alien]] (1979)") leaves the link non-terminal on the first pass,
	// so it only becomes unwrappable once the year is off the end. Users
	// write it that way deliberately — moving the year inside would point
	// the wikilink at a different note.
	text = unwrapLink(strings.TrimSpace(text))

	info.Title = strings.TrimSpace(text)
	return info
}

// stripIgnoreToken removes a case-insensitive %%reel:ignore%% token from
// text, reporting whether it was present.
func stripIgnoreToken(text string) (string, bool) {
	lower := strings.ToLower(text)
	idx := strings.Index(lower, strings.ToLower(reelIgnoreToken))
	if idx < 0 {
		return text, false
	}
	return text[:idx] + text[idx+len(reelIgnoreToken):], true
}

// unwrapLink unwraps a single leading Obsidian wikilink ("[[Target]]" or
// "[[Target|Display]]", using the display text per Obsidian's own
// rendering) or a single leading markdown link ("[Display](url)"),
// returning its display text. If text isn't wholly one such link, it is
// returned unchanged.
// Both branches require the link to span the whole of text. Unwrapping a
// link with trailing content would silently discard that content — dropping
// the year from "[Alien](url) (1979)", for instance, or reducing
// "[[Alien]] and [[Heat]]" to nonsense. Returning such text unchanged leaves
// it to fail visibly as an unresolvable title instead.
func unwrapLink(text string) string {
	if strings.HasPrefix(text, "[[") && strings.HasSuffix(text, "]]") {
		inner := text[2 : len(text)-2]
		if strings.Contains(inner, "]]") {
			return text
		}
		if pipe := strings.LastIndex(inner, "|"); pipe >= 0 {
			return inner[pipe+1:]
		}
		return inner
	}
	if strings.HasPrefix(text, "[") {
		closeIdx := strings.Index(text, "](")
		if closeIdx > 0 && strings.HasSuffix(text, ")") {
			// A ")" inside the target means the ")" ending text closes
			// something else, so the link doesn't reach the end.
			if target := text[closeIdx+2 : len(text)-1]; !strings.Contains(target, ")") {
				return text[1:closeIdx]
			}
		}
	}
	return text
}

// stripTrailingHashtags removes one or more trailing whitespace-separated
// hashtags (e.g. "#film", "#to-watch", "#a/b") from the end of text. A "#"
// that isn't part of a trailing run of tags (e.g. mid-title) is left
// alone. Reports whether anything was removed.
func stripTrailingHashtags(text string) (string, bool) {
	end := len(text)
	removedAny := false
	for {
		trimmed := strings.TrimRight(text[:end], " \t")
		if trimmed == "" {
			break
		}
		lastSpace := strings.LastIndexAny(trimmed, " \t")
		tag := trimmed[lastSpace+1:]
		if !isHashtag(tag) {
			break
		}
		end = lastSpace + 1
		if lastSpace < 0 {
			end = 0
		}
		removedAny = true
	}
	if !removedAny {
		return text, false
	}
	return text[:end], true
}

// isHashtag reports whether tag looks like an Obsidian hashtag: a "#"
// followed by at least one letter/digit/hyphen/underscore/slash (slash
// supports nested tags like "#a/b").
func isHashtag(tag string) bool {
	if len(tag) < 2 || tag[0] != '#' {
		return false
	}
	for _, r := range tag[1:] {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9',
			r == '-', r == '_', r == '/':
		default:
			return false
		}
	}
	return true
}

// stripTrailingBlockID removes a trailing Obsidian block ID ("^abc123")
// from the end of text, reporting whether one was found.
func stripTrailingBlockID(text string) (string, bool) {
	trimmed := strings.TrimRight(text, " \t")
	caret := strings.LastIndex(trimmed, "^")
	if caret < 0 {
		return text, false
	}
	id := trimmed[caret+1:]
	if id == "" || !isBlockID(id) {
		return text, false
	}
	return trimmed[:caret], true
}

// isBlockID reports whether id is a valid Obsidian block ID body:
// letters, digits, and hyphens.
func isBlockID(id string) bool {
	for _, r := range id {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
		default:
			return false
		}
	}
	return true
}

// peelTrailingYear extracts a trailing "(YYYY)" hint (1800-2999) from the
// end of text, returning the text with it removed and the year as a
// string. A parenthesized group anywhere other than the very end doesn't
// count, so "Se7en (1995) (dir cut)" is left untouched.
func peelTrailingYear(text string) (string, string) {
	trimmed := strings.TrimRight(text, " \t")
	if len(trimmed) < 6 || trimmed[len(trimmed)-1] != ')' {
		return text, ""
	}
	open := strings.LastIndex(trimmed, "(")
	if open < 0 {
		return text, ""
	}
	inner := trimmed[open+1 : len(trimmed)-1]
	if len(inner) != 4 {
		return text, ""
	}
	for _, r := range inner {
		if r < '0' || r > '9' {
			return text, ""
		}
	}
	if inner < "1800" || inner > "2999" {
		return text, ""
	}
	return trimmed[:open], inner
}
