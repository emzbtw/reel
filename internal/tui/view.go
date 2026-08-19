package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/emzbtw/reel/internal/models"
)

// Palette, matching Seerr's own status semantics for the status colors:
// pending/requested (🎬), processing/downloading (↓), available (✓),
// declined/failed (✗).
const (
	colorMagenta    = lipgloss.Color("#BB9AF7") // selected row: text + selection arrow
	colorMuted      = lipgloss.Color("#565F89") // secondary text: metadata, hints, page numbers; header background
	colorHeaderFg   = lipgloss.Color("#C0CAF5") // header text, for contrast against colorMuted
	colorPending    = lipgloss.Color("#E0AF68")
	colorProcessing = lipgloss.Color("#7AA2F7")
	colorAvailable  = lipgloss.Color("#9ECE6A")
	colorDeclined   = lipgloss.Color("#F7768E")
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Background(colorMuted).Foreground(colorHeaderFg).Padding(0, 1)
	// listStyle's left padding matches headerStyle's own, so list rows
	// start under "reel" the same way "Search:" and error/success lines
	// do. The delegate title/desc styles in newModel add the rest of the
	// indent that lands title text under the header's "—".
	listStyle    = lipgloss.NewStyle().PaddingLeft(1)
	searchStyle  = lipgloss.NewStyle().Padding(0, 1) // same left padding as headerStyle, so "Search:" lines up with "reel —"
	footerStyle  = lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 1)
	errorStyle   = lipgloss.NewStyle().Foreground(colorDeclined).Padding(0, 1)
	successStyle = lipgloss.NewStyle().Foreground(colorAvailable).Padding(0, 1)
	boxStyle     = lipgloss.NewStyle().Padding(1, 2)
	promptStyle  = lipgloss.NewStyle().Bold(true).Padding(1, 2) // same horizontal padding as boxStyle, so the confirm prompt lines up under the box above it
	titleStyle   = lipgloss.NewStyle().Bold(true)
	mutedStyle   = lipgloss.NewStyle().Foreground(colorMuted)
)

// selectionMarker marks the selected list row in place of a full-height
// border bar — just a marker on the title line. lipgloss's left border
// only supports one column per rendered line (a multi-character string
// cycles one character per *line* of a multi-line box, not multiple
// columns on the same line), so this is necessarily a single character.
var selectionMarker = lipgloss.Border{Left: ">"}

// fallbackWidth/fallbackHeight are used only for the brief window before the
// first tea.WindowSizeMsg arrives (or in tests that never send one) — real
// usage always has a window size by the time anything is rendered.
const fallbackWidth = 80
const fallbackHeight = 24

// minContentWidth keeps wrapping sane on a genuinely tiny terminal instead
// of collapsing text to near-zero columns.
const minContentWidth = 20

// termWidth is the terminal width to render against: m.width once a
// WindowSizeMsg has arrived, otherwise fallbackWidth.
func (m model) termWidth() int {
	if m.width > 0 {
		return m.width
	}
	return fallbackWidth
}

// termHeight mirrors termWidth for vertical sizing.
func (m model) termHeight() int {
	if m.height > 0 {
		return m.height
	}
	return fallbackHeight
}

// sized returns style with its Width set so it (including its own border,
// if any) renders no wider than the terminal — the same contract
// list.SetSize already gives the list in update.go, applied to every other
// style so no screen bleeds past the actual window size. Content that
// doesn't fit wraps onto additional lines.
func (m model) sized(style lipgloss.Style) lipgloss.Style {
	w := m.termWidth() - style.GetHorizontalBorderSize()
	if w < minContentWidth {
		w = minContentWidth
	}
	return style.Width(w)
}

// sizedLine is sized's counterpart for the header and footer: they're
// single-line status bars, so content that doesn't fit is truncated rather
// than wrapped onto a second line — View()'s fixed-height layout assumes
// exactly one row each.
func (m model) sizedLine(style lipgloss.Style) lipgloss.Style {
	w := m.termWidth()
	if w < minContentWidth {
		w = minContentWidth
	}
	return style.MaxWidth(w)
}

