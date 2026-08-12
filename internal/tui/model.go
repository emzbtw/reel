package tui

import (
	"context"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/emzbtw/reel/internal/api"
	"github.com/emzbtw/reel/internal/models"
)

type mode int

const (
	modeBrowsing mode = iota
	modeSearch
	modeDetail
	modeConfirm
	modeResult
)

// source is what the current list's items came from, and therefore what
// n/p (paging) and tab (media type toggle) should do.
type source int

const (
	sourceDiscover source = iota
	sourceSearch
)

// browseItem flattens a MovieResult or TvResult to what the list and detail
// views need. It satisfies list.DefaultItem (FilterValue/Title/Description).
type browseItem struct {
	id          int
	mediaType   models.MediaType
	title       string
	year        string
	overview    string
	voteAverage float64
	voteCount   int
	popularity  float64
	status      *models.MediaStatus // nil when Seerr has no library record for it
}

func (i browseItem) FilterValue() string { return i.title }
func (i browseItem) Title() string       { return i.title }
func (i browseItem) Description() string {
	desc := typeLabel(i.mediaType)
	if i.year != "" {
		desc += " · " + i.year
	}
	if i.status != nil {
		if glyph := statusGlyph(*i.status); glyph != "" {
			desc += " · " + lipgloss.NewStyle().Foreground(statusColor(*i.status)).Render(glyph)
		}
	}
	return desc
}

func typeLabel(t models.MediaType) string {
	if t == models.MediaTV {
		return "TV"
	}
	return "Movie"
}

func movieItem(m models.MovieResult) browseItem {
	item := browseItem{
		id:          m.ID,
		mediaType:   models.MediaMovie,
		title:       m.Title,
		year:        year(m.ReleaseDate),
		overview:    m.Overview,
		voteAverage: m.VoteAverage,
		voteCount:   m.VoteCount,
		popularity:  m.Popularity,
	}
	if m.MediaInfo != nil {
		s := m.MediaInfo.Status
		item.status = &s
	}
	return item
}

func tvItem(t models.TvResult) browseItem {
	item := browseItem{
		id:          t.ID,
		mediaType:   models.MediaTV,
		title:       t.Name,
		year:        year(t.FirstAirDate),
		overview:    t.Overview,
		voteAverage: t.VoteAverage,
		voteCount:   t.VoteCount,
		popularity:  t.Popularity,
	}
	if t.MediaInfo != nil {
		s := t.MediaInfo.Status
		item.status = &s
	}
	return item
}

func year(date string) string {
	if len(date) < 4 {
		return ""
	}
	return date[:4]
}

// searchItems flattens a search result list down to browseItems, dropping
// person results (they can't be requested). Mirrors internal/cli's
// mediaOnly, kept local rather than shared per the TUI's own view logic.
//
// Seerr's relevance ordering is kept as-is except where promoteDominant
// nudges a landslide-more-popular result above weak matches Seerr happened
// to rank ahead of it — see promoteDominant's doc comment.
func searchItems(results []models.SearchResult) []browseItem {
	items := make([]browseItem, 0, len(results))
	for _, r := range results {
		switch r.MediaType {
		case models.MediaMovie:
			if r.Movie != nil {
				items = append(items, movieItem(*r.Movie))
			}
		case models.MediaTV:
			if r.TV != nil {
				items = append(items, tvItem(*r.TV))
			}
		}
	}
	promoteDominant(items)
	return items
}

// Dominance thresholds for promoteDominant, mirroring
// internal/obsidian/sync.go's dominanceRatio/minVotesToJudge — same
// judgment call (is this a landslide, not a coin-flip), duplicated locally
// since sync.go's constants are unexported and the two use cases are
// otherwise unrelated.
const (
	searchDominanceRatio  = 10
	searchMinVotesToJudge = 20
)

// dominatesForSearch reports whether top is so far ahead of second, by vote
// count or (when second is too new/obscure to have accumulated real votes)
// popularity, that it can be promoted above it without asking. Ported from
// internal/obsidian/sync.go's dominates — same reasoning, applied to a
// browseItem pair instead of a Candidate pair.
func dominatesForSearch(top, second browseItem) bool {
	if second.voteCount >= searchMinVotesToJudge {
		return top.voteCount >= searchDominanceRatio*second.voteCount
	}
	return top.popularity > 0 && top.popularity >= searchDominanceRatio*second.popularity
}

// promoteDominant nudges search results toward "the one people mean" without
// discarding Seerr's relevance ordering wholesale: a short or ambiguous
// query (e.g. "cher") can rank a well-known title below several low-signal
// partial matches, since Seerr/TMDB's multi-search relevance doesn't weigh
// popularity heavily for partial matches.
//
// This is deliberately not a popularity sort — that would risk burying a
// precise, exact-title match under a generically popular but unrelated
// title. Instead it's a single stable insertion-sort-style pass: an item
// only moves up past the item immediately ahead of it, and only while it
// dominates that specific neighbor (10x its vote count, or 10x its
// popularity when neither has enough votes to judge on). Two items that are
// merely somewhat different in popularity never swap, so Seerr's ordering
// wins by default; a landslide-popular result can still climb past several
// weak items, one dominated neighbor at a time.
func promoteDominant(items []browseItem) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && dominatesForSearch(items[j], items[j-1]); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

