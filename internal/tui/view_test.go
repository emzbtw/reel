package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/emzbtw/reel/internal/api"
	"github.com/emzbtw/reel/internal/models"
)

// These are smoke tests for View(): the sandbox this runs in has no TTY to
// drive a real bubbletea program interactively, so they check each mode
// renders the substrings a person would rely on, without panicking.

func TestView_Browsing(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.width, m.height = 80, 24
	m.list.SetSize(80, 20)
	updated, _ := m.Update(pageLoadedMsg{page: 2, totalPages: 10, items: []browseItem{testItem()}})
	m = updated.(model)

	out := m.View()
	for _, want := range []string{"Discover · Movies", "2/10", "Dune", "tab: movie/tv"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() missing %q:\n%s", want, out)
		}
	}
}

func TestView_Search(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeSearch
	m.searchInput.Focus()
	m.searchInput.SetValue("dune")

	out := m.View()
	for _, want := range []string{"Search:", "dune", "enter: search"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() missing %q:\n%s", want, out)
		}
	}
}

// TestHeaderView_ReflectsMode is the regression check for the header being
// stuck on the Discover movie/tv toggle text regardless of Search mode: it
// used to only key off m.source (set on submit), so it kept showing
// "Discover · Movies" the whole time a query was being typed in modeSearch.
func TestHeaderView_ReflectsMode(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mediaType = models.MediaTV

	if got := m.headerView(); !strings.Contains(got, "Discover · TV") {
		t.Errorf("headerView() = %q, want it to contain %q", got, "Discover · TV")
	}

	// While typing, the header shouldn't repeat the live query — the body's
	// searchView() already shows "Search: <input>" directly below it.
	m.mode = modeSearch
	m.searchInput.SetValue("dune")
	if got := m.headerView(); !strings.Contains(got, "Search") || strings.Contains(got, "dune") {
		t.Errorf("headerView() while typing = %q, want it to contain %q but not the query", got, "Search")
	}

	m.mode = modeBrowsing
	m.source, m.query = sourceSearch, "dune"
	if got := m.headerView(); !strings.Contains(got, "Search: dune") {
		t.Errorf("headerView() browsing search results = %q, want it to contain %q", got, "Search: dune")
	}

	for _, mode := range []mode{modeRequests, modeRequestDetail, modeRequestConfirm, modeRequestResult} {
		m.mode = mode
		if got := m.headerView(); !strings.Contains(got, "Requests") {
			t.Errorf("mode %v: headerView() = %q, want it to contain %q", mode, got, "Requests")
		}
	}
}

func TestView_BrowsingSearchResults(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.source, m.query = sourceSearch, "dune"
	m.list.SetSize(80, 20)
	updated, _ := m.Update(pageLoadedMsg{page: 1, totalPages: 1, items: []browseItem{testItem()}})
	m = updated.(model)

	out := m.View()
	for _, want := range []string{"Search: dune", "Dune", "esc: clear"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() missing %q:\n%s", want, out)
		}
	}
}

func TestView_Loading(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.loading = true

	out := m.View()
	if !strings.Contains(out, "Loading") {
		t.Errorf("View() missing \"Loading\":\n%s", out)
	}
}

func TestView_BrowsingError(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.err = errors.New("seerr: unavailable")

	out := m.View()
	if !strings.Contains(out, "seerr: unavailable") {
		t.Errorf("View() missing error text:\n%s", out)
	}
}

func TestView_Detail(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeDetail
	m.selected = browseItem{
		id: 1, mediaType: models.MediaTV, title: "Chernobyl", year: "2019",
		overview: "A dramatization.", voteAverage: 8.5,
		status: statusPtr(models.MediaStatusAvailable),
	}

	out := m.View()
	for _, want := range []string{"Chernobyl", "2019", "TV", "8.5", "Available", "A dramatization.", "r/enter: request"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() missing %q:\n%s", want, out)
		}
	}
}