// View renders header, body, and footer stacked to fill exactly
// termHeight() rows, so the footer stays anchored to the bottom of the
// screen on every mode instead of trailing directly under whatever the body
// happens to render (which is as short as one line on e.g. the search or
// result screens).
func (m model) View() string {
	header := m.headerView()
	footer := m.footerView()

	h := m.termHeight() - chromeLines
	if h < 1 {
		h = 1
	}
	body := lipgloss.Place(m.termWidth(), h, lipgloss.Left, lipgloss.Top, m.bodyView())

	return lipgloss.JoinVertical(lipgloss.Left, header, "", body, footer)
}

func (m model) headerView() string {
	switch m.mode {
	case modeSearch:
		return m.sizedLine(headerStyle).Render("reel — Search")
	case modeRequests, modeRequestDetail, modeRequestConfirm, modeRequestResult:
		return m.sizedLine(headerStyle).Render("reel — Requests")
	}
	if m.source == sourceSearch {
		return m.sizedLine(headerStyle).Render("reel — Search: " + m.query)
	}

	mt := "Movies"
	if m.mediaType == models.MediaTV {
		mt = "TV"
	}
	return m.sizedLine(headerStyle).Render("reel — Discover · " + mt)
}

func (m model) bodyView() string {
	if m.loading {
		return fmt.Sprintf(" %s Loading…", m.spinner.View())
	}
	if m.err != nil && m.mode != modeResult && m.mode != modeRequestResult {
		return m.sized(errorStyle).Render("Error: " + m.err.Error())
	}

	switch m.mode {
	case modeSearch:
		return m.searchView()
	case modeDetail, modeConfirm:
		return m.detailView()
	case modeResult:
		return m.resultView()
	case modeRequestDetail, modeRequestConfirm:
		return m.requestDetailView()
	case modeRequestResult:
		return m.requestResultView()
	default:
		return listStyle.Render(m.list.View())
	}
}

func (m model) searchView() string {
	return m.sized(searchStyle).Render("Search: " + m.searchInput.View())
}

func (m model) detailView() string {
	it := m.selected
	var b strings.Builder
	b.WriteString(titleStyle.Render(it.title))
	if it.year != "" {
		fmt.Fprintf(&b, " %s", mutedStyle.Render(fmt.Sprintf("(%s)", it.year)))
	}
	b.WriteString("\n")
	typeText := typeLabel(it.mediaType)
	if it.isAnime {
		typeText += " · Anime"
	}
	b.WriteString(mutedStyle.Render(typeText))
	if it.voteAverage > 0 {
		fmt.Fprintf(&b, "%s", mutedStyle.Render(fmt.Sprintf(" · ★ %.1f", it.voteAverage)))
	}
	if it.status != nil {
		fmt.Fprintf(&b, "  %s", statusBadge(*it.status))
	}
	b.WriteString("\n")
	b.WriteString(m.detailDivider())
	b.WriteString("\n")
	if it.overview != "" {
		b.WriteString(it.overview)
	} else {
		b.WriteString("No overview available.")
	}

	box := m.sized(boxStyle).Render(b.String())
	if m.mode == modeConfirm {
		promptText := fmt.Sprintf("Request %q? [y/N]", it.title)
		if m.canPickServer() {
			promptText += fmt.Sprintf("  Server: %s", m.selectedServerName())
		}
		prompt := m.sized(promptStyle).Render(promptText)
		return lipgloss.JoinVertical(lipgloss.Left, box, m.standaloneDivider(), prompt)
	}
	return box
}

// detailDivider is a muted horizontal rule sized to detailView/
// requestDetailView's shared box width, separating their title/metadata
// block from the body text below it. It's meant for use as content inside
// the box, where boxStyle's own padding provides the left/right inset.
func (m model) detailDivider() string {
	dividerWidth := m.termWidth() - boxStyle.GetHorizontalFrameSize()
	if dividerWidth < 1 {
		dividerWidth = 1
	}
	return mutedStyle.Render(strings.Repeat("─", dividerWidth))
}