// model is the root bubbletea model driving reel's TUI.
type model struct {
	ctx    context.Context
	client *api.Client

	mode      mode
	mediaType models.MediaType

	source      source
	query       string // last-submitted search query; only meaningful when source == sourceSearch
	searchInput textinput.Model

	list             list.Model
	page, totalPages int
	// loadSeq tags each fetch so a pageLoadedMsg/errMsg from a fetch that's
	// no longer current (e.g. the user toggled type or paged again before
	// it returned) is discarded instead of clobbering newer state.
	loadSeq int

	selected    browseItem
	lastRequest *models.MediaRequest

	loading bool
	err     error
	spinner spinner.Model

	width, height int
}

func newModel(ctx context.Context, client *api.Client) model {
	delegate := list.NewDefaultDelegate()
	// Titles stay bold; description (metadata) stays muted regardless of
	// selection. The selected row is marked by ">" on its title line; its
	// title text also turns magenta. Title text lands at column 4, under
	// the "l" of "reel" (accounting for listStyle's column-1 left pad in
	// view.go): NormalTitle has no border, so its padding (3) covers both
	// the marker's column and the gap up to column 4; SelectedTitle's
	// padding (2) covers just the gap, since its border already takes the
	// marker's column. SelectedDesc/NormalDesc have no marker to account
	// for, so their padding matches NormalTitle's.
	delegate.Styles.NormalTitle = lipgloss.NewStyle().Bold(true).Padding(0, 0, 0, 3)
	delegate.Styles.NormalDesc = lipgloss.NewStyle().Foreground(colorMuted).Padding(0, 0, 0, 3)
	delegate.Styles.SelectedTitle = lipgloss.NewStyle().
		Border(selectionMarker, false, false, false, true).
		BorderForeground(colorMagenta).
		Foreground(colorMagenta).
		Bold(true).
		Padding(0, 0, 0, 2)
	delegate.Styles.SelectedDesc = lipgloss.NewStyle().
		Foreground(colorMuted).
		Padding(0, 0, 0, 3)
	delegate.SetSpacing(0) // default's blank line between items is more gap than these two-line rows need
	l := list.New(nil, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	l.DisableQuitKeybindings()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	ti := textinput.New()
	ti.Prompt = "" // "Search: " (rendered in searchView) already serves as the prompt
	ti.Placeholder = "Search movies and TV…"
	ti.CharLimit = 200
	ti.Width = searchInputWidth(fallbackWidth)

	return model{
		ctx:         ctx,
		client:      client,
		mode:        modeBrowsing,
		mediaType:   models.MediaMovie,
		searchInput: ti,
		list:        l,
		page:        1,
		spinner:     sp,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, fetchPageCmd(m.ctx, m.client, m.source, m.mediaType, m.query, m.page, m.loadSeq))
}

// Messages

type pageLoadedMsg struct {
	seq        int
	items      []browseItem
	page       int
	totalPages int
}

type errMsg struct {
	seq int
	err error
}

type requestResultMsg struct {
	req *models.MediaRequest
	err error
}

// Commands

// fetchPageCmd fetches one page of results, either a Discover feed (for the
// given mediaType) or a Search (for query) depending on src. query is
// ignored when src is sourceDiscover, and mediaType when src is
// sourceSearch (search results mix both types already).
func fetchPageCmd(ctx context.Context, client *api.Client, src source, mediaType models.MediaType, query string, page, seq int) tea.Cmd {
	return func() tea.Msg {
		if src == sourceSearch {
			results, err := client.Search(ctx, query, page)
			if err != nil {
				return errMsg{seq: seq, err: err}
			}
			return pageLoadedMsg{seq: seq, items: searchItems(results.Results), page: results.Page, totalPages: results.TotalPages}
		}

		if mediaType == models.MediaTV {
			results, err := client.DiscoverTV(ctx, page)
			if err != nil {
				return errMsg{seq: seq, err: err}
			}
			items := make([]browseItem, len(results.Results))
			for i, t := range results.Results {
				items[i] = tvItem(t)
			}
			return pageLoadedMsg{seq: seq, items: items, page: results.Page, totalPages: results.TotalPages}
		}

		results, err := client.DiscoverMovies(ctx, page)
		if err != nil {
			return errMsg{seq: seq, err: err}
		}
		items := make([]browseItem, len(results.Results))
		for i, mv := range results.Results {
			items[i] = movieItem(mv)
		}
		return pageLoadedMsg{seq: seq, items: items, page: results.Page, totalPages: results.TotalPages}
	}
}

func submitRequestCmd(ctx context.Context, client *api.Client, item browseItem) tea.Cmd {
	return func() tea.Msg {
		input := api.CreateRequestInput{MediaType: item.mediaType, MediaID: item.id}
		if item.mediaType == models.MediaTV {
			input.AllSeasons = true
		}
		req, err := client.CreateRequest(ctx, input)
		return requestResultMsg{req: req, err: err}
	}
}
