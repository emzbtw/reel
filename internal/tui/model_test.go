package tui

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/emzbtw/reel/internal/api"
	"github.com/emzbtw/reel/internal/config"
	"github.com/emzbtw/reel/internal/models"
)

func testClient(t *testing.T, handler http.HandlerFunc) *api.Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return api.NewClient(&config.Config{SeerrURL: srv.URL, SeerrAPIKey: "test-key"})
}

func keyMsg(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func testItem() browseItem {
	return browseItem{id: 42, mediaType: models.MediaMovie, title: "Dune", year: "2021"}
}

// fetchPageCmd/submitRequestCmd

func TestFetchPageCmd_Movies(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/discover/movies" {
			t.Errorf("path = %q, want /api/v1/discover/movies", r.URL.Path)
		}
		if got := r.URL.Query().Get("page"); got != "2" {
			t.Errorf("page param = %q, want %q", got, "2")
		}
		w.Write([]byte(`{
			"page": 2, "totalPages": 50, "totalResults": 1000,
			"results": [{"id": 1, "mediaType": "movie", "title": "Dune", "releaseDate": "2021-10-22"}]
		}`))
	})

	msg := fetchPageCmd(context.Background(), client, sourceDiscover, models.MediaMovie, "", 2, 7)()
	loaded, ok := msg.(pageLoadedMsg)
	if !ok {
		t.Fatalf("msg = %#v, want pageLoadedMsg", msg)
	}
	if loaded.seq != 7 || loaded.page != 2 || loaded.totalPages != 50 {
		t.Errorf("loaded = %+v, want seq=7 page=2 totalPages=50", loaded)
	}
	if len(loaded.items) != 1 || loaded.items[0].title != "Dune" || loaded.items[0].mediaType != models.MediaMovie {
		t.Errorf("loaded.items = %+v, want a single movie Dune", loaded.items)
	}
}

func TestFetchPageCmd_TV(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/discover/tv" {
			t.Errorf("path = %q, want /api/v1/discover/tv", r.URL.Path)
		}
		w.Write([]byte(`{
			"page": 1, "totalPages": 10, "totalResults": 200,
			"results": [{"id": 5, "mediaType": "tv", "name": "Chernobyl", "firstAirDate": "2019-05-06"}]
		}`))
	})

	msg := fetchPageCmd(context.Background(), client, sourceDiscover, models.MediaTV, "", 1, 0)()
	loaded, ok := msg.(pageLoadedMsg)
	if !ok {
		t.Fatalf("msg = %#v, want pageLoadedMsg", msg)
	}
	if len(loaded.items) != 1 || loaded.items[0].title != "Chernobyl" || loaded.items[0].mediaType != models.MediaTV {
		t.Errorf("loaded.items = %+v, want a single tv Chernobyl", loaded.items)
	}
}