func TestView_Confirm(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeConfirm
	m.selected = testItem()

	out := m.View()
	for _, want := range []string{"Request", "Dune", "[y/N]", "y: confirm"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() missing %q:\n%s", want, out)
		}
	}
}

// TestView_Confirm_TVShowsServerPick checks the confirm screen surfaces the
// current Sonarr server choice — and the "s: server" hint — only when
// there's a real choice to make (TV, with more than one server known).
func TestView_Confirm_TVShowsServerPick(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeConfirm
	m.selected = testItem()
	m.selected.mediaType = models.MediaTV
	m.sonarrServers = testServers()
	m.serverIdx = 1 // "Sonarr Anime"

	out := m.View()
	for _, want := range []string{"Server: Sonarr Anime", "s: server"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Sonarr (default)") {
		t.Errorf("View() shows the default server while Sonarr Anime is picked:\n%s", out)
	}
}

// TestView_Confirm_TVDefaultServerLabeled checks the server Seerr itself
// defaults to is labeled as such — entering modeConfirm resets serverIdx to
// it (see updateDetail), so this is what a fresh TV request confirm screen
// shows before the user ever presses "s".
func TestView_Confirm_TVDefaultServerLabeled(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeConfirm
	m.selected = testItem()
	m.selected.mediaType = models.MediaTV
	m.sonarrServers = testServers()
	m.serverIdx = 0 // "Sonarr", the one marked IsDefault in testServers()

	out := m.View()
	if !strings.Contains(out, "Server: Sonarr (default)") {
		t.Errorf("View() missing %q:\n%s", "Server: Sonarr (default)", out)
	}
}

// TestView_Confirm_NoServerPickWithoutChoice mirrors
// TestUpdate_Confirm_SNoopsWithoutChoice: neither the server line nor its
// footer hint should appear when there's nothing to cycle through.
func TestView_Confirm_NoServerPickWithoutChoice(t *testing.T) {
	tests := []struct {
		name    string
		tv      bool
		servers []api.SonarrServer
	}{
		{"movie with servers", false, testServers()},
		{"TV with one server", true, testServers()[:1]},
		{"TV with no servers loaded", true, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newModel(context.Background(), nil)
			m.mode = modeConfirm
			m.selected = testItem()
			if tt.tv {
				m.selected.mediaType = models.MediaTV
			}
			m.sonarrServers = tt.servers

			out := m.View()
			if strings.Contains(out, "Server:") || strings.Contains(out, "s: server") {
				t.Errorf("View() unexpectedly shows a server pick:\n%s", out)
			}
		})
	}
}

func TestView_ResultSuccess(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeResult
	m.selected = testItem()
	m.lastRequest = &models.MediaRequest{ID: 9}

	out := m.View()
	if !strings.Contains(out, "Requested \"Dune\".") {
		t.Errorf("View() missing success message:\n%s", out)
	}
	if strings.Contains(out, "#9") {
		t.Errorf("View() = %q, want no raw request ID shown", out)
	}
}

func TestView_ResultError(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeResult
	m.err = errors.New("seerr: forbidden")

	out := m.View()
	if !strings.Contains(out, "Request failed: seerr: forbidden") {
		t.Errorf("View() missing failure message:\n%s", out)
	}
}

func TestView_Requests(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeRequests
	m.list.SetSize(80, 20)
	updated, _ := m.Update(requestsPageLoadedMsg{page: 2, totalPages: 10, items: []requestItem{testRequestItem()}})
	m = updated.(model)

	out := m.View()
	for _, want := range []string{"Requests", "2/10", "TMDB 42", "esc: back"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() missing %q:\n%s", want, out)
		}
	}
}

