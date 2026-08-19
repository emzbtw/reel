package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/emzbtw/reel/internal/models"
)

// chromeLines is how many terminal rows the header, the blank gap row below
// it, and the footer each take, so the list gets sized to exactly what's
// left.
const chromeLines = 3

// searchPromptWidth is len("Search: "), the literal prefix searchView()
// renders ahead of the input itself.
const searchPromptWidth = 8

// searchInputWidth sizes the text input so "Search: " plus its content
// never bleeds past the terminal width, mirroring how chromeLines sizes the
// list. The -3 leaves room for textinput's own cursor-overlay allowance
// (its internal scroll window can run one cell past Width).
func searchInputWidth(termWidth int) int {
	w := termWidth - searchPromptWidth - 3
	if w < 10 {
		w = 10
	}
	return w
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		h := msg.Height - chromeLines
		if h < 1 {
			h = 1
		}
		listWidth := msg.Width - listStyle.GetHorizontalFrameSize()
		if listWidth < 1 {
			listWidth = 1
		}
		listHeight := h - listStyle.GetVerticalFrameSize()
		if listHeight < 1 {
			listHeight = 1
		}
		m.list.SetSize(listWidth, listHeight)
		m.searchInput.Width = searchInputWidth(msg.Width)
		// Width alone doesn't reflow textinput's internal scroll window —
		// that only happens inside SetValue/SetCursor — so nudge it via a
		// cursor no-op to make the new Width actually take effect.
		m.searchInput.SetCursor(m.searchInput.Position())
		return m, nil

	case tea.KeyMsg:
		// ctrl+c always quits, including while typing a search query. Plain
		// "q" quits from every other mode, but not modeSearch — "q" is a
		// perfectly normal character to search for (e.g. "Queen's Gambit").
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		case "q":
			if m.mode != modeSearch {
				return m, tea.Quit
			}
		}

	case pageLoadedMsg:
		if msg.seq != m.loadSeq {
			return m, nil
		}
		m.loading = false
		m.err = nil
		m.page, m.totalPages = msg.page, msg.totalPages
		m.filtered = msg.filtered
		items := make([]list.Item, len(msg.items))
		for i, it := range msg.items {
			items[i] = it
		}
		m.list.SetItems(items)
		m.list.Select(0)
		return m, nil

	case errMsg:
		if msg.seq != m.loadSeq {
			return m, nil
		}
		m.loading = false
		m.err = msg.err
		return m, nil

	case requestResultMsg:
		m.loading = false
		m.lastRequest = msg.req
		m.err = msg.err
		m.mode = modeResult
		return m, nil

	case requestsPageLoadedMsg:
		if msg.seq != m.loadSeq {
			return m, nil
		}
		m.loading = false
		m.err = nil
		m.requestsPage, m.requestsTotalPages = msg.page, msg.totalPages
		items := make([]list.Item, len(msg.items))
		for i, it := range msg.items {
			items[i] = it
		}
		m.list.SetItems(items)
		m.list.Select(0)
		return m, nil

	case sonarrServersLoadedMsg:
		if msg.err == nil {
			m.sonarrServers = msg.servers
		}
		return m, nil

	case deleteResultMsg:
		m.loading = false
		m.err = msg.err
		if msg.err == nil {
			m.deletedRequestID = msg.id
		}
		m.mode = modeRequestResult
		return m, nil

	case spinner.TickMsg:
		if !m.loading {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}

	switch m.mode {
	case modeBrowsing:
		return m.updateBrowsing(msg)
	case modeSearch:
		return m.updateSearch(msg)
	case modeDetail:
		return m.updateDetail(msg)
	case modeConfirm:
		return m.updateConfirm(msg)
	case modeResult:
		return m.updateResult(msg)
	case modeRequests:
		return m.updateRequests(msg)
	case modeRequestDetail:
		return m.updateRequestDetail(msg)
	case modeRequestConfirm:
		return m.updateRequestConfirm(msg)
	case modeRequestResult:
		return m.updateRequestResult(msg)
	default:
		return m, nil
	}
}