// standaloneDivider is detailDivider's counterpart for use between the box
// and the confirm prompt — outside the box, so (unlike detailDivider) it
// needs its own left inset to line up with the box/prompt's, rather than
// inheriting one from a wrapping style.
func (m model) standaloneDivider() string {
	return "  " + m.detailDivider()
}

// requestDetailView mirrors detailView's shape (title, status line,
// divider, body text) for a requestItem instead of a browseItem: request
// status and media availability status both get a badge on the title line
// itself, a TV request's per-season availability gets its own line below
// that (see seasonsLine — movie requests never have one), and the body is
// the full request timestamp rather than an overview (requests don't have
// one).
func (m model) requestDetailView() string {
	it := m.selectedRequest
	var b strings.Builder
	b.WriteString(titleStyle.Render(it.Title()))
	b.WriteString(" ")
	b.WriteString(requestStatusBadge(it.requestStatus))
	b.WriteString("  ")
	b.WriteString(statusBadge(it.mediaStatus))
	b.WriteString("\n")
	if seasons := seasonsLine(it.seasons); seasons != "" {
		b.WriteString(seasons)
		b.WriteString("\n")
	}
	b.WriteString(m.detailDivider())
	b.WriteString("\n")
	fmt.Fprintf(&b, "Requested: %s", it.createdAt)

	box := m.sized(boxStyle).Render(b.String())
	if m.mode == modeRequestConfirm {
		// No ID here, matching the CLI's own delete confirmation wording
		// exactly ("Delete this request? [y/N]") — a bare request ID isn't
		// meaningful to look at, especially once titles are showing.
		prompt := m.sized(promptStyle).Render("Delete this request? [y/N]")
		return lipgloss.JoinVertical(lipgloss.Left, box, m.standaloneDivider(), prompt)
	}
	return box
}

// requestResultView mirrors resultView for a delete instead of a create.
func (m model) requestResultView() string {
	if m.err != nil {
		return m.sized(errorStyle).Render("Delete failed: " + m.err.Error())
	}
	if m.deletedRequestID != 0 {
		// The name, not the raw ID — same reasoning as dropping it from
		// the delete confirmation prompt. m.selectedRequest is still the
		// item that was just deleted; nothing clears it before this.
		// name() (plain), not Title() — Title's embedded ANSI color codes
		// would come out as literal "\x1b[...m" text once %q escapes them.
		return m.sized(successStyle).Render(fmt.Sprintf("Deleted %q.", m.selectedRequest.name()))
	}
	return ""
}

func (m model) resultView() string {
	if m.err != nil {
		return m.sized(errorStyle).Render("Request failed: " + m.err.Error())
	}
	if m.lastRequest != nil {
		return m.sized(successStyle).Render(fmt.Sprintf("Requested %q.", m.selected.title))
	}
	return ""
}

// footerView renders the keybinding hints flush left and, when paging
// applies, "page X/Y" flush right on the same row — anchored to the bottom
// of the screen by View(). Hint order is the same across every mode:
// mode-specific actions first, "q: quit" always last (modeSearch is the one
// exception, since "q" is a character there rather than the quit key).
func (m model) footerView() string {
	hints := m.footerHints()

	inner := m.termWidth() - footerStyle.GetHorizontalFrameSize()
	if inner < minContentWidth {
		inner = minContentWidth
	}

	line := hints
	if page := m.pageIndicator(); page != "" {
		gap := inner - lipgloss.Width(hints) - lipgloss.Width(page)
		if gap < 1 {
			gap = 1
		}
		line = hints + strings.Repeat(" ", gap) + page
	}
	return m.sizedLine(footerStyle).Render(line)
}