// TestView_Requests_ShowsResolvedTitle checks a request row renders its
// resolved title instead of the "TMDB <id>" fallback once one's been
// looked up.
func TestView_Requests_ShowsResolvedTitle(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeRequests
	m.list.SetSize(80, 20)
	item := testRequestItem()
	item.title = "Chernobyl"
	updated, _ := m.Update(requestsPageLoadedMsg{page: 1, totalPages: 1, items: []requestItem{item}})
	m = updated.(model)

	out := m.View()
	if !strings.Contains(out, "Chernobyl") {
		t.Errorf("View() missing resolved title %q:\n%s", "Chernobyl", out)
	}
	if strings.Contains(out, "TMDB 42") {
		t.Errorf("View() = %q, want the fallback \"TMDB 42\" not shown once a title is resolved", out)
	}
}

func TestView_RequestDetail(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeRequestDetail
	m.selectedRequest = requestItem{
		id: 9, requestStatus: 2, tmdbID: 42,
		mediaStatus: models.MediaStatusProcessing, createdAt: "2024-01-15T10:30:00.000Z",
	}

	out := m.View()
	for _, want := range []string{"TMDB 42", "Approved", "Processing", "2024-01-15", "d: delete"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Seasons:") {
		t.Errorf("View() = %q, want no seasons line (this item has none)", out)
	}
}

// TestView_RequestDetail_ShowsSeasonsForTV checks a TV request's per-season
// line renders, and that a movie request (no Seasons) never shows one.
func TestView_RequestDetail_ShowsSeasonsForTV(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeRequestDetail
	m.selectedRequest = requestItem{
		id: 9, requestStatus: 5, mediaType: models.MediaTV, tmdbID: 87108, title: "Chernobyl",
		mediaStatus: models.MediaStatusAvailable, createdAt: "2024-01-15T10:30:00.000Z",
		seasons: []models.RequestSeason{
			{SeasonNumber: 1, Status: models.MediaStatusAvailable},
			{SeasonNumber: 2, Status: models.MediaStatusProcessing},
		},
	}

	out := m.View()
	for _, want := range []string{"Seasons:", "S1", "S2"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() missing %q:\n%s", want, out)
		}
	}
}

func TestView_RequestConfirm(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeRequestConfirm
	m.selectedRequest = testRequestItem()

	out := m.View()
	for _, want := range []string{"Delete this request?", "[y/N]", "y: confirm"} {
		if !strings.Contains(out, want) {
			t.Errorf("View() missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "#9") {
		t.Errorf("View() contains a raw request ID, want the ID dropped:\n%s", out)
	}
}

func TestView_RequestResultSuccess(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeRequestResult
	m.deletedRequestID = 9
	m.selectedRequest = testRequestItem() // still populated: nothing clears it before rendering
	m.selectedRequest.title = "Dune"

	out := m.View()
	// Plain name only, no "(year · Type)" tag — that tag is Title's own
	// styling (embedded ANSI), which %q would otherwise escape into
	// literal "\x1b[...m" garbage instead of rendering as color.
	if !strings.Contains(out, "Deleted \"Dune\".") {
		t.Errorf("View() missing success message:\n%s", out)
	}
	if strings.Contains(out, "\\x1b") {
		t.Errorf("View() = %q, want no escaped ANSI codes leaking through as literal text", out)
	}
	if strings.Contains(out, "#9") {
		t.Errorf("View() = %q, want no raw request ID shown", out)
	}
}

func TestView_RequestResultError(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeRequestResult
	m.err = errors.New("seerr: forbidden")

	out := m.View()
	if !strings.Contains(out, "Delete failed: seerr: forbidden") {
		t.Errorf("View() missing failure message:\n%s", out)
	}
}

// TestView_FooterAnchoredToBottomInEveryMode checks the footer sits on the
// last line of the screen no matter how little (search, result) or how much
// (a long overview) the body renders — it must never trail directly under
// short content instead of staying pinned to the bottom.
func TestView_FooterAnchoredToBottomInEveryMode(t *testing.T) {
	for _, tt := range []struct {
		name string
		m    func() model
	}{
		{"browsing_shortList", func() model {
			m := newModel(context.Background(), nil)
			updated, _ := m.Update(pageLoadedMsg{page: 1, totalPages: 1, items: []browseItem{testItem()}})
			return updated.(model)
		}},
		{"search", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeSearch
			return m
		}},
		{"detail_shortOverview", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeDetail
			m.selected = testItem()
			return m
		}},
		{"detail_realisticOverview", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeDetail
			m.selected = browseItem{
				title: "Dune", overview: "A noble family becomes embroiled in a war for control " +
					"over the galaxy's most valuable asset while its heir becomes troubled by visions of a dark future.",
			}
			return m
		}},
		{"confirm", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeConfirm
			m.selected = testItem()
			return m
		}},
		{"result", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeResult
			m.lastRequest = &models.MediaRequest{ID: 1}
			m.selected = testItem()
			return m
		}},
		{"requests_shortList", func() model {
			m := newModel(context.Background(), nil)
			updated, _ := m.Update(requestsPageLoadedMsg{page: 1, totalPages: 1, items: []requestItem{testRequestItem()}})
			got := updated.(model)
			got.mode = modeRequests
			return got
		}},
		{"requestDetail", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeRequestDetail
			m.selectedRequest = testRequestItem()
			return m
		}},
		{"requestConfirm", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeRequestConfirm
			m.selectedRequest = testRequestItem()
			return m
		}},
		{"requestResult", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeRequestResult
			m.deletedRequestID = 9
			return m
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.m()
			updated, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
			got := updated.(model)

			lines := strings.Split(got.View(), "\n")
			if len(lines) != 20 {
				t.Fatalf("View() has %d lines, want exactly 20 (window height)", len(lines))
			}

			// JoinVertical pads shorter blocks (header/footer) out to the
			// widest block's width (the fully-Placed body), so compare with
			// trailing spaces trimmed rather than exact equality.
			wantFooter := strings.TrimRight(got.footerView(), " ")
			gotFooter := strings.TrimRight(lines[len(lines)-1], " ")
			if gotFooter != wantFooter {
				t.Errorf("last line = %q, want footerView() = %q", gotFooter, wantFooter)
			}
		})
	}
}