func TestFetchPageCmd_Error(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"message":"nope"}`))
	})

	msg := fetchPageCmd(context.Background(), client, sourceDiscover, models.MediaMovie, "", 1, 3)()
	errored, ok := msg.(errMsg)
	if !ok {
		t.Fatalf("msg = %#v, want errMsg", msg)
	}
	if errored.seq != 3 {
		t.Errorf("errored.seq = %d, want 3", errored.seq)
	}
	if !errors.Is(errored.err, api.ErrUnauthorized) {
		t.Errorf("errored.err = %v, want it to wrap api.ErrUnauthorized", errored.err)
	}
}

func TestFetchPageCmd_Search(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Errorf("path = %q, want /api/v1/search", r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); got != "dune" {
			t.Errorf("query param = %q, want %q", got, "dune")
		}
		if got := r.URL.Query().Get("page"); got != "2" {
			t.Errorf("page param = %q, want %q", got, "2")
		}
		w.Write([]byte(`{
			"page": 2, "totalPages": 3, "totalResults": 50,
			"results": [
				{"id": 1, "mediaType": "movie", "title": "Dune", "releaseDate": "2021-10-22"},
				{"id": 2, "mediaType": "tv", "name": "Dune: Prophecy", "firstAirDate": "2024-11-17"},
				{"id": 3, "mediaType": "person", "profilePath": "/x.jpg"}
			]
		}`))
	})

	msg := fetchPageCmd(context.Background(), client, sourceSearch, models.MediaMovie, "dune", 2, 5)()
	loaded, ok := msg.(pageLoadedMsg)
	if !ok {
		t.Fatalf("msg = %#v, want pageLoadedMsg", msg)
	}
	if loaded.seq != 5 || loaded.page != 2 || loaded.totalPages != 3 {
		t.Errorf("loaded = %+v, want seq=5 page=2 totalPages=3", loaded)
	}
	// Person result must be dropped; only the movie and TV results remain.
	if len(loaded.items) != 2 {
		t.Fatalf("len(loaded.items) = %d, want 2 (person result dropped)", len(loaded.items))
	}
	if loaded.items[0].title != "Dune" || loaded.items[0].mediaType != models.MediaMovie {
		t.Errorf("loaded.items[0] = %+v, want movie Dune", loaded.items[0])
	}
	if loaded.items[1].title != "Dune: Prophecy" || loaded.items[1].mediaType != models.MediaTV {
		t.Errorf("loaded.items[1] = %+v, want tv Dune: Prophecy", loaded.items[1])
	}
}

func TestSubmitRequestCmd(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/request" {
			t.Errorf("path = %q, want /api/v1/request", r.URL.Path)
		}
		w.Write([]byte(`{"id": 9, "status": 1, "media": {"id": 1, "tmdbId": 42}}`))
	})

	msg := submitRequestCmd(context.Background(), client, testItem())()
	result, ok := msg.(requestResultMsg)
	if !ok {
		t.Fatalf("msg = %#v, want requestResultMsg", msg)
	}
	if result.err != nil {
		t.Fatalf("result.err = %v, want nil", result.err)
	}
	if result.req == nil || result.req.ID != 9 {
		t.Errorf("result.req = %+v, want ID 9", result.req)
	}
}

// Update: async message handling

func TestUpdate_PageLoaded_SetsListAndClearsLoading(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.loading = true

	updated, _ := m.Update(pageLoadedMsg{
		seq: 0, page: 4, totalPages: 20,
		items: []browseItem{testItem()},
	})
	got := updated.(model)

	if got.loading {
		t.Error("loading = true, want false after pageLoadedMsg")
	}
	if got.page != 4 || got.totalPages != 20 {
		t.Errorf("page/totalPages = %d/%d, want 4/20", got.page, got.totalPages)
	}
	if len(got.list.Items()) != 1 {
		t.Errorf("len(list.Items()) = %d, want 1", len(got.list.Items()))
	}
}

func TestUpdate_PageLoaded_StaleSeqDiscarded(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.loading = true
	m.loadSeq = 2

	updated, _ := m.Update(pageLoadedMsg{seq: 1, page: 99, totalPages: 99})
	got := updated.(model)

	if !got.loading {
		t.Error("loading = false, want true: a stale pageLoadedMsg should be ignored")
	}
	if got.page == 99 {
		t.Error("page was updated from a stale pageLoadedMsg")
	}
}

func TestUpdate_ErrMsg_StaleSeqDiscarded(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.loading = true
	m.loadSeq = 2

	wantErr := errors.New("boom")
	updated, _ := m.Update(errMsg{seq: 1, err: wantErr})
	got := updated.(model)

	if !got.loading || got.err != nil {
		t.Errorf("loading/err = %v/%v, want true/nil: a stale errMsg should be ignored", got.loading, got.err)
	}
}

func TestUpdate_ErrMsg_CurrentSeqSets(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.loading = true

	wantErr := errors.New("boom")
	updated, _ := m.Update(errMsg{seq: 0, err: wantErr})
	got := updated.(model)

	if got.loading {
		t.Error("loading = true, want false after errMsg")
	}
	if got.err != wantErr {
		t.Errorf("err = %v, want %v", got.err, wantErr)
	}
}

// Update: window resize

func TestUpdate_WindowSizeMsg_ResizesListAndReflowsEveryScreen(t *testing.T) {
	longOverview := strings.Repeat("word ", 60)

	for _, tt := range []struct {
		name string
		m    func() model
		// expectExactHeight is false for cases whose content is
		// deliberately taller than the 20-row viewport (they exist to
		// stress-test wrapping/no-bleed, not anchoring): the footer can't
		// stay pinned below content that doesn't fit, the same way a list
		// longer than the terminal can't either. Anchoring under realistic
		// content sizes is covered separately by
		// TestView_FooterAnchoredToBottomInEveryMode.
		expectExactHeight bool
	}{
		{"browsing", func() model {
			m := newModel(context.Background(), nil)
			updated, _ := m.Update(pageLoadedMsg{items: []browseItem{testItem()}})
			return updated.(model)
		}, true},
		{"search", func() model {
			// searchInput is a single-line scrolling field, so even a very
			// long value never grows past one row.
			m := newModel(context.Background(), nil)
			m.mode = modeSearch
			m.searchInput.SetValue(longOverview)
			return m
		}, true},
		{"detail", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeDetail
			m.selected = browseItem{title: "Dune", overview: longOverview}
			return m
		}, false},
		{"confirm", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeConfirm
			m.selected = browseItem{title: "Dune", overview: longOverview}
			return m
		}, false},
		{"result", func() model {
			m := newModel(context.Background(), nil)
			m.mode = modeResult
			m.err = errors.New(strings.Repeat("failure ", 60))
			return m
		}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.m()

			updated, _ := m.Update(tea.WindowSizeMsg{Width: 30, Height: 20})
			narrow := updated.(model)

			if narrow.width != 30 || narrow.height != 20 {
				t.Fatalf("width/height = %d/%d, want 30/20", narrow.width, narrow.height)
			}
			wantListWidth := 30 - listStyle.GetHorizontalFrameSize()
			if got := narrow.list.Width(); got != wantListWidth {
				t.Errorf("list width = %d, want %d (list should track window size, minus listStyle's border+padding)", got, wantListWidth)
			}
			if got := lipgloss.Width(narrow.View()); got > 30 {
				t.Errorf("View() at width 30 renders %d cells wide, want <= 30:\n%s", got, narrow.View())
			}
			if tt.expectExactHeight {
				if got := lipgloss.Height(narrow.View()); got != 20 {
					t.Errorf("View() at height 20 renders %d lines, want exactly 20 (footer anchored)", got)
				}
			}

			// Widening re-applies the new size rather than caching the
			// first wrap.
			updatedWide, _ := narrow.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
			wide := updatedWide.(model)
			if got := lipgloss.Width(wide.View()); got > 100 {
				t.Errorf("View() at width 100 renders %d cells wide, want <= 100", got)
			}
			if tt.expectExactHeight {
				if got := lipgloss.Height(wide.View()); got != 20 {
					t.Errorf("View() at height 20 renders %d lines, want exactly 20 (footer anchored)", got)
				}
			}
		})
	}
}

// Update: browsing mode

func TestUpdate_Browsing_EnterOpensDetail(t *testing.T) {
	m := newModel(context.Background(), nil)
	updated, _ := m.Update(pageLoadedMsg{items: []browseItem{testItem()}})
	m = updated.(model)

	updated, _ = m.Update(keyMsg("enter"))
	got := updated.(model)

	if got.mode != modeDetail {
		t.Errorf("mode = %v, want modeDetail", got.mode)
	}
	if got.selected.title != "Dune" {
		t.Errorf("selected = %+v, want Dune", got.selected)
	}
}

func TestUpdate_Browsing_TabTogglesTypeAndFetches(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/discover/tv" {
			t.Errorf("path = %q, want /api/v1/discover/tv (tab should switch to TV)", r.URL.Path)
		}
		w.Write([]byte(`{"page": 1, "totalPages": 1, "totalResults": 0, "results": []}`))
	})
	m := newModel(context.Background(), client)
	m.page = 3

	updated, cmd := m.Update(keyMsg("tab"))
	got := updated.(model)

	if got.mediaType != models.MediaTV {
		t.Errorf("mediaType = %v, want MediaTV", got.mediaType)
	}
	if got.page != 1 {
		t.Errorf("page = %d, want 1 (reset on type toggle)", got.page)
	}
	if !got.loading {
		t.Error("loading = false, want true")
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want a fetch command")
	}
	if _, ok := cmd().(pageLoadedMsg); !ok {
		t.Error("cmd() did not hit /discover/tv as expected")
	}
}

func TestUpdate_Browsing_PageNavigation(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.page, m.totalPages = 2, 5

	// "p" (prev) and "n" (next) both fetch when in range.
	if _, cmd := m.Update(keyMsg("n")); cmd == nil {
		t.Error("\"n\" with page < totalPages: cmd = nil, want a fetch command")
	}
	if _, cmd := m.Update(keyMsg("p")); cmd == nil {
		t.Error("\"p\" with page > 1: cmd = nil, want a fetch command")
	}

	// Bounds: can't go below page 1 or past totalPages.
	atStart := m
	atStart.page = 1
	if _, cmd := atStart.Update(keyMsg("p")); cmd != nil {
		t.Error("\"p\" at page 1: cmd != nil, want no-op")
	}
	atEnd := m
	atEnd.page = 5
	if _, cmd := atEnd.Update(keyMsg("n")); cmd != nil {
		t.Error("\"n\" at last page: cmd != nil, want no-op")
	}

	// Already loading: further page requests are ignored.
	loading := m
	loading.loading = true
	if _, cmd := loading.Update(keyMsg("n")); cmd != nil {
		t.Error("\"n\" while loading: cmd != nil, want no-op")
	}
}

// Update: search mode

func TestUpdate_Browsing_SlashEntersSearchAndClearsStalePriorInput(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.searchInput.SetValue("leftover from a previous search")

	updated, _ := m.Update(keyMsg("/"))
	got := updated.(model)

	if got.mode != modeSearch {
		t.Errorf("mode = %v, want modeSearch", got.mode)
	}
	if got.searchInput.Value() != "" {
		t.Errorf("searchInput.Value() = %q, want empty: stale text from a previous search must not carry over", got.searchInput.Value())
	}
	if !got.searchInput.Focused() {
		t.Error("searchInput.Focused() = false, want true")
	}
}

func TestUpdate_Search_TypingEntersText(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeSearch
	m.searchInput.Focus()

	updated, _ := m.Update(keyMsg("d"))
	updated, _ = updated.(model).Update(keyMsg("u"))
	got := updated.(model)

	if got.searchInput.Value() != "du" {
		t.Errorf("searchInput.Value() = %q, want %q", got.searchInput.Value(), "du")
	}
}

func TestUpdate_Search_EnterWithEmptyQueryIsNoOp(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeSearch

	updated, cmd := m.Update(keyMsg("enter"))
	got := updated.(model)

	if got.mode != modeSearch {
		t.Errorf("mode = %v, want modeSearch (empty query shouldn't submit)", got.mode)
	}
	if cmd != nil {
		t.Error("cmd != nil, want no fetch for an empty query")
	}
}

func TestUpdate_Search_EnterSubmitsAndFetches(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/search" {
			t.Errorf("path = %q, want /api/v1/search", r.URL.Path)
		}
		if got := r.URL.Query().Get("query"); got != "dune" {
			t.Errorf("query param = %q, want %q", got, "dune")
		}
		w.Write([]byte(`{"page": 1, "totalPages": 1, "totalResults": 1,
			"results": [{"id": 1, "mediaType": "movie", "title": "Dune", "releaseDate": "2021-10-22"}]}`))
	})
	m := newModel(context.Background(), client)
	m.mode = modeSearch
	m.searchInput.SetValue("  dune  ")

	updated, cmd := m.Update(keyMsg("enter"))
	got := updated.(model)

	if got.mode != modeBrowsing {
		t.Errorf("mode = %v, want modeBrowsing", got.mode)
	}
	if got.source != sourceSearch {
		t.Errorf("source = %v, want sourceSearch", got.source)
	}
	if got.query != "dune" {
		t.Errorf("query = %q, want %q (trimmed)", got.query, "dune")
	}
	if !got.loading {
		t.Error("loading = false, want true")
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want a fetch command")
	}
	if _, ok := cmd().(pageLoadedMsg); !ok {
		t.Error("cmd() did not hit /search as expected")
	}
}

func TestUpdate_Search_EscCancelsWithoutChangingWhatWasShown(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.source, m.query = sourceDiscover, ""
	m.mode = modeSearch
	m.searchInput.SetValue("dune")

	updated, cmd := m.Update(keyMsg("esc"))
	got := updated.(model)

	if got.mode != modeBrowsing {
		t.Errorf("mode = %v, want modeBrowsing", got.mode)
	}
	if got.source != sourceDiscover || got.query != "" {
		t.Errorf("source/query = %v/%q, want unchanged (sourceDiscover/\"\")", got.source, got.query)
	}
	if cmd != nil {
		t.Error("cmd != nil, want no fetch: cancelling a search shouldn't refetch anything")
	}
}

func TestUpdate_Browsing_TabIsNoOpWhileSourceSearch(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.source, m.query = sourceSearch, "dune"
	m.mediaType = models.MediaMovie

	updated, cmd := m.Update(keyMsg("tab"))
	got := updated.(model)

	if got.mediaType != models.MediaMovie {
		t.Errorf("mediaType = %v, want unchanged MediaMovie", got.mediaType)
	}
	if cmd != nil {
		t.Error("cmd != nil, want no-op: nothing to toggle in mixed search results")
	}
}

func TestUpdate_Browsing_EscClearsSearchBackToDiscover(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/discover/movies" {
			t.Errorf("path = %q, want /api/v1/discover/movies", r.URL.Path)
		}
		w.Write([]byte(`{"page": 1, "totalPages": 1, "totalResults": 0, "results": []}`))
	})
	m := newModel(context.Background(), client)
	m.source, m.query, m.page = sourceSearch, "dune", 2

	updated, cmd := m.Update(keyMsg("esc"))
	got := updated.(model)

	if got.source != sourceDiscover {
		t.Errorf("source = %v, want sourceDiscover", got.source)
	}
	if got.query != "" {
		t.Errorf("query = %q, want cleared", got.query)
	}
	if got.page != 1 {
		t.Errorf("page = %d, want reset to 1", got.page)
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want a fetch command")
	}
	if _, ok := cmd().(pageLoadedMsg); !ok {
		t.Error("cmd() did not hit /discover/movies as expected")
	}
}

// TestUpdate_Search_SecondSearchDoesNotBleedIntoFirst is the specific
// regression this was checked for before implementing: search, submit,
// then search again for something else — the second query must fully
// replace the first rather than merging with or racing against it.
func TestUpdate_Search_SecondSearchDoesNotBleedIntoFirst(t *testing.T) {
	var gotQueries []string
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQueries = append(gotQueries, r.URL.Query().Get("query"))
		w.Write([]byte(`{"page": 1, "totalPages": 1, "totalResults": 0, "results": []}`))
	})
	m := newModel(context.Background(), client)

	// First search: "dune".
	updated, _ := m.Update(keyMsg("/"))
	m = updated.(model)
	for _, r := range "dune" {
		updated, _ = m.Update(keyMsg(string(r)))
		m = updated.(model)
	}
	updated, cmd1 := m.Update(keyMsg("enter"))
	m = updated.(model)
	if _, ok := cmd1().(pageLoadedMsg); !ok {
		t.Fatal("first search's cmd did not resolve to pageLoadedMsg")
	}
	if m.query != "dune" {
		t.Fatalf("query after first search = %q, want %q", m.query, "dune")
	}

	// Second search, without ever clearing via esc: "heat".
	updated, _ = m.Update(keyMsg("/"))
	m = updated.(model)
	if m.searchInput.Value() != "" {
		t.Fatalf("searchInput.Value() = %q, want empty when starting a new search", m.searchInput.Value())
	}
	for _, r := range "heat" {
		updated, _ = m.Update(keyMsg(string(r)))
		m = updated.(model)
	}
	updated, cmd2 := m.Update(keyMsg("enter"))
	m = updated.(model)
	if _, ok := cmd2().(pageLoadedMsg); !ok {
		t.Fatal("second search's cmd did not resolve to pageLoadedMsg")
	}

	if m.query != "heat" {
		t.Errorf("query after second search = %q, want %q", m.query, "heat")
	}
	if len(gotQueries) != 2 || gotQueries[0] != "dune" || gotQueries[1] != "heat" {
		t.Errorf("queries sent to Seerr = %v, want [dune heat]", gotQueries)
	}
}

// TestUpdate_Search_StaleFirstSearchResponseDiscardedAfterSecondSearch
// covers the race variant: the first search's HTTP response is still
// in-flight (simulated by holding its pageLoadedMsg rather than feeding it
// immediately) when the second search is submitted. Feeding the stale
// response into Update afterward must not clobber the second search's
// state.
func TestUpdate_Search_StaleFirstSearchResponseDiscardedAfterSecondSearch(t *testing.T) {
	m := newModel(context.Background(), nil)

	updated, _ := m.Update(keyMsg("/"))
	m = updated.(model)
	m.searchInput.SetValue("dune")
	updated, cmd := m.Update(keyMsg("enter"))
	m = updated.(model)
	staleSeq := m.loadSeq
	_ = cmd // first search's request is never "delivered" — simulating it arriving late, below

	updated, _ = m.Update(keyMsg("/"))
	m = updated.(model)
	m.searchInput.SetValue("heat")
	updated, _ = m.Update(keyMsg("enter"))
	m = updated.(model)

	if m.query != "heat" {
		t.Fatalf("query = %q, want %q before the stale response arrives", m.query, "heat")
	}

	// The first search's (now stale) response finally arrives.
	updated, _ = m.Update(pageLoadedMsg{
		seq: staleSeq, page: 1, totalPages: 1,
		items: []browseItem{{title: "Dune", mediaType: models.MediaMovie}},
	})
	got := updated.(model)

	if got.query != "heat" {
		t.Errorf("query = %q after a stale response arrived, want it to remain %q", got.query, "heat")
	}
	if len(got.list.Items()) == 1 {
		if it, ok := got.list.Items()[0].(browseItem); ok && it.title == "Dune" {
			t.Error("list was overwritten by the stale first search's results")
		}
	}
}

func TestUpdate_Search_QDoesNotQuitButCtrlCDoes(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeSearch
	m.searchInput.Focus()

	updated, cmd := m.Update(keyMsg("q"))
	got := updated.(model)
	if cmd != nil {
		if _, isQuit := cmd().(tea.QuitMsg); isQuit {
			t.Fatal("\"q\" quit the program while typing a search query")
		}
	}
	if got.searchInput.Value() != "q" {
		t.Errorf("searchInput.Value() = %q, want %q: \"q\" should be typed, not treated as quit", got.searchInput.Value(), "q")
	}

	_, cmd = m.Update(keyMsg("ctrl+c"))
	if cmd == nil {
		t.Fatal("ctrl+c: cmd = nil, want tea.Quit even while typing")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c did not quit while in modeSearch")
	}
}

// Update: detail/confirm/result modes

func TestUpdate_Detail_Transitions(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeDetail
	m.selected = testItem()

	updated, _ := m.Update(keyMsg("r"))
	if got := updated.(model); got.mode != modeConfirm {
		t.Errorf("after \"r\": mode = %v, want modeConfirm", got.mode)
	}

	updated, _ = m.Update(keyMsg("esc"))
	if got := updated.(model); got.mode != modeBrowsing {
		t.Errorf("after \"esc\": mode = %v, want modeBrowsing", got.mode)
	}
}

func TestUpdate_Confirm_YSubmits(t *testing.T) {
	client := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"id": 11, "status": 1, "media": {"id": 1, "tmdbId": 42}}`))
	})
	m := newModel(context.Background(), client)
	m.mode = modeConfirm
	m.selected = testItem()

	updated, cmd := m.Update(keyMsg("y"))
	got := updated.(model)

	if !got.loading {
		t.Error("loading = false, want true after confirming")
	}
	if cmd == nil {
		t.Fatal("cmd = nil, want the request-submitting command")
	}
	result, ok := cmd().(requestResultMsg)
	if !ok || result.req == nil || result.req.ID != 11 {
		t.Errorf("cmd() = %#v, want requestResultMsg with request #11", cmd())
	}

	// Feeding the result back in should land on modeResult.
	updated, _ = got.Update(result)
	final := updated.(model)
	if final.mode != modeResult {
		t.Errorf("mode = %v, want modeResult", final.mode)
	}
	if final.lastRequest == nil || final.lastRequest.ID != 11 {
		t.Errorf("lastRequest = %+v, want request #11", final.lastRequest)
	}
}

