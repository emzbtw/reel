package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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

func TestView_ResultSuccess(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeResult
	m.selected = testItem()
	m.lastRequest = &models.MediaRequest{ID: 9}

	out := m.View()
	if !strings.Contains(out, "Requested \"Dune\" (request #9)") {
		t.Errorf("View() missing success message:\n%s", out)
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
	// The n/p hint must survive alongside the badge, not get truncated away
	// to make room for it.
	if !strings.Contains(footer, "n/p: page") {
		t.Errorf("footerView() = %q, want it to still include the \"n/p: page\" hint", footer)
	}
}

// TestView_PageIndicator_HiddenOutsideBrowsing checks the badge doesn't
// leak into detail/confirm/result footers just because m.totalPages still
// holds the underlying browse session's value — pagination isn't a real
// action once a single item is selected.
func TestView_PageIndicator_HiddenOutsideBrowsing(t *testing.T) {
	for _, mode := range []mode{modeDetail, modeConfirm, modeResult} {
		m := newModel(context.Background(), nil)
		m.mode = mode
		m.page, m.totalPages = 3, 12
		m.selected = testItem()

		if got := m.pageIndicator(); got != "" {
			t.Errorf("mode %v: pageIndicator() = %q, want empty", mode, got)
		}
		if footer := m.footerView(); strings.Contains(footer, "3/12") {
			t.Errorf("mode %v: footerView() = %q, should not include the page badge", mode, footer)
		}
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