// TestFooterHints_QuitAlwaysLast is the specific regression reported: "tab:
// movie/tv" used to render after "q: quit", breaking the pattern every
// other mode already followed. Order must now be consistent, with "q: quit"
// last wherever it appears (modeSearch is the one mode that doesn't offer
// it, since "q" is a character there).
func TestFooterHints_QuitAlwaysLast(t *testing.T) {
	for _, tt := range []struct {
		name string
		m    model
	}{
		{"browsing_discover", func() model {
			m := newModel(context.Background(), nil)
			m.source = sourceDiscover
			return m
		}()},
		{"browsing_search", func() model {
			m := newModel(context.Background(), nil)
			m.source, m.query = sourceSearch, "dune"
			return m
		}()},
		{"detail", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeDetail
			return m
		}()},
		{"confirm", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeConfirm
			return m
		}()},
		{"result", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeResult
			return m
		}()},
		{"requests", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeRequests
			return m
		}()},
		{"requestDetail", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeRequestDetail
			return m
		}()},
		{"requestConfirm", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeRequestConfirm
			return m
		}()},
		{"requestResult", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeRequestResult
			return m
		}()},
	} {
		t.Run(tt.name, func(t *testing.T) {
			hints := tt.m.footerHints()
			if !strings.HasSuffix(hints, "q: quit") {
				t.Errorf("footerHints() = %q, want it to end with %q", hints, "q: quit")
			}
		})
	}

	// The regression itself: tab must now precede quit, not follow it.
	m := newModel(context.Background(), nil)
	m.source = sourceDiscover
	hints := m.footerHints()
	if strings.Index(hints, "tab: movie/tv") > strings.Index(hints, "q: quit") {
		t.Errorf("footerHints() = %q, want \"tab: movie/tv\" before \"q: quit\"", hints)
	}
}