// TestUpdate_Confirm_DefaultIsNo checks the [y/N] convention shared with the
// CLI's request/delete commands: only "y" submits, everything else
// (including a bare enter) is a no-op or cancels back to detail.
func TestUpdate_Confirm_DefaultIsNo(t *testing.T) {
	for _, k := range []string{"n", "esc"} {
		m := newModel(context.Background(), nil)
		m.mode = modeConfirm
		m.selected = testItem()

		updated, cmd := m.Update(keyMsg(k))
		got := updated.(model)
		if got.mode != modeDetail {
			t.Errorf("key %q: mode = %v, want modeDetail", k, got.mode)
		}
		if cmd != nil {
			t.Errorf("key %q: cmd != nil, want no request submitted", k)
		}
	}

	// A key that's neither y/n/esc doesn't submit either.
	m := newModel(context.Background(), nil)
	m.mode = modeConfirm
	m.selected = testItem()
	updated, cmd := m.Update(keyMsg("x"))
	got := updated.(model)
	if got.mode != modeConfirm {
		t.Errorf("key \"x\": mode = %v, want modeConfirm (unchanged)", got.mode)
	}
	if cmd != nil {
		t.Error("key \"x\": cmd != nil, want no request submitted")
	}
}

func TestUpdate_Result_AnyKeyReturnsToBrowsing(t *testing.T) {
	m := newModel(context.Background(), nil)
	m.mode = modeResult
	m.err = errors.New("boom")
	m.lastRequest = &models.MediaRequest{ID: 1}

	updated, _ := m.Update(keyMsg("x"))
	got := updated.(model)

	if got.mode != modeBrowsing {
		t.Errorf("mode = %v, want modeBrowsing", got.mode)
	}
	if got.err != nil || got.lastRequest != nil {
		t.Errorf("err/lastRequest = %v/%v, want both cleared", got.err, got.lastRequest)
	}
}

// Update: global quit

func TestUpdate_QuitFromEveryMode(t *testing.T) {
	for _, mode := range []mode{modeBrowsing, modeDetail, modeConfirm, modeResult} {
		for _, k := range []string{"q", "ctrl+c"} {
			m := newModel(context.Background(), nil)
			m.mode = mode

			_, cmd := m.Update(keyMsg(k))
			if cmd == nil {
				t.Fatalf("mode %v key %q: cmd = nil, want tea.Quit", mode, k)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Errorf("mode %v key %q: cmd() = %#v, want tea.QuitMsg", mode, k, cmd())
			}
		}
	}
}