// updateBrowsing intercepts tab (media type toggle), n/p (Seerr-side
// pagination) and enter (open detail); everything else — including
// left/right/h/l/pgup/pgdown, which list.Model already binds to scrolling
// through the current page's items — is forwarded to the list untouched.
func (m model) updateBrowsing(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	switch keyMsg.String() {
	case "tab":
		// Search results already mix movies and TV; there's nothing to
		// toggle while browsing them.
		if m.source == sourceSearch {
			return m, nil
		}
		if m.mediaType == models.MediaMovie {
			m.mediaType = models.MediaTV
		} else {
			m.mediaType = models.MediaMovie
		}
		m.page = 1
		return m.startFetch()

	case "n":
		if m.loading || (m.totalPages > 0 && m.page >= m.totalPages) {
			return m, nil
		}
		m.page++
		return m.startFetch()

	case "p":
		if m.loading || m.page <= 1 {
			return m, nil
		}
		m.page--
		return m.startFetch()

	case "/":
		m.searchInput.SetValue("")
		cmd := m.searchInput.Focus()
		m.mode = modeSearch
		return m, cmd

	case "s":
		m.mode = modeRequests
		m.requestsPage = 1
		return m.startRequestsFetch()

	case "esc":
		// Clear an active search back to plain Discover browsing. No-op
		// otherwise — nothing else in modeBrowsing binds esc.
		if m.source != sourceSearch {
			return m, nil
		}
		m.source = sourceDiscover
		m.query = ""
		m.page = 1
		return m.startFetch()

	case "enter":
		if item, ok := m.list.SelectedItem().(browseItem); ok {
			m.selected = item
			m.mode = modeDetail
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// updateSearch handles text entry for the search query. enter submits (if
// non-empty) and starts a fetch; esc cancels back to browsing, leaving
// whatever was already shown untouched.
func (m model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.searchInput, cmd = m.searchInput.Update(msg)
		return m, cmd
	}

	switch keyMsg.Type {
	case tea.KeyEnter:
		query := strings.TrimSpace(m.searchInput.Value())
		if query == "" {
			return m, nil
		}
		m.searchInput.Blur()
		m.mode = modeBrowsing
		m.source = sourceSearch
		m.query = query
		m.page = 1
		return m.startFetch()

	case tea.KeyEsc:
		m.searchInput.Blur()
		m.mode = modeBrowsing
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

// startFetch bumps loadSeq and kicks off a fetch for the model's current
// source/mediaType/query/page, discarding any in-flight fetch's eventual
// response.
func (m model) startFetch() (tea.Model, tea.Cmd) {
	m.loadSeq++
	m.loading = true
	m.err = nil
	return m, fetchPageCmd(m.ctx, m.client, m.source, m.mediaType, m.query, m.page, m.loadSeq)
}

// startRequestsFetch is startFetch's counterpart for modeRequests: bumps
// loadSeq and kicks off a fetch for the model's current requestsPage.
func (m model) startRequestsFetch() (tea.Model, tea.Cmd) {
	m.loadSeq++
	m.loading = true
	m.err = nil
	return m, fetchRequestsCmd(m.ctx, m.client, m.titles, m.requestsPage, m.loadSeq)
}

func (m model) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "r", "enter":
		m.mode = modeConfirm
		// A stale server pick from a previous TV request must never
		// silently carry into this one.
		m.serverIdx = m.defaultServerIdx()
	case "esc", "b":
		m.mode = modeBrowsing
	}
	return m, nil
}

// canPickServer reports whether the confirm screen has a real choice of
// Sonarr instance to offer: only for a TV item, and only once more than one
// server is known (a single-server or failed-fetch case has nothing to
// cycle through).
func (m model) canPickServer() bool {
	return m.selected.mediaType == models.MediaTV && len(m.sonarrServers) > 1
}

func (m model) updateConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "s":
		if m.canPickServer() {
			m.serverIdx = (m.serverIdx + 1) % len(m.sonarrServers)
		}
	case "y":
		m.loading = true
		m.err = nil
		return m, submitRequestCmd(m.ctx, m.client, m.selected, m.selectedServerID())
	// "enter" cancels along with "n"/"esc": a bare enter on a [y/N] prompt
	// should take the shown default (N), not be a no-op.
	case "n", "esc", "enter":
		m.mode = modeDetail
	}
	return m, nil
}

// updateResult returns to browsing on any key; the global q/ctrl+c handling
// above already covers quitting from here.
func (m model) updateResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		m.mode = modeBrowsing
		m.err = nil
		m.lastRequest = nil
	}
	return m, nil
}

// updateRequests intercepts n/p (Seerr-side pagination), enter (open
// request detail) and esc/b (back to browsing, refetching it since m.list
// was just overwritten with requests); everything else is forwarded to the
// list untouched, mirroring updateBrowsing.
func (m model) updateRequests(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		var cmd tea.Cmd
		m.list, cmd = m.list.Update(msg)
		return m, cmd
	}

	switch keyMsg.String() {
	case "n":
		if m.loading || (m.requestsTotalPages > 0 && m.requestsPage >= m.requestsTotalPages) {
			return m, nil
		}
		m.requestsPage++
		return m.startRequestsFetch()

	case "p":
		if m.loading || m.requestsPage <= 1 {
			return m, nil
		}
		m.requestsPage--
		return m.startRequestsFetch()

	case "enter":
		if item, ok := m.list.SelectedItem().(requestItem); ok {
			m.selectedRequest = item
			m.mode = modeRequestDetail
		}
		return m, nil

	case "esc", "b":
		m.mode = modeBrowsing
		return m.startFetch()
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// updateRequestDetail mirrors updateDetail, but "d" (not "enter" — a
// destructive action shouldn't share a trigger with a navigation key) opens
// the delete confirmation instead of a request-creation one.
func (m model) updateRequestDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "d":
		m.mode = modeRequestConfirm
	case "esc", "b":
		m.mode = modeRequests
	}
	return m, nil
}

// updateRequestConfirm mirrors updateConfirm exactly, deleting instead of
// requesting.
func (m model) updateRequestConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "y":
		m.loading = true
		m.err = nil
		return m, deleteRequestCmd(m.ctx, m.client, m.selectedRequest.id)
	// "enter" cancels along with "n"/"esc": a bare enter on a [y/N] prompt
	// should take the shown default (N), not be a no-op.
	case "n", "esc", "enter":
		m.mode = modeRequestDetail
	}
	return m, nil
}

// updateRequestResult returns to the requests list on any key, same as
// updateResult — except it refetches on the way back, since staying on the
// stale list would keep showing the request that was just deleted.
func (m model) updateRequestResult(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		m.err = nil
		m.deletedRequestID = 0
		m.mode = modeRequests
		return m.startRequestsFetch()
	}
	return m, nil
}