// TestView_PageIndicator_BottomRight checks the page indicator moved out of
// the header and onto the right edge of the footer row.
func TestView_PageIndicator_BottomRight(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.width, m.height = 80, 20
	m.list.SetSize(80, 16)
	updated, _ := m.Update(pageLoadedMsg{page: 3, totalPages: 12, items: []browseItem{testItem()}})
	m = updated.(model)

	if strings.Contains(m.headerView(), "page") {
		t.Errorf("headerView() = %q, should no longer include the page indicator", m.headerView())
	}

	footer := m.footerView()
	if !strings.HasSuffix(strings.TrimRight(footer, " "), "3/12") {
		t.Errorf("footerView() = %q, want it to end with the right-aligned page indicator", footer)
	}
	// The n/p and s hints must survive alongside the badge, not get
	// truncated away to make room for it.
	if !strings.Contains(footer, "n/p: page") {
		t.Errorf("footerView() = %q, want it to still include the \"n/p: page\" hint", footer)
	}
	if !strings.Contains(footer, "s: status") {
		t.Errorf("footerView() = %q, want it to still include the \"s: status\" hint", footer)
	}
}

// TestView_PageIndicator_SurvivesDeepPaginationAt80Columns is the specific
// regression this guards against: adding "s: status" to the browsing
// footer once pushed its hints wide enough that, at a plain 80-column
// terminal, the page badge silently vanished off the end (footerView's
// truncation drops it rather than garbling it — see footerHints' doc
// comment on the budget this leaves). 275 pages was a real total from a
// broad search query earlier in this project's use, so "123/275" is a
// realistic worst case, not a contrived one.
func TestView_PageIndicator_SurvivesDeepPaginationAt80Columns(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.width, m.height = 80, 20
	m.list.SetSize(80, 16)
	updated, _ := m.Update(pageLoadedMsg{page: 123, totalPages: 275, items: []browseItem{testItem()}})
	m = updated.(model)

	footer := m.footerView()
	if !strings.Contains(footer, "123/275") {
		t.Errorf("footerView() = %q, want it to still include the \"123/275\" page badge", footer)
	}
	if !strings.Contains(footer, "s: status") {
		t.Errorf("footerView() = %q, want it to still include the \"s: status\" hint", footer)
	}
}

// TestView_PageIndicator_FilteredNoteOnThinSearchPage checks that a search
// page which came back thin because person results were filtered out says
// so, rather than just rendering a sparse list with no explanation.
func TestView_PageIndicator_FilteredNoteOnThinSearchPage(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.source = sourceSearch
	updated, _ := m.Update(pageLoadedMsg{page: 9, totalPages: 275, filtered: 15, items: []browseItem{testItem()}})
	m = updated.(model)

	got := m.pageIndicator()
	want := "9/275 · 1 shown (15 filtered)"
	if got != want {
		t.Errorf("pageIndicator() = %q, want %q", got, want)
	}
}

// TestView_PageIndicator_NoFilteredNoteWhenNothingFiltered checks the note
// only appears when a page actually lost results to filtering — not on
// every search page, and never on Discover (which never filters).
func TestView_PageIndicator_NoFilteredNoteWhenNothingFiltered(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.source = sourceSearch
	updated, _ := m.Update(pageLoadedMsg{page: 1, totalPages: 275, filtered: 0, items: []browseItem{testItem()}})
	m = updated.(model)

	if got := m.pageIndicator(); got != "1/275" {
		t.Errorf("pageIndicator() = %q, want %q (no filtered note when filtered=0)", got, "1/275")
	}
}

// TestView_PageIndicator_HiddenOutsideBrowsing checks the badge doesn't
// leak into detail/confirm/result footers just because m.totalPages still
// holds the underlying browse session's value — pagination isn't a real
// action once a single item is selected.
func TestView_PageIndicator_HiddenOutsideBrowsing(t *testing.T) {
	for _, mode := range []mode{
		modeDetail, modeConfirm, modeResult,
		modeRequestDetail, modeRequestConfirm, modeRequestResult,
	} {
		m := newModel(context.Background(), nil)
		m.mode = mode
		m.page, m.totalPages = 3, 12
		m.requestsPage, m.requestsTotalPages = 3, 12
		m.selected = testItem()
		m.selectedRequest = testRequestItem()

		if got := m.pageIndicator(); got != "" {
			t.Errorf("mode %v: pageIndicator() = %q, want empty", mode, got)
		}
		if footer := m.footerView(); strings.Contains(footer, "3/12") {
			t.Errorf("mode %v: footerView() = %q, should not include the page badge", mode, footer)
		}
	}
}

