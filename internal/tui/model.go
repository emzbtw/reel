package tui

import (
	"context"
	"fmt"

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
	// modeRequests/modeRequestDetail/modeRequestConfirm/modeRequestResult
	// mirror modeBrowsing/modeDetail/modeConfirm/modeResult's shape for
	// existing requests (list -> detail -> delete confirm -> result), kept
	// as their own mode values rather than folded into the browse ones
	// since the underlying item (requestItem, not browseItem) and the
	// available action (delete, not request) both genuinely differ.
	modeRequests
	modeRequestDetail
	modeRequestConfirm
	modeRequestResult
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
	// isAnime is TV-only — see isAnime's doc comment for how it's derived.
	isAnime bool
}

func (i browseItem) FilterValue() string { return i.title }
func (i browseItem) Title() string       { return i.title }
func (i browseItem) Description() string {
	desc := typeLabel(i.mediaType)
	if i.isAnime {
		desc += " · Anime"
	}
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
		isAnime:     isAnime(t.GenreIDs, t.OriginalLanguage),
	}
	if t.MediaInfo != nil {
		s := t.MediaInfo.Status
		item.status = &s
	}
	return item
}

// animeGenreID is TMDB's "Animation" genre — id 16 in both its movie and TV
// genre lists.
const animeGenreID = 16

// isAnime approximates Seerr's own "Series Type: Anime" flag. That flag
// comes from a TMDB keyword (id 210024, "anime") only present on a show's
// full detail response — Discover/Search list results don't carry it, and
// fetching it per row isn't viable for a paginated list. Genre-includes-
// Animation plus a Japanese original language is the standard proxy other
// TMDB-consuming apps use instead: no extra request, and — since it's
// computed from the same list-result fields everywhere reel uses it —
// consistent between the list rows and the detail view, even though it can
// occasionally disagree with Seerr's own flag on an edge-case title.
func isAnime(genreIDs []int, originalLanguage string) bool {
	if originalLanguage != "ja" {
		return false
	}
	for _, id := range genreIDs {
		if id == animeGenreID {
			return true
		}
	}
	return false
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

// requestItem flattens a models.MediaRequest to what the requests list and
// detail views need. It satisfies list.DefaultItem (FilterValue/Title/
// Description).
type requestItem struct {
	id            int
	requestStatus int // models.MediaRequest.Status: 1 Pending, 2 Approved, 3 Declined, 4 Failed, 5 Completed
	mediaType     models.MediaType
	tmdbID        int
	mediaStatus   models.MediaStatus // media.status: library availability, not the request's own status
	seasons       []models.RequestSeason
	createdAt     string
	// title/year are resolved separately from GET /request (which doesn't
	// join either in) via a per-item MediaSummary lookup — see
	// resolveTitles. Both empty until resolved, or if the lookup failed
	// for this item.
	title string
	year  string
}

func (i requestItem) FilterValue() string { return fmt.Sprintf("%d", i.tmdbID) }

// name resolves the item's plain display name — the title if resolved, or
// the TMDB-ID fallback otherwise — with no styling applied. Use this (not
// Title) anywhere the result needs to stay plain text, e.g. embedded in a
// fmt "%q"-quoted message: %q escapes control characters, so a string
// containing Title's embedded ANSI color codes comes out as literal
// "\x1b[...m" garbage instead of being rendered as color.
func (i requestItem) name() string {
	if i.title != "" {
		return i.title
	}
	return fmt.Sprintf("TMDB %d", i.tmdbID)
}

// Title is name suffixed with a muted "(<year> · Movie/TV)" tag (year
// omitted if unresolved) — the tag is colored last in the string
// deliberately: a nested Render's own trailing reset only clears the outer
// style's color for whatever comes after it, so this is safe exactly
// because nothing does.
func (i requestItem) Title() string {
	meta := typeLabel(i.mediaType)
	if i.year != "" {
		meta = i.year + " · " + meta
	}
	return i.name() + " " + mutedStyle.Render("("+meta+")")
}

// Description puts the request-status badge first and the muted "requested
// <date>" text after it, both self-colored via their own Render calls
// rather than leaning on the delegate's NormalDesc/SelectedDesc wrap for the
// date's color: a nested Render's own trailing reset would clear that outer
// style's color for everything rendered after it. (browseItem.Description's
// status glyph avoids this the other way, by being the last thing in the
// string instead.)
func (i requestItem) Description() string {
	return requestStatusBadge(i.requestStatus) + "  " + mutedStyle.Render("requested "+dateOnly(i.createdAt))
}

func requestItemOf(r models.MediaRequest) requestItem {
	return requestItem{
		id:            r.ID,
		requestStatus: r.Status,
		mediaType:     r.Type,
		tmdbID:        r.Media.TmdbID,
		mediaStatus:   r.Media.Status,
		seasons:       r.Seasons,
		createdAt:     r.CreatedAt,
	}
}

// dateOnly trims an ISO8601 timestamp down to its date, mirroring year's
// truncation of a release date — a request list row has no room for a full
// timestamp, and to-the-day precision is all that's useful there.
func dateOnly(s string) string {
	if len(s) < 10 {
		return s
	}
	return s[:10]
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
	// filtered is the current page's dropped-result count from the most
	// recent pageLoadedMsg — see pageLoadedMsg.filtered.
	filtered int
	// loadSeq tags each fetch so a pageLoadedMsg/errMsg from a fetch that's
	// no longer current (e.g. the user toggled type or paged again before
	// it returned) is discarded instead of clobbering newer state.
	loadSeq int

	selected    browseItem
	lastRequest *models.MediaRequest

	// sonarrServers is fetched once at startup (see Init) so a TV request's
	// confirm screen can offer routing to a specific Sonarr instance
	// (Seerr Settings → Services) instead of always taking Seerr's default.
	// A failed fetch leaves this empty rather than setting m.err — some
	// Seerr setups have no Sonarr configured at all, and that must never
	// block browsing or movie requests.
	sonarrServers []api.SonarrServer
	// serverIdx is modeConfirm's current pick, indexing sonarrServers
	// directly. Reset to defaultServerIdx() every time a fresh item enters
	// modeConfirm — there's no separate "no override" choice, since that's
	// behaviorally identical to explicitly picking whichever server Seerr
	// itself already defaults to.
	serverIdx int

	// requestsPage/requestsTotalPages are modeRequests' own paging state,
	// kept separate from page/totalPages (Discover/Search's) so leaving
	// requests and coming back to browsing doesn't lose your place there —
	// same reasoning as source/query already being independent of it.
	requestsPage, requestsTotalPages int
	selectedRequest                  requestItem
	// deletedRequestID is the ID shown by modeRequestResult on a successful
	// delete; 0 (Seerr IDs start at 1) means nothing to show. Mirrors
	// lastRequest's role in the create-request result, but as a bare ID
	// since DeleteRequest's response carries nothing else to show.
	deletedRequestID int
	// titles resolves requestItem titles across the life of the program —
	// see api.TitleCache's doc comment. Held as a pointer so the same cache
	// survives Bubble Tea's per-Update value copies of model rather than
	// being recreated (and emptied) on every message.
	titles *api.TitleCache

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
	delegate.SetSpacing(1) // blank line between items: without it the two-line rows run together
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
		titles:      api.NewTitleCache(),
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		fetchPageCmd(m.ctx, m.client, m.source, m.mediaType, m.query, m.page, m.loadSeq),
		fetchSonarrServersCmd(m.ctx, m.client),
	)
}