// footerHints uses a plain double space between entries rather than a " · "
// bullet — freeing enough width for the hints to sit alongside the page
// badge at a normal 80-column width without truncating. The default
// (browsing) case drops "enter: view" for the same budget reason: opening
// the selected item on enter is the single most standard list convention
// here, so it's the one hint that can go unstated to make room for less
// guessable bindings like "s: status" — a page badge as deep as "123/275"
// still comfortably fits alongside what's left.
func (m model) footerHints() string {
	switch m.mode {
	case modeSearch:
		return "enter: search  esc: cancel"
	case modeDetail:
		return "r/enter: request  esc: back  q: quit"
	case modeConfirm:
		if m.canPickServer() {
			return "y: confirm  n/esc: cancel  s: server  q: quit"
		}
		return "y: confirm  n/esc: cancel  q: quit"
	case modeRequestConfirm:
		return "y: confirm  n/esc: cancel  q: quit"
	case modeResult, modeRequestResult:
		return "any key: continue  q: quit"
	case modeRequests:
		return "↑/↓: nav  enter: view  n/p: page  esc: back  q: quit"
	case modeRequestDetail:
		return "d: delete  esc: back  q: quit"
	default:
		hints := "↑/↓: nav  n/p: page"
		if m.source == sourceSearch {
			hints += "  /: search  esc: clear"
		} else {
			hints += "  tab: movie/tv  /: search"
		}
		hints += "  s: status"
		return hints + "  q: quit"
	}
}

// pageIndicator is "X/Y" while browsing (Discover, search results, or
// requests) has a paginated fetch loaded — not in modeDetail/modeConfirm/
// modeResult or their request counterparts, where a single selected item
// isn't paginated even though totalPages still holds the underlying list's
// last known value. Kept terse (no "page" word) to leave the footer's width
// budget for the hints.
//
// Search results append a "shown (filtered)" note whenever this page came
// back thinner than usual: Seerr paginates the raw, unfiltered result set
// (movies/TV/people together), so a person-heavy page can drop most of its
// 20 raw results once person results (which can't be requested) are
// filtered out — this makes clear that's filtering, not a bug, rather than
// just leaving the list looking sparse.
func (m model) pageIndicator() string {
	if m.mode == modeRequests {
		if m.requestsTotalPages <= 0 {
			return ""
		}
		return fmt.Sprintf("%d/%d", m.requestsPage, m.requestsTotalPages)
	}
	if m.mode != modeBrowsing || m.totalPages <= 0 {
		return ""
	}
	page := fmt.Sprintf("%d/%d", m.page, m.totalPages)
	if m.source == sourceSearch && m.filtered > 0 {
		page += fmt.Sprintf(" · %d shown (%d filtered)", len(m.list.Items()), m.filtered)
	}
	return page
}

// mediaStatusLabel mirrors internal/cli's label for models.MediaStatus.
// Kept local rather than shared: the TUI intentionally doesn't depend on
// internal/cli's rendering code.
func mediaStatusLabel(status models.MediaStatus) string {
	switch status {
	case models.MediaStatusUnknown:
		return "Unknown"
	case models.MediaStatusPending:
		return "Pending"
	case models.MediaStatusProcessing:
		return "Processing"
	case models.MediaStatusPartiallyAvailable:
		return "Partially available"
	case models.MediaStatusAvailable:
		return "Available"
	case models.MediaStatusBlocklisted:
		return "Blocklisted"
	case models.MediaStatusDeleted:
		return "Deleted"
	default:
		return "Unknown"
	}
}

// statusColor and statusGlyph give each models.MediaStatus a color and a
// compact glyph, mirroring Seerr's own status semantics. Blocklisted reads
// as declined/failed (Seerr won't fulfil it); Deleted and Unknown don't fit
// any of those buckets, so they render muted with no glyph rather than
// being forced into one.
func statusColor(status models.MediaStatus) lipgloss.Color {
	switch status {
	case models.MediaStatusPending:
		return colorPending
	case models.MediaStatusProcessing:
		return colorProcessing
	case models.MediaStatusPartiallyAvailable, models.MediaStatusAvailable:
		return colorAvailable
	case models.MediaStatusBlocklisted:
		return colorDeclined
	default: // Unknown, Deleted
		return colorMuted
	}
}