// TestView_PageIndicator_RequestsUsesRequestsPageState checks pageIndicator
// reads requestsPage/requestsTotalPages while in modeRequests, not the
// stale page/totalPages left over from whatever Discover/Search browsing
// was active before "s" was pressed.
func TestView_PageIndicator_RequestsUsesRequestsPageState(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeRequests
	m.page, m.totalPages = 99, 99
	m.requestsPage, m.requestsTotalPages = 3, 40

	if got := m.pageIndicator(); got != "3/40" {
		t.Errorf("pageIndicator() = %q, want %q", got, "3/40")
	}
}

// TestView_SearchAlignsWithHeader checks "Search:" starts in the same
// column as "reel —" on the header line, with a blank gap row between them.
func TestView_SearchAlignsWithHeader(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeSearch

	lines := strings.Split(m.View(), "\n")
	if len(lines) < 3 {
		t.Fatalf("View() has %d lines, want at least 3 (header + gap + search row)", len(lines))
	}
	if strings.TrimSpace(lines[1]) != "" {
		t.Errorf("line 1 = %q, want a blank gap row between header and search row", lines[1])
	}
	headerIndent := leadingSpaces(lines[0])
	searchIndent := leadingSpaces(lines[2])
	if headerIndent != searchIndent {
		t.Errorf("header starts at column %d, search row starts at column %d, want equal\nheader: %q\nsearch: %q",
			headerIndent, searchIndent, lines[0], lines[2])
	}
}

func leadingSpaces(s string) int {
	return len(s) - len(strings.TrimLeft(s, " "))
}

func statusPtr(s models.MediaStatus) *models.MediaStatus { return &s }

func TestSeasonsLine_Empty(t *testing.T) {
	if got := seasonsLine(nil); got != "" {
		t.Errorf("seasonsLine(nil) = %q, want empty", got)
	}
}

// TestSeasonsLine_SortsAndOrdersBySeasonNumber checks output is sorted by
// season number regardless of input order (Seerr doesn't guarantee one).
func TestSeasonsLine_SortsAndOrdersBySeasonNumber(t *testing.T) {
	got := seasonsLine([]models.RequestSeason{
		{SeasonNumber: 3, Status: models.MediaStatusAvailable},
		{SeasonNumber: 1, Status: models.MediaStatusAvailable},
		{SeasonNumber: 2, Status: models.MediaStatusProcessing},
	})
	s1, s2, s3 := strings.Index(got, "S1"), strings.Index(got, "S2"), strings.Index(got, "S3")
	if s1 < 0 || s2 < 0 || s3 < 0 {
		t.Fatalf("seasonsLine() = %q, want S1/S2/S3 all present", got)
	}
	if !(s1 < s2 && s2 < s3) {
		t.Errorf("seasonsLine() = %q, want S1 before S2 before S3", got)
	}
}

// TestSeasonsLine_UnknownStatusGetsFallbackGlyph checks a season status
// with no glyph of its own (statusGlyph's default case) still gets a
// marker, rather than leaving a bare "S<n>" with nothing after it.
func TestSeasonsLine_UnknownStatusGetsFallbackGlyph(t *testing.T) {
	got := seasonsLine([]models.RequestSeason{{SeasonNumber: 1, Status: models.MediaStatusUnknown}})
	if !strings.Contains(got, "S1") || !strings.Contains(got, "•") {
		t.Errorf("seasonsLine() = %q, want \"S1\" and a fallback \"•\" glyph", got)
	}
}