// Messages

type pageLoadedMsg struct {
	seq        int
	items      []browseItem
	page       int
	totalPages int
	// filtered is how many raw results this page's fetch dropped (search
	// only — person results, which can't be requested). Seerr paginates
	// the raw, unfiltered set, so a page can come back much thinner than
	// its neighbors purely because it happened to be person-heavy; this
	// lets the UI say so instead of just looking sparse.
	filtered int
}

type errMsg struct {
	seq int
	err error
}

type requestResultMsg struct {
	req *models.MediaRequest
	err error
}

type requestsPageLoadedMsg struct {
	seq        int
	items      []requestItem
	page       int
	totalPages int
}

type deleteResultMsg struct {
	id  int
	err error
}

type sonarrServersLoadedMsg struct {
	servers []api.SonarrServer
	err     error
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
			items := searchItems(results.Results)
			filtered := len(results.Results) - len(items)
			return pageLoadedMsg{seq: seq, items: items, page: results.Page, totalPages: results.TotalPages, filtered: filtered}
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

func submitRequestCmd(ctx context.Context, client *api.Client, item browseItem, serverID *int) tea.Cmd {
	return func() tea.Msg {
		input := api.CreateRequestInput{MediaType: item.mediaType, MediaID: item.id, ServerID: serverID}
		if item.mediaType == models.MediaTV {
			input.AllSeasons = true
		}
		req, err := client.CreateRequest(ctx, input)
		return requestResultMsg{req: req, err: err}
	}
}

// fetchSonarrServersCmd is a best-effort background fetch, batched
// alongside the initial page load in Init — see sonarrServers' doc comment.
func fetchSonarrServersCmd(ctx context.Context, client *api.Client) tea.Cmd {
	return func() tea.Msg {
		servers, err := client.ListSonarrServers(ctx)
		return sonarrServersLoadedMsg{servers: servers, err: err}
	}
}

// serverIdxInRange clamps serverIdx to a valid index of sonarrServers,
// falling back to 0 — sonarrServers can shrink to empty (a failed refetch
// never happens today, but this keeps the accessors below panic-free either
// way) or serverIdx can be left over from a longer server list.
func (m model) serverIdxInRange() int {
	if m.serverIdx < 0 || m.serverIdx >= len(m.sonarrServers) {
		return 0
	}
	return m.serverIdx
}

// selectedServerID resolves serverIdx into the *int CreateRequestInput
// expects: nil for a movie (sonarrServers indexes Sonarr instances, not
// Radarr — sending one here would silently reroute a movie to whichever
// Radarr instance happens to share that numeric ID) or when no servers are
// known at all (a failed fetch, or a Seerr setup with no Sonarr
// configured). Otherwise the picked server's ID, explicitly, even when
// it's the one Seerr already defaults to, since that's simplest and
// behaviorally identical to omitting it.
func (m model) selectedServerID() *int {
	if m.selected.mediaType != models.MediaTV || len(m.sonarrServers) == 0 {
		return nil
	}
	id := m.sonarrServers[m.serverIdxInRange()].ID
	return &id
}

// selectedServerName is selectedServerID's display counterpart, for the
// confirm prompt. The server Seerr itself defaults to is labeled as such,
// so cycling between two servers reads as "Sonarr (default)" / "Sonarr
// Anime" rather than a bare, unexplained name.
func (m model) selectedServerName() string {
	if len(m.sonarrServers) == 0 {
		return "default"
	}
	s := m.sonarrServers[m.serverIdxInRange()]
	if s.IsDefault {
		return s.Name + " (default)"
	}
	return s.Name
}

// defaultServerIdx is where serverIdx resets to on entering modeConfirm:
// whichever server Seerr itself defaults to, or 0 if none is marked (or
// sonarrServers is empty).
func (m model) defaultServerIdx() int {
	for i, s := range m.sonarrServers {
		if s.IsDefault {
			return i
		}
	}
	return 0
}

// requestsPageSize is passed as ListRequestsOptions.Take. internal/cli's
// status command leaves Take unset and takes whatever Seerr defaults to;
// the TUI needs a known, consistent page size for n/p paging to mean
// anything, so it asks explicitly — matching Discover/Search's typical
// page density.
const requestsPageSize = 20

// fetchRequestsCmd fetches one page of existing requests, via the same
// ListRequests call internal/cli's status command uses.
func fetchRequestsCmd(ctx context.Context, client *api.Client, cache *api.TitleCache, page, seq int) tea.Cmd {
	return func() tea.Msg {
		if page < 1 {
			page = 1
		}
		opts := api.ListRequestsOptions{Take: requestsPageSize, Skip: (page - 1) * requestsPageSize}
		reqList, err := client.ListRequests(ctx, opts)
		if err != nil {
			return errMsg{seq: seq, err: err}
		}
		items := make([]requestItem, len(reqList.Results))
		for i, r := range reqList.Results {
			items[i] = requestItemOf(r)
		}
		resolveTitles(ctx, client, cache, items)
		totalPages := reqList.PageInfo.Pages
		if totalPages < 1 {
			totalPages = 1
		}
		return requestsPageLoadedMsg{seq: seq, items: items, page: page, totalPages: totalPages}
	}
}

// resolveTitles fills in items' title/year in place, via the same cached,
// concurrent TMDB-ID lookup internal/cli's request tables use — see
// api.ResolveTitles's doc comment. A failed lookup (e.g. a since-deleted
// TMDB entry) leaves that item's title/year empty rather than failing the
// whole page; requestItem.Title falls back to the TMDB ID (and omits the
// year) for it.
func resolveTitles(ctx context.Context, client *api.Client, cache *api.TitleCache, items []requestItem) {
	queries := make([]api.TitleQuery, len(items))
	for i, it := range items {
		queries[i] = api.TitleQuery{MediaType: it.mediaType, TmdbID: it.tmdbID}
	}
	summaries := api.ResolveTitles(ctx, client, cache, queries)
	for i, s := range summaries {
		items[i].title, items[i].year = s.Title, s.Year
	}
}

// deleteRequestCmd deletes a request, via the same DeleteRequest call
// internal/cli's delete command uses.
func deleteRequestCmd(ctx context.Context, client *api.Client, id int) tea.Cmd {
	return func() tea.Msg {
		err := client.DeleteRequest(ctx, id)
		return deleteResultMsg{id: id, err: err}
	}
}