func statusGlyph(status models.MediaStatus) string {
	switch status {
	case models.MediaStatusPending:
		return "🎬"
	case models.MediaStatusProcessing:
		return "↓"
	case models.MediaStatusPartiallyAvailable, models.MediaStatusAvailable:
		return "✓"
	case models.MediaStatusBlocklisted:
		return "✗"
	default: // Unknown, Deleted
		return ""
	}
}

// statusBadge renders a status's glyph (if it has one) and label together in
// the status's color, e.g. "✓ Available".
func statusBadge(status models.MediaStatus) string {
	text := mediaStatusLabel(status)
	if glyph := statusGlyph(status); glyph != "" {
		text = glyph + " " + text
	}
	return lipgloss.NewStyle().Foreground(statusColor(status)).Render(text)
}

// requestStatusLabel mirrors internal/cli/status.go's requestStatusLabel.
// Kept local rather than shared, same reasoning as mediaStatusLabel.
func requestStatusLabel(status int) string {
	switch status {
	case 1:
		return "Pending"
	case 2:
		return "Approved"
	case 3:
		return "Declined"
	case 4:
		return "Failed"
	case 5:
		return "Completed"
	default:
		return "Unknown"
	}
}

// requestStatusColor and requestStatusGlyph map models.MediaRequest.Status
// onto the same four buckets/colors/glyphs as statusColor/statusGlyph
// (pending/requested, processing/downloading, available, declined/failed),
// since a request's lifecycle is a different enum but the same underlying
// semantics: Pending is still awaiting action (pending bucket); Approved is
// accepted and moving toward fulfillment (processing bucket); Declined and
// Failed both mean Seerr won't fulfil it (declined bucket); Completed means
// it's done (available bucket).
func requestStatusColor(status int) lipgloss.Color {
	switch status {
	case 1: // Pending
		return colorPending
	case 2: // Approved
		return colorProcessing
	case 3, 4: // Declined, Failed
		return colorDeclined
	case 5: // Completed
		return colorAvailable
	default:
		return colorMuted
	}
}

func requestStatusGlyph(status int) string {
	switch status {
	case 1: // Pending
		return "🎬"
	case 2: // Approved
		return "↓"
	case 3, 4: // Declined, Failed
		return "✗"
	case 5: // Completed
		return "✓"
	default:
		return ""
	}
}

// requestStatusBadge is statusBadge's counterpart for a request's own
// status (as opposed to its media's library-availability status).
func requestStatusBadge(status int) string {
	text := requestStatusLabel(status)
	if glyph := requestStatusGlyph(status); glyph != "" {
		text = glyph + " " + text
	}
	return lipgloss.NewStyle().Foreground(requestStatusColor(status)).Render(text)
}

// seasonsLine summarizes a TV request's per-season availability as a
// single compact row, e.g. "Seasons: S1 ✓  S2 ✓  S3 ↓" — reusing the same
// status colors/glyphs as media availability generally (statusColor/
// statusGlyph, keyed on the same models.MediaStatus enum Seerr uses for a
// season's own status). A status with no glyph of its own (Unknown/
// Deleted) still gets a muted bullet, so every season shows some marker
// rather than an unexplained gap. Movie requests, or a TV request Seerr
// hasn't attached season data to, return "".
func seasonsLine(seasons []models.RequestSeason) string {
	if len(seasons) == 0 {
		return ""
	}
	sorted := append([]models.RequestSeason(nil), seasons...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].SeasonNumber < sorted[j].SeasonNumber })

	parts := make([]string, len(sorted))
	for i, s := range sorted {
		glyph := statusGlyph(s.Status)
		if glyph == "" {
			glyph = "•"
		}
		label := mutedStyle.Render(fmt.Sprintf("S%d", s.SeasonNumber))
		coloredGlyph := lipgloss.NewStyle().Foreground(statusColor(s.Status)).Render(glyph)
		parts[i] = label + " " + coloredGlyph
	}
	return "Seasons: " + strings.Join(parts, "  ")
}
