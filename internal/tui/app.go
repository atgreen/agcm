// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package tui

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/green/agcm/internal/about"
	"github.com/green/agcm/internal/api"
	"github.com/green/agcm/internal/config"
	"github.com/green/agcm/internal/export"
	"github.com/green/agcm/internal/tui/components"
	"github.com/green/agcm/internal/tui/styles"
	"github.com/mattn/go-runewidth"
)

// Pane represents which pane is focused
type Pane int

const (
	PaneList Pane = iota
	PaneDetail
)

// SortField represents the field to sort by
type SortField int

const (
	SortByLastModified SortField = iota
	SortByCreated
	SortBySeverity
	SortByCaseNumber
)

func (s SortField) String() string {
	switch s {
	case SortByLastModified:
		return "Last Modified"
	case SortByCreated:
		return "Date Created"
	case SortBySeverity:
		return "Severity"
	case SortByCaseNumber:
		return "Case Number"
	default:
		return "Unknown"
	}
}

// Options configures the TUI
type Options struct {
	Accounts    []string
	GroupNumber string
	MaskMode    bool
	Version     string
}

// CachedCaseDetail holds cached case details
type CachedCaseDetail struct {
	Case        *api.Case
	Comments    []api.Comment
	Attachments []api.Attachment
}

// Model is the main TUI model
type Model struct {
	client    *api.Client
	configMgr *config.Manager
	opts      Options
	styles    *styles.Styles
	keys      *styles.KeyMap
	width     int
	height    int
	ready     bool

	// Components
	caseList   *components.CaseList
	caseDetail *components.CaseDetail
	statusBar  *components.StatusBar
	spinner    spinner.Model
	modal      *components.Modal
	filePicker *components.FilePickerDialog

	// State
	currentPane      Pane
	showHelp         bool
	showAbout        bool
	cases            []api.Case
	sortField        SortField
	sortReverse      bool
	loadingCases     bool
	loadingPage      bool   // True when loading an additional page (append), not a fresh list
	loadingDetail    bool
	initialLoadDone  bool   // Set true after first successful case load
	loadGen          uint64 // Bumped on each fresh (non-append) case load to detect stale responses
	highlightedCase  string // Currently highlighted case number
	pendingFetch     string // Case number waiting to be fetched (debounce)
	detailCache      map[string]*CachedCaseDetail
	exporting        bool
	exportCancel     context.CancelFunc
	pendingExport    string // "single" or "bulk"
	exportCaseNumber string // For single export
	exportPath       string // File or directory path
	exportProgressCh chan export.Progress

	// Layout info for mouse
	listHeight     int
	detailY        int // Y position where detail pane starts
	layoutDebug    string
	scrollDrag     bool
	listScrollDrag bool

	// Quick search
	quickSearch     *components.QuickSearch
	quickSearchMode bool

	// Filter
	filterDialog *components.FilterDialog
	filterBar    *components.FilterBar
	activeFilter *api.CaseFilter
	totalCases   int    // Total cases before filtering (for display)
	nextCursor   string // GraphQL cursor for next page
	products     []string

	// Presets
	presetSaveMode bool   // True when waiting for digit to save preset
	activePreset   string // Currently active preset slot (empty if none)

	// Theme
	themeIndex int // Index into styles.Themes()

	// Text search within case
	textSearch     *components.TextSearch
	textSearchMode bool
}

// Messages
type casesLoadedMsg struct {
	cases      []api.Case
	totalCount int
	startIndex int
	requested  int    // maxResults that was sent in the request
	append     bool
	err        error
	gen        uint64 // Generation counter — stale non-append responses are dropped
	nextCursor string // GraphQL cursor for next page
}

type caseDetailLoadedMsg struct {
	caseNumber  string
	case_       *api.Case
	comments    []api.Comment
	attachments []api.Attachment
	err         error
	commentsErr error
	attachErr   error
}

type debounceTimeoutMsg struct {
	caseNumber string
}

type errMsg struct {
	err error
}

type exportProgressMsg struct {
	progress float64
	message  string
}

type exportCompleteMsg struct {
	outputPath string
	err        error
}

type quickSearchResultMsg struct {
	caseNumber string
	case_      *api.Case
	err        error
}

type productsLoadedMsg struct {
	products []string
	err      error
}

// Debounce delay for auto-fetching case details
const debounceDelay = 500 * time.Millisecond
const casePageSize = 100

// NewModel creates a new TUI model
func NewModel(client *api.Client, opts Options, configMgr *config.Manager) *Model {
	s, themeIndex := initialStyles(configMgr)
	keys := styles.DefaultKeyMap()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	caseList := components.NewCaseList(s, keys)
	caseList.SetMaskMode(opts.MaskMode)

	caseDetail := components.NewCaseDetail(s, keys)
	caseDetail.SetMaskMode(opts.MaskMode)

	return &Model{
		client:       client,
		configMgr:    configMgr,
		opts:         opts,
		styles:       s,
		keys:         keys,
		caseList:     caseList,
		caseDetail:   caseDetail,
		statusBar:    components.NewStatusBar(s, keys),
		spinner:      sp,
		modal:        components.NewModal(s),
		filePicker:   components.NewFilePickerDialog(s),
		quickSearch:  components.NewQuickSearch(s),
		filterDialog: components.NewFilterDialog(s),
		filterBar:    components.NewFilterBar(s),
		textSearch:   components.NewTextSearch(s),
		currentPane:  PaneList,
		sortField:    SortByLastModified,
		sortReverse:  true,
		themeIndex:   themeIndex,
		detailCache:  make(map[string]*CachedCaseDetail),
	}
}

// initialStyles picks the startup theme: the configured ui.theme when it
// names a known theme, otherwise auto-detection from the terminal background
func initialStyles(configMgr *config.Manager) (*styles.Styles, int) {
	if configMgr != nil {
		if idx := styles.ThemeIndex(configMgr.GetTheme()); idx >= 0 {
			return styles.NewStyles(styles.Themes()[idx].Colors), idx
		}
	}
	if lipgloss.HasDarkBackground() {
		return styles.DarkStyles(), styles.ThemeIndex("dark")
	}
	return styles.LightStyles(), styles.ThemeIndex("light")
}

// cycleTheme switches to the next theme, re-skins the UI in place, and
// persists the choice to the config file
func (m *Model) cycleTheme() {
	themes := styles.Themes()
	m.themeIndex = (m.themeIndex + 1) % len(themes)
	theme := themes[m.themeIndex]
	m.styles.Apply(theme.Colors)

	msg := "Theme: " + theme.Name
	if m.configMgr != nil {
		m.configMgr.Get().UI.Theme = theme.Name
		if err := m.configMgr.Save(); err != nil {
			msg += " (failed to save: " + err.Error() + ")"
		}
	}
	m.statusBar.SetMessage(m.styles.Label.Render(msg), 2*time.Second)
}

// Init implements tea.Model
func (m *Model) Init() tea.Cmd {
	m.loadingCases = true
	return tea.Batch(
		m.loadCasesPage(0, false),
		tea.EnterAltScreen,
		m.spinner.Tick,
		m.statusBar.SpinnerTick(),
	)
}

// loadCasesWithFilter loads the first page of cases for the given filter.
// It bumps loadGen so any in-flight non-append responses become stale.
func (m *Model) loadCasesWithFilter(filter *api.CaseFilter) tea.Cmd {
	m.loadGen++
	m.nextCursor = ""
	gen := m.loadGen
	return func() tea.Msg { return m.fetchCasesPage(filter, 0, false, gen) }
}

// loadCasesPage loads a page of cases under the active filter, optionally appending.
func (m *Model) loadCasesPage(start int, append bool) tea.Cmd {
	if !append {
		m.loadGen++
		m.nextCursor = ""
	}
	gen := m.loadGen
	return func() tea.Msg { return m.fetchCasesPage(m.activeFilter, start, append, gen) }
}

func (m *Model) fetchCasesPage(filter *api.CaseFilter, start int, append bool, gen uint64) tea.Msg {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	merged := m.withDefaults(filter, start, casePageSize)
	if append {
		merged.Cursor = m.nextCursor
	}
	result, err := m.client.ListCases(ctx, merged)
	if err != nil {
		return casesLoadedMsg{err: err, gen: gen}
	}
	return casesLoadedMsg{
		cases:      result.Items,
		totalCount: result.TotalCount,
		startIndex: result.StartIndex,
		requested:  merged.Count,
		append:     append,
		gen:        gen,
		nextCursor: result.NextCursor,
	}
}

func (m *Model) withDefaults(filter *api.CaseFilter, start, count int) *api.CaseFilter {
	req := &api.CaseFilter{
		Count:       count,
		StartIndex:  start,
		Accounts:    m.opts.Accounts,
		GroupNumber: m.opts.GroupNumber,
	}
	if filter == nil {
		return req
	}
	// If filter has accounts, use those instead of defaults
	if len(filter.Accounts) > 0 {
		req.Accounts = filter.Accounts
	}
	req.Status = append(req.Status, filter.Status...)
	req.Severity = append(req.Severity, filter.Severity...)
	req.Products = append(req.Products, filter.Products...)
	req.Keyword = filter.Keyword
	req.IncludeClosed = filter.IncludeClosed
	return req
}

// loadCaseDetail loads full case details
func (m *Model) loadCaseDetail(caseNumber string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Load case details
		c, err := m.client.GetCase(ctx, caseNumber)
		if err != nil {
			return caseDetailLoadedMsg{caseNumber: caseNumber, err: err}
		}

		// Load comments
		comments, commentsErr := m.client.GetCaseComments(ctx, caseNumber)
		if commentsErr != nil {
			comments = nil
		}

		// Load attachments
		attachments, attachErr := m.client.GetCaseAttachments(ctx, caseNumber)
		if attachErr != nil {
			attachments = nil
		}

		return caseDetailLoadedMsg{
			caseNumber:  caseNumber,
			case_:       c,
			comments:    comments,
			attachments: attachments,
			commentsErr: commentsErr,
			attachErr:   attachErr,
		}
	}
}

func (m *Model) fetchAllCaseNumbers(ctx context.Context, filter *api.CaseFilter) ([]string, error) {
	var cursor string
	var caseNumbers []string

	for {
		reqFilter := m.withDefaults(filter, 0, casePageSize)
		reqFilter.Cursor = cursor
		result, err := m.client.ListCases(ctx, reqFilter)
		if err != nil {
			return nil, err
		}
		for _, c := range result.Items {
			caseNumbers = append(caseNumbers, c.CaseNumber)
		}
		if result.NextCursor == "" || len(result.Items) == 0 {
			break
		}
		cursor = result.NextCursor
	}

	return caseNumbers, nil
}

func (m *Model) loadProducts() tea.Cmd {
	// Extract products from already-loaded cases to avoid a slow extra API call.
	if len(m.cases) > 0 {
		seen := make(map[string]bool)
		for _, c := range m.cases {
			if c.Product != "" {
				seen[c.Product] = true
			}
		}
		products := make([]string, 0, len(seen))
		for p := range seen {
			products = append(products, p)
		}
		sort.Strings(products)
		return func() tea.Msg { return productsLoadedMsg{products: products} }
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		names, err := m.client.ListCaseProducts(ctx)
		if err != nil {
			return productsLoadedMsg{err: err}
		}
		return productsLoadedMsg{products: names}
	}
}

// debounceCmd returns a command that fires after the debounce delay
func debounceCmd(caseNumber string) tea.Cmd {
	return tea.Tick(debounceDelay, func(t time.Time) tea.Msg {
		return debounceTimeoutMsg{caseNumber: caseNumber}
	})
}

func normalizeCaseNumber(value string) string {
	s := strings.TrimSpace(value)
	if s == "" {
		return s
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return s
		}
	}
	if len(s) < 8 {
		return strings.Repeat("0", 8-len(s)) + s
	}
	return s
}

// searchCaseByNumber performs direct case lookup by case number
func (m *Model) searchCaseByNumber(caseNumber string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		c, err := m.client.GetCase(ctx, caseNumber)
		return quickSearchResultMsg{
			caseNumber: caseNumber,
			case_:      c,
			err:        err,
		}
	}
}

// searchInCase searches for query in the current case content
func (m *Model) searchInCase(query string) []components.TextMatch {
	var matches []components.TextMatch
	query = strings.ToLower(query)

	// Get current case from detail view (has full data including description)
	c := m.caseDetail.GetCase()
	if c == nil {
		return matches
	}

	// Search in summary
	if strings.Contains(strings.ToLower(c.Summary), query) {
		matches = append(matches, components.TextMatch{TabIndex: 0, LineNumber: 0, Text: c.Summary})
	}

	// Search in description
	lines := strings.Split(c.Description, "\n")
	for i, line := range lines {
		if strings.Contains(strings.ToLower(line), query) {
			matches = append(matches, components.TextMatch{TabIndex: 0, LineNumber: i + 1, Text: line})
		}
	}

	// Search in cached comments
	if cached, ok := m.detailCache[c.CaseNumber]; ok && cached != nil {
		for i, comment := range cached.Comments {
			text := comment.GetText()
			commentLines := strings.Split(text, "\n")
			for j, line := range commentLines {
				if strings.Contains(strings.ToLower(line), query) {
					matches = append(matches, components.TextMatch{TabIndex: 1, LineNumber: i*100 + j, Text: line})
				}
			}
		}
	}

	return matches
}

// applyTextSearchQuery recomputes matches for the query, highlights them in
// the detail pane, and jumps to the first match
func (m *Model) applyTextSearchQuery(query string) {
	if query == "" {
		m.textSearch.SetMatches(nil)
		m.caseDetail.ClearSearchHighlight()
		return
	}
	matches := m.searchInCase(query)
	m.textSearch.SetMatches(matches)
	m.caseDetail.SetSearchHighlight(query)
	if len(matches) > 0 {
		m.caseDetail.SetActiveTab(matches[0].TabIndex)
		m.caseDetail.SetCurrentMatch(matches[0].TabIndex, matches[0].LineNumber)
		m.caseDetail.ScrollToMatch(matches[0].TabIndex, matches[0].LineNumber)
	}
}

// addOrSelectCase adds a case to the list if not present, then selects it
func (m *Model) addOrSelectCase(c *api.Case) {
	if c == nil {
		return
	}

	// Check if case already in list
	for i, existing := range m.cases {
		if existing.CaseNumber == c.CaseNumber {
			m.caseList.SetCursor(i)
			m.highlightedCase = c.CaseNumber
			m.loadingDetail = true
			return
		}
	}

	// Add to beginning of list
	m.cases = append([]api.Case{*c}, m.cases...)
	m.sortCases()

	// Find the new position and select it
	for i, existing := range m.cases {
		if existing.CaseNumber == c.CaseNumber {
			m.caseList.SetCursor(i)
			break
		}
	}
	m.highlightedCase = c.CaseNumber
}

// sortCases sorts the cases based on current sort settings, keeping the
// cursor on the same case across the re-order.
func (m *Model) sortCases() {
	selected := ""
	if sel := m.caseList.SelectedCase(); sel != nil {
		selected = sel.CaseNumber
	}
	sort.Slice(m.cases, func(i, j int) bool {
		var less bool
		switch m.sortField {
		case SortByLastModified:
			less = m.cases[i].LastModified.Before(m.cases[j].LastModified.Time)
		case SortByCreated:
			less = m.cases[i].CreatedDate.Before(m.cases[j].CreatedDate.Time)
		case SortBySeverity:
			less = m.cases[i].Severity < m.cases[j].Severity
		case SortByCaseNumber:
			less = m.cases[i].CaseNumber < m.cases[j].CaseNumber
		default:
			less = m.cases[i].LastModified.Before(m.cases[j].LastModified.Time)
		}
		if m.sortReverse {
			return !less
		}
		return less
	})
	m.caseList.SetCases(m.cases)
	m.caseList.SetSort(components.SortField(m.sortField), m.sortReverse)
	if selected != "" {
		for i, c := range m.cases {
			if c.CaseNumber == selected {
				m.caseList.SetCursor(i)
				break
			}
		}
	}
}

// cycleSortField cycles through sort fields
func (m *Model) cycleSortField() {
	m.sortField = (m.sortField + 1) % 4
	m.sortCases()
	m.statusBar.SetMessage(fmt.Sprintf("Sorted by: %s", m.sortField.String()), 2*time.Second)
}

// toggleSortOrder toggles sort order
func (m *Model) toggleSortOrder() {
	m.sortReverse = !m.sortReverse
	m.sortCases()
	order := "ascending"
	if m.sortReverse {
		order = "descending"
	}
	m.statusBar.SetMessage(fmt.Sprintf("Sort order: %s", order), 2*time.Second)
}

// mergeCases appends a fetched page, deduplicating by case number. The
// server pages by row offset over a list it sorts by last-modified, so a
// case modified between fetches shifts the rows and gets re-delivered on
// the next page; the fresher copy replaces the one already loaded.
func mergeCases(existing, page []api.Case) []api.Case {
	index := make(map[string]int, len(existing))
	for i, c := range existing {
		index[c.CaseNumber] = i
	}
	for _, c := range page {
		if i, ok := index[c.CaseNumber]; ok {
			existing[i] = c
			continue
		}
		index[c.CaseNumber] = len(existing)
		existing = append(existing, c)
	}
	return existing
}

// resetForReload clears cached details and counts before loading a fresh case list
func (m *Model) resetForReload() {
	m.loadingCases = true
	m.detailCache = make(map[string]*CachedCaseDetail)
	m.totalCases = 0
	m.nextCursor = ""
}

// checkHighlightChange checks if the highlighted case changed and triggers debounced fetch
func (m *Model) checkHighlightChange() tea.Cmd {
	selected := m.caseList.SelectedCase()
	if selected == nil {
		return nil
	}

	newCase := selected.CaseNumber
	if newCase == m.highlightedCase {
		return nil
	}

	m.highlightedCase = newCase

	// Check cache first
	if cached, ok := m.detailCache[newCase]; ok {
		m.caseDetail.SetCase(cached.Case)
		m.caseDetail.SetComments(cached.Comments)
		m.caseDetail.SetAttachments(cached.Attachments)
		return nil
	}

	// Start debounce timer
	m.pendingFetch = newCase
	return debounceCmd(newCase)
}

// Update implements tea.Model
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle product list loading even when dialogs are visible.
	if pl, ok := msg.(productsLoadedMsg); ok {
		if pl.err != nil {
			errText := pl.err.Error()
			m.statusBar.SetMessage(m.styles.Warning.Render("Failed to load products: "+errText), 5*time.Second)
			m.filterDialog.SetProductsError(errText)
		} else {
			m.products = pl.products
			m.filterDialog.SetProducts(m.products)
		}
		return m, nil
	}

	// Handle file picker input first
	if m.filePicker.IsVisible() {
		filePicker, cmd := m.filePicker.Update(msg)
		m.filePicker = filePicker
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
		// If file picker is no longer visible and we have a pending export with path, start it
		if !m.filePicker.IsVisible() {
			if m.pendingExport != "" && m.exportPath != "" {
				var exportCmd tea.Cmd
				switch m.pendingExport {
				case "single":
					exportCmd = m.startSingleExport(m.exportCaseNumber, m.exportPath)
				case "bulk":
					exportCmd = m.startBulkExport(m.exportPath)
				case "bundle":
					exportCmd = m.startBundleExport(m.exportPath)
				}
				m.pendingExport = ""
				m.exportPath = ""
				if exportCmd != nil {
					cmds = append(cmds, exportCmd)
				}
			}
		}
		return m, tea.Batch(cmds...)
	}

	// Handle progress modal input
	if m.modal.IsVisible() {
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			modal, cmd := m.modal.Update(keyMsg)
			m.modal = modal
			return m, cmd
		}
	}

	// Handle quick search input
	if m.quickSearchMode {
		quickSearch, cmd := m.quickSearch.Update(msg)
		m.quickSearch = quickSearch
		if !m.quickSearch.IsVisible() {
			m.quickSearchMode = false
		}
		return m, cmd
	}

	// Handle filter dialog input
	if m.filterDialog.IsVisible() {
		filterDialog, cmd := m.filterDialog.Update(msg)
		m.filterDialog = filterDialog
		return m, cmd
	}

	// Handle text search input
	if m.textSearchMode && m.textSearch.IsVisible() {
		// Handle search query messages here (they come back from textSearch.Update)
		if queryMsg, ok := msg.(components.TextSearchQueryMsg); ok {
			m.applyTextSearchQuery(queryMsg.Query)
			return m, nil
		}

		// Handle navigation between matches
		if navMsg, ok := msg.(components.TextSearchNavigateMsg); ok {
			if navMsg.Match != nil {
				// Switch to the tab containing the match and scroll to it
				m.caseDetail.SetActiveTab(navMsg.Match.TabIndex)
				// Update the current match highlighting (must happen before scroll to update line offsets)
				m.caseDetail.SetCurrentMatch(navMsg.Match.TabIndex, navMsg.Match.LineNumber)
				// Scroll to the actual viewport line for this match
				m.caseDetail.ScrollToMatch(navMsg.Match.TabIndex, navMsg.Match.LineNumber)
			}
			return m, nil
		}

		// Allow mouse events to pass through for scrolling
		if _, ok := msg.(tea.MouseMsg); ok {
			// Don't intercept mouse events - let them fall through to normal handling
		} else {
			textSearch, cmd := m.textSearch.Update(msg)
			m.textSearch = textSearch
			if !m.textSearch.IsVisible() {
				m.textSearchMode = false
				m.caseDetail.ClearSearchHighlight()
			}
			return m, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.updateLayout()
		m.modal.SetSize(msg.Width, msg.Height)
		m.filePicker.SetSize(msg.Width, msg.Height)

	case tea.MouseMsg:
		if !m.modal.IsVisible() && !m.filePicker.IsVisible() {
			cmds = append(cmds, m.handleMouse(msg))
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if m.loadingCases || m.loadingDetail || m.exporting {
			cmds = append(cmds, cmd)
		}

	case exportProgressMsg:
		m.modal.UpdateProgress(msg.progress, msg.message)
		cmds = append(cmds, m.spinner.Tick)
		cmds = append(cmds, m.waitExportProgress())

	case exportCompleteMsg:
		m.exporting = false
		m.exportProgressCh = nil
		if m.exportCancel != nil {
			m.exportCancel()
			m.exportCancel = nil
		}
		m.modal.Hide()
		switch {
		case errors.Is(msg.err, context.Canceled):
			m.statusBar.SetMessage(m.styles.Warning.Render("Export cancelled"), 3*time.Second)
		case msg.err != nil:
			m.statusBar.SetMessage(m.styles.Error.Render("Export failed: "+msg.err.Error()), 5*time.Second)
		default:
			m.statusBar.SetMessage(m.styles.Success.Render("Exported to: "+msg.outputPath), 5*time.Second)
		}

	case components.QuickSearchSubmitMsg:
		m.quickSearchMode = false
		caseNumber := normalizeCaseNumber(msg.CaseNumber)
		m.statusBar.SetMessage(m.styles.Muted.Render("Searching for case "+caseNumber+"..."), 0)
		return m, m.searchCaseByNumber(caseNumber)

	case components.QuickSearchCancelMsg:
		m.quickSearchMode = false

	case quickSearchResultMsg:
		if msg.err != nil {
			m.statusBar.SetMessage(m.styles.Error.Render("Case not found: "+msg.caseNumber), 3*time.Second)
		} else {
			m.addOrSelectCase(msg.case_)
			m.statusBar.SetMessage(m.styles.Success.Render("Found case: "+msg.caseNumber), 2*time.Second)
		}

	case components.FilterApplyMsg:
		m.activeFilter = msg.Filter
		m.resetForReload()
		m.statusBar.SetMessage(m.styles.Muted.Render("Applying filter..."), 0)
		return m, tea.Batch(m.loadCasesWithFilter(msg.Filter), m.spinner.Tick)

	case components.FilterClearMsg:
		m.activeFilter = nil
		m.resetForReload()
		m.statusBar.SetMessage(m.styles.Muted.Render("Clearing filter..."), 0)
		return m, tea.Batch(m.loadCasesPage(0, false), m.spinner.Tick)

	case components.FilterCancelMsg:
		// Dialog closed without changes

	case components.TextSearchCloseMsg:
		m.textSearchMode = false
		m.caseDetail.ClearSearchHighlight()

	case components.TextSearchQueryMsg:
		m.applyTextSearchQuery(msg.Query)

	case debounceTimeoutMsg:
		// Only fetch if this is still the pending case
		if msg.caseNumber == m.pendingFetch && msg.caseNumber != "" {
			m.pendingFetch = ""
			m.loadingDetail = true
			cmds = append(cmds, m.loadCaseDetail(msg.caseNumber), m.spinner.Tick)
		}

	case tea.KeyMsg:
		// Global keys
		if key.Matches(msg, m.keys.Quit) {
			return m, tea.Quit
		}

		// Any key dismisses help screen or About box
		if m.showHelp || m.showAbout {
			m.showHelp = false
			m.showAbout = false
			return m, nil
		}

		// Preset save mode is modal: the next key picks a slot or cancels
		if m.presetSaveMode {
			m.presetSaveMode = false
			s := msg.String()
			if len(s) == 1 && s >= "0" && s <= "9" {
				m.savePreset(s)
			} else {
				m.statusBar.SetMessage(m.styles.Muted.Render("Preset save cancelled"), 2*time.Second)
			}
			return m, nil
		}

		if key.Matches(msg, m.keys.Help) {
			m.showHelp = true
			return m, nil
		}

		// About box (a)
		if key.Matches(msg, m.keys.About) {
			m.showAbout = true
			return m, nil
		}

		// Tab navigation between list and detail
		if key.Matches(msg, m.keys.Tab) || key.Matches(msg, m.keys.ShiftTab) {
			m.togglePane()
			return m, nil
		}

		// Refresh
		if key.Matches(msg, m.keys.Refresh) {
			m.loadingCases = true
			m.detailCache = make(map[string]*CachedCaseDetail)
			m.nextCursor = ""
			return m, tea.Batch(m.loadCasesPage(0, false), m.spinner.Tick)
		}

		// Cycle color theme (t)
		if key.Matches(msg, m.keys.Theme) {
			m.cycleTheme()
			return m, nil
		}

		// Quick search by case number (/)
		if key.Matches(msg, m.keys.Search) {
			m.quickSearchMode = true
			return m, m.quickSearch.Show()
		}

		// Filter dialog (f)
		if key.Matches(msg, m.keys.Filter) {
			cmds = append(cmds, m.filterDialog.ShowWithFilter(m.activeFilter))
			if len(m.products) == 0 {
				m.filterDialog.SetProductsLoading()
				cmds = append(cmds, m.loadProducts())
			} else {
				m.filterDialog.SetProducts(m.products)
			}
			return m, tea.Batch(cmds...)
		}

		// Clear filter (F)
		if key.Matches(msg, m.keys.ClearFilter) && (m.activeFilter != nil || m.activePreset != "") {
			m.activeFilter = nil
			m.activePreset = ""
			m.filterBar.Clear()
			m.filterBar.ClearPreset()
			m.updateLayout()
			m.resetForReload()
			m.statusBar.SetMessage(m.styles.Muted.Render("Filter cleared"), 2*time.Second)
			return m, tea.Batch(m.loadCasesPage(0, false), m.spinner.Tick)
		}

		// Preset save mode (Ctrl+s)
		if key.Matches(msg, m.keys.PresetSave) {
			m.presetSaveMode = true
			m.statusBar.SetMessage(m.styles.Label.Render("Press 1-9 or 0 to save current filter to preset slot..."), 5*time.Second)
			return m, nil
		}

		// Handle digit keys for loading presets (1-9, 0)
		if key.Matches(msg, m.keys.PresetLoad) {
			slot := msg.String()
			preset := m.configMgr.GetPreset(slot)
			if preset == nil {
				m.statusBar.SetMessage(m.styles.Muted.Render(fmt.Sprintf("No preset in slot %s (Ctrl+s to save)", slot)), 2*time.Second)
				return m, nil
			}
			m.activeFilter = m.presetToFilter(preset)
			m.activePreset = slot
			m.filterBar.SetFilter(m.activeFilter, 0, 0)
			m.filterBar.SetPreset(slot, preset.Name)
			m.updateLayout()
			m.loadingCases = true
			m.detailCache = make(map[string]*CachedCaseDetail)
			name := preset.Name
			if name == "" {
				name = "Preset " + slot
			}
			m.statusBar.SetMessage(m.styles.Success.Render(fmt.Sprintf("Loaded preset %s: %s", slot, name)), 2*time.Second)
			return m, tea.Batch(m.loadCasesWithFilter(m.activeFilter), m.spinner.Tick)
		}

		// Open selected case in the support portal (o)
		if key.Matches(msg, m.keys.Open) {
			if sel := m.caseList.SelectedCase(); sel != nil {
				return m, openURL(casePortalURL(sel.CaseNumber))
			}
			m.statusBar.SetMessage(m.styles.Warning.Render("No case selected"), 2*time.Second)
			return m, nil
		}

		// Sort controls
		if key.Matches(msg, m.keys.Sort) {
			m.cycleSortField()
			return m, nil
		}
		if key.Matches(msg, m.keys.SortOrder) {
			m.toggleSortOrder()
			return m, nil
		}

		// Export current case (e)
		if key.Matches(msg, m.keys.Export) {
			c := m.caseDetail.GetCase()
			if c == nil {
				m.statusBar.SetMessage(m.styles.Warning.Render("No case selected"), 2*time.Second)
				return m, nil
			}
			m.exportCaseNumber = c.CaseNumber
			return m, m.promptExport("single", "Export Case",
				fmt.Sprintf("Export case %s to markdown file", c.CaseNumber),
				components.FilePickerModeFile, fmt.Sprintf("case-%s.md", c.CaseNumber))
		}

		// Export all cases (E)
		if key.Matches(msg, m.keys.BulkExport) {
			if len(m.cases) == 0 {
				m.statusBar.SetMessage(m.styles.Warning.Render("No cases loaded"), 2*time.Second)
				return m, nil
			}
			return m, m.promptExport("bulk", "Export All Cases",
				fmt.Sprintf("Select directory for %d cases", len(m.cases)),
				components.FilePickerModeDir, "./exports")
		}

		// Bundle export (B) - export to bundled markdown files
		if key.Matches(msg, m.keys.BundleExport) {
			if len(m.cases) == 0 {
				m.statusBar.SetMessage(m.styles.Warning.Render("No cases loaded"), 2*time.Second)
				return m, nil
			}
			return m, m.promptExport("bundle", "Bundle Export",
				fmt.Sprintf("Select directory for %d cases (4MB bundles)", len(m.cases)),
				components.FilePickerModeDir, "./exports")
		}

		// Global left/right for tab switching in detail pane
		if key.Matches(msg, m.keys.Left) || key.Matches(msg, m.keys.Right) {
			// Switch focus to detail pane when using left/right
			m.currentPane = PaneDetail
			m.updateFocus()
			caseDetail, cmd := m.caseDetail.Update(msg)
			m.caseDetail = caseDetail
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}
		// Global next/previous comment shortcuts (only in Comments tab)
		if (key.Matches(msg, m.keys.NextComment) || key.Matches(msg, m.keys.PrevComment)) && m.caseDetail.ActiveTab() == 1 {
			caseDetail, cmd := m.caseDetail.Update(msg)
			m.caseDetail = caseDetail
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		// Pass to focused component
		switch m.currentPane {
		case PaneList:
			// Enter moves focus to the detail pane
			if key.Matches(msg, m.keys.Select) {
				m.currentPane = PaneDetail
				m.updateFocus()
				return m, nil
			}

			caseList, cmd := m.caseList.Update(msg)
			m.caseList = caseList
			cmds = append(cmds, cmd)

			// Check if highlight changed (for any key that might move selection)
			cmds = append(cmds, m.checkHighlightChange())
			cmds = append(cmds, m.maybeLoadMoreCases())

		case PaneDetail:
			if key.Matches(msg, m.keys.Back) {
				m.currentPane = PaneList
				return m, nil
			}

			// Ctrl+F for text search within case
			if key.Matches(msg, m.keys.TextSearch) {
				m.textSearchMode = true
				m.textSearch.SetWidth(m.width)
				return m, m.textSearch.Show()
			}

			caseDetail, cmd := m.caseDetail.Update(msg)
			m.caseDetail = caseDetail
			cmds = append(cmds, cmd)
		}

	case casesLoadedMsg:
		m.loadingCases = false
		m.loadingPage = false
		selectedCase := ""
		savedOffset := m.caseList.GetOffset()
		if sel := m.caseList.SelectedCase(); sel != nil {
			selectedCase = sel.CaseNumber
		}
		if msg.err != nil {
			if msg.gen < m.loadGen {
				break // Stale error from a superseded load — ignore
			}
			m.statusBar.SetConnected(false)
			m.statusBar.SetMessage(m.styles.Error.Render("Error: "+msg.err.Error()), 5*time.Second)
		} else if !msg.append && msg.gen < m.loadGen {
			// Stale non-append response: a newer load was triggered (e.g. preset
			// applied while the initial load was in flight) — drop it.
			m.statusBar.SetConnected(true)
		} else if msg.append && (msg.gen < m.loadGen || msg.startIndex != len(m.cases)) {
			// Stale page: a refresh or filter change replaced the list while
			// this fetch was in flight — drop it. maybeLoadMoreCases will
			// re-request from the correct offset if still needed.
			m.statusBar.SetConnected(true)
		} else {
			m.statusBar.SetConnected(true)
			if msg.append {
				prevLen := len(m.cases)
				m.cases = mergeCases(m.cases, msg.cases)
				if len(m.cases) == prevLen || len(msg.cases) == 0 {
					// No new unique cases from this page — the API's
					// totalCount exceeds what it can actually return.
					// Cap to stop further paging.
					m.totalCases = len(m.cases)
				} else if msg.totalCount > 0 {
					m.totalCases = msg.totalCount
				}
			} else {
				m.cases = msg.cases
				m.initialLoadDone = true // First load complete
				if msg.totalCount > 0 {
					m.totalCases = msg.totalCount
				} else {
					m.totalCases = len(m.cases)
				}
				// If the API returned fewer cases than requested, it has
				// no more to give regardless of what totalCount claims.
				if msg.requested > 0 && len(msg.cases) < msg.requested && len(msg.cases) < m.totalCases {
					m.totalCases = len(m.cases)
				}
			}
			m.sortCases()
			m.nextCursor = msg.nextCursor
			if msg.append {
				// Stop any active scrollbar drag - case count changed so drag math is invalid
				m.listScrollDrag = false
				// Restore cursor position first (this calls ensureVisible which may change offset)
				if selectedCase != "" {
					for i, c := range m.cases {
						if c.CaseNumber == selectedCase {
							m.caseList.SetCursor(i)
							break
						}
					}
				}
				// Then restore scroll offset (overrides ensureVisible's changes)
				m.caseList.SetOffset(savedOffset)
			}
			// Update filter bar
			if m.activeFilter != nil {
				m.filterBar.SetFilter(m.activeFilter, len(m.cases), m.totalCases)
			} else {
				m.filterBar.Clear()
			}
			// Recalculate layout when filter bar visibility changes
			m.updateLayout()
			// Clear detail panel if no cases loaded, otherwise trigger highlight check
			if len(m.cases) == 0 {
				m.highlightedCase = ""
				m.caseDetail.SetCase(nil)
				m.caseDetail.SetComments(nil)
				m.caseDetail.SetAttachments(nil)
			} else {
				cmds = append(cmds, m.checkHighlightChange())
			}
		}

	case caseDetailLoadedMsg:
		m.loadingDetail = false
		if msg.err != nil {
			m.statusBar.SetMessage(m.styles.Error.Render("Error: "+msg.err.Error()), 5*time.Second)
		} else {
			// Cache the result
			m.detailCache[msg.caseNumber] = &CachedCaseDetail{
				Case:        msg.case_,
				Comments:    msg.comments,
				Attachments: msg.attachments,
			}
			// Sync the case list entry with fresh REST API data so the
			// list reflects real-time status instead of stale Hydra/Solr data.
			for i, c := range m.cases {
				if c.CaseNumber == msg.caseNumber {
					m.cases[i].Status = msg.case_.Status
					m.cases[i].Severity = msg.case_.Severity
					m.cases[i].Summary = msg.case_.Summary
					m.cases[i].Product = msg.case_.Product
					m.cases[i].Version = msg.case_.Version
					m.cases[i].LastModified = msg.case_.LastModified
					break
				}
			}
			// Only update display if this is still the highlighted case
			if msg.caseNumber == m.highlightedCase {
				m.caseDetail.SetCase(msg.case_)
				m.caseDetail.SetComments(msg.comments)
				m.caseDetail.SetAttachments(msg.attachments)
			}
			// Show errors for comments/attachments if any
			if msg.commentsErr != nil {
				m.statusBar.SetMessage(m.styles.Warning.Render("Comments: "+msg.commentsErr.Error()), 3*time.Second)
			}
		}

	case errMsg:
		m.statusBar.SetMessage(m.styles.Error.Render("Error: "+msg.err.Error()), 5*time.Second)
	}

	// Update status bar (only show loading in status bar after initial load; overlay handles initial)
	m.statusBar.SetLoading(m.loadingCases && m.initialLoadDone, "Loading cases...")
	statusBar, cmd := m.statusBar.Update(msg)
	m.statusBar = statusBar
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *Model) togglePane() {
	if m.currentPane == PaneList {
		m.currentPane = PaneDetail
	} else {
		m.currentPane = PaneList
	}
	m.updateFocus()
}

// promptExport opens the file picker for an export, remembering which kind
// to start once a path is chosen
func (m *Model) promptExport(kind, title, desc string, mode components.FilePickerMode, defaultPath string) tea.Cmd {
	m.pendingExport = kind
	return m.filePicker.Show(title, desc, mode, defaultPath,
		func(path string) { m.exportPath = path },
		func() { m.pendingExport = "" },
	)
}

// cancelExport aborts any in-flight export; wired to Esc in the progress modal.
func (m *Model) cancelExport() {
	if m.exportCancel != nil {
		m.exportCancel()
	}
}

func (m *Model) startSingleExport(caseNumber, filename string) tea.Cmd {
	m.exporting = true
	ctx, cancel := context.WithCancel(context.Background())
	m.exportCancel = cancel
	m.modal.ShowProgress("Exporting Case", "Preparing export...", m.cancelExport)

	return func() tea.Msg {
		opts := export.DefaultOptions()
		opts.OutputFile = filename

		exporter, err := export.NewExporter(m.client, opts)
		if err != nil {
			return exportCompleteMsg{err: err}
		}

		err = exporter.ExportCaseToFile(ctx, caseNumber, filename)
		if err != nil {
			return exportCompleteMsg{err: err}
		}

		absPath, _ := filepath.Abs(filename)
		return exportCompleteMsg{outputPath: absPath}
	}
}

func (m *Model) startBulkExport(outputDir string) tea.Cmd {
	m.exporting = true
	ctx, cancel := context.WithCancel(context.Background())
	m.exportCancel = cancel
	m.modal.ShowProgress("Exporting Cases", "Starting export...", m.cancelExport)
	progressCh := make(chan export.Progress, 10)
	m.exportProgressCh = progressCh
	filter := m.activeFilter
	exportCmd := func() tea.Msg {
		defer close(progressCh)

		opts := export.DefaultOptions()
		opts.OutputDir = outputDir

		exporter, err := export.NewExporter(m.client, opts)
		if err != nil {
			return exportCompleteMsg{err: err}
		}

		caseNumbers, err := m.fetchAllCaseNumbers(ctx, filter)
		if err != nil {
			return exportCompleteMsg{err: err}
		}

		_, err = exporter.ExportCases(ctx, caseNumbers, progressCh)

		absPath, _ := filepath.Abs(outputDir)
		return exportCompleteMsg{outputPath: absPath, err: err}
	}
	return tea.Batch(exportCmd, m.waitExportProgress())
}

func (m *Model) startBundleExport(outputDir string) tea.Cmd {
	m.exporting = true
	ctx, cancel := context.WithCancel(context.Background())
	m.exportCancel = cancel
	m.modal.ShowProgress("Bundle Export", "Starting bundle export...", m.cancelExport)
	progressCh := make(chan export.Progress, 10)
	m.exportProgressCh = progressCh
	filter := m.activeFilter

	exportCmd := func() tea.Msg {
		defer close(progressCh)

		caseNumbers, err := m.fetchAllCaseNumbers(ctx, filter)
		if err != nil {
			return exportCompleteMsg{err: err}
		}

		exporter, err := export.NewExporter(m.client, export.DefaultOptions())
		if err != nil {
			return exportCompleteMsg{err: err}
		}

		if _, _, err := exporter.ExportBundles(ctx, caseNumbers, outputDir, progressCh); err != nil {
			return exportCompleteMsg{err: err}
		}

		absPath, _ := filepath.Abs(outputDir)
		return exportCompleteMsg{outputPath: absPath}
	}

	return tea.Batch(exportCmd, m.waitExportProgress())
}

func (m *Model) updateFocus() {
	m.caseList.SetFocused(m.currentPane == PaneList)
	m.caseDetail.SetFocused(m.currentPane == PaneDetail)
}

func (m *Model) maybeLoadMoreCases() tea.Cmd {
	if m.loadingCases {
		return nil
	}
	if m.totalCases == 0 || len(m.cases) >= m.totalCases {
		return nil
	}
	visible := m.caseList.VisibleRows()
	if m.caseList.GetOffset()+visible >= len(m.cases)-1 {
		m.loadingCases = true
		m.loadingPage = true
		return tea.Batch(m.loadCasesPage(len(m.cases), true), m.spinner.Tick)
	}
	return nil
}

func (m *Model) updateLayout() {
	headerHeight := 1
	footerHeight := 1
	filterBarHeight := 0
	if m.filterBar.HasActiveFilter() {
		filterBarHeight = 2
	}
	// Subtract 1 to avoid an extra padding line at the bottom
	contentHeight := m.height - headerHeight - footerHeight - filterBarHeight - 1
	if contentHeight < 2 {
		contentHeight = 2
	}

	// List takes ~1/3 of content, capped
	listHeight := contentHeight / 3
	if listHeight < 6 {
		listHeight = 6
	}
	if listHeight > 10 {
		listHeight = 10
	}
	// Detail gets the rest
	detailHeight := contentHeight - listHeight
	if detailHeight < 6 {
		detailHeight = 6
		listHeight = contentHeight - detailHeight
	}
	if listHeight < 1 {
		listHeight = 1
		detailHeight = contentHeight - listHeight
	}
	if detailHeight < 1 {
		detailHeight = 1
		listHeight = contentHeight - detailHeight
	}

	if os.Getenv("AGCM_DEBUG_LAYOUT") != "" {
		totalUsed := headerHeight + footerHeight + filterBarHeight + listHeight + detailHeight
		m.layoutDebug = fmt.Sprintf("DEBUG term %dx%d content %d list %d detail %d total %d", m.width, m.height, contentHeight, listHeight, detailHeight, totalUsed)
	} else {
		m.layoutDebug = ""
	}

	// Store layout for mouse handling
	m.listHeight = listHeight
	m.detailY = headerHeight + filterBarHeight + listHeight

	m.caseList.SetSize(m.width, listHeight)
	m.caseList.SetDebugInfo(m.layoutDebug)
	m.caseDetail.SetSize(m.width, detailHeight)
	m.statusBar.SetWidth(m.width)
	m.filterBar.SetWidth(m.width)

	m.updateFocus()
}

func (m *Model) handleMouse(msg tea.MouseMsg) tea.Cmd {
	headerHeight := 1
	filterBarHeight := 0
	if m.filterBar.HasActiveFilter() {
		filterBarHeight = 2
	}
	listTop := headerHeight + filterBarHeight
	listHeaderY := listTop + 1 // Column headers row (after border)
	detailTop := m.detailY
	// Inside detail viewport: border (1), tabs (1), separator (1)
	detailViewportTop := detailTop + 3

	switch msg.Button {
	case tea.MouseButtonLeft:
		if msg.Action == tea.MouseActionRelease {
			m.scrollDrag = false
			m.listScrollDrag = false
			return nil
		}
		if msg.Action == tea.MouseActionMotion && m.scrollDrag {
			// Continue dragging scrollbar thumb
			if msg.Y >= detailViewportTop {
				relY := msg.Y - detailViewportTop
				m.caseDetail.ScrollToRelativeLine(relY)
			}
			return nil
		}
		if msg.Action == tea.MouseActionMotion && m.listScrollDrag {
			// Continue dragging list scrollbar thumb (clamp Y to valid range)
			y := msg.Y
			if y <= listTop {
				y = listTop + 1
			}
			if y >= m.detailY {
				y = m.detailY - 1
			}
			relY := y - (listTop + 1)
			m.caseList.ScrollToRelativeLine(relY)
			if cmd := m.maybeLoadMoreCases(); cmd != nil {
				return cmd
			}
			return nil
		}
		if msg.Action != tea.MouseActionPress {
			return nil
		}

		// Click on column headers in case list; hit-testing is derived from
		// the same column widths the list renders with
		if msg.Y == listHeaderY {
			switch m.caseList.HeaderColumnAt(msg.X - 1) { // -1 for border
			case components.HeaderColCase:
				if m.sortField == SortByCaseNumber {
					m.toggleSortOrder()
				} else {
					m.sortField = SortByCaseNumber
					m.sortCases()
				}
			case components.HeaderColDate:
				// Toggle between LastModified and Created
				switch m.sortField {
				case SortByLastModified:
					m.sortField = SortByCreated
					m.sortCases()
				case SortByCreated:
					m.toggleSortOrder()
				default:
					m.sortField = SortByLastModified
					m.sortCases()
				}
			case components.HeaderColSev:
				if m.sortField == SortBySeverity {
					m.toggleSortOrder()
				} else {
					m.sortField = SortBySeverity
					m.sortCases()
				}
			}
			return nil
		}

		// Click on list scrollbar area (right-most 5 columns)
		if msg.Y > listTop && msg.Y < m.detailY && msg.X >= m.width-5 {
			m.listScrollDrag = true
			y := msg.Y
			if y <= listTop {
				y = listTop + 1
			}
			relY := y - (listTop + 1)
			m.caseList.ScrollToRelativeLine(relY)
			return m.maybeLoadMoreCases()
		}

		// Click in case list data area (not on scrollbar)
		if msg.Y > listHeaderY+1 && msg.Y < m.detailY && msg.X < m.width-5 {
			m.currentPane = PaneList
			m.updateFocus()

			// Calculate which row was clicked (listTop + border=1, colheader=1, separator=1)
			rowOffset := msg.Y - listTop - 3
			if rowOffset >= 0 {
				clickedIdx := m.caseList.GetOffset() + rowOffset
				if clickedIdx >= 0 && clickedIdx < len(m.cases) {
					m.caseList.SetCursor(clickedIdx)
					return m.checkHighlightChange()
				}
			}
		}

		// Click in detail area
		if msg.Y >= m.detailY {
			m.currentPane = PaneDetail
			m.updateFocus()

			// Click/drag on scrollbar area (right-most 2 columns inside border)
			if msg.X >= m.width-3 && msg.X <= m.width-2 && msg.Y >= detailViewportTop {
				m.scrollDrag = true
				relY := msg.Y - detailViewportTop
				m.caseDetail.ScrollToRelativeLine(relY)
				return nil
			}

			// Click-to-open links in detail viewport
			if msg.Y >= detailViewportTop && msg.X > 0 && msg.X < m.width-3 {
				relX := msg.X - 1
				relY := msg.Y - detailViewportTop
				if url, ok := m.caseDetail.LinkAt(relX, relY); ok {
					return openURL(url)
				}
			}

			// Check if clicking on tabs (first row inside border)
			tabRowY := m.detailY + 1
			if msg.Y == tabRowY {
				x := msg.X - 2 // Account for border padding
				// Tab layout: " Details  | Comments  | Attachments "
				if x >= 0 && x < 10 {
					m.caseDetail.SetActiveTab(0) // Details
				} else if x >= 13 && x < 24 {
					m.caseDetail.SetActiveTab(1) // Comments
				} else if x >= 27 && x < 42 {
					m.caseDetail.SetActiveTab(2) // Attachments
				}
			}
		}

	case tea.MouseButtonWheelUp:
		if msg.Y >= m.detailY {
			m.caseDetail.ScrollUp(3)
		} else if msg.Y >= headerHeight && msg.Y < m.detailY {
			m.caseList.ScrollUp(1)
			return tea.Batch(m.checkHighlightChange(), m.maybeLoadMoreCases())
		}

	case tea.MouseButtonWheelDown:
		if msg.Y >= m.detailY {
			m.caseDetail.ScrollDown(3)
		} else if msg.Y >= headerHeight && msg.Y < m.detailY {
			m.caseList.ScrollDown(1)
			return tea.Batch(m.checkHighlightChange(), m.maybeLoadMoreCases())
		}
	case tea.MouseButtonRight:
		if msg.Action != tea.MouseActionPress {
			return nil
		}
		// Right-click on case list row opens in support portal
		if msg.Y > listHeaderY+1 && msg.Y < m.detailY {
			// Calculate which row was clicked (listTop + border=1, colheader=1, separator=1)
			rowOffset := msg.Y - listTop - 3
			if rowOffset >= 0 {
				clickedIdx := m.caseList.GetOffset() + rowOffset
				if clickedIdx >= 0 && clickedIdx < len(m.cases) {
					return openURL(casePortalURL(m.cases[clickedIdx].CaseNumber))
				}
			}
		}
	}

	return nil
}

func casePortalURL(caseNumber string) string {
	return fmt.Sprintf("https://access.redhat.com/support/cases/#/case/%s", caseNumber)
}

func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "windows":
			cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
		default:
			cmd = exec.Command("xdg-open", url)
		}
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
		if err := cmd.Start(); err != nil {
			return errMsg{err: err}
		}
		return nil
	}
}

func (m *Model) waitExportProgress() tea.Cmd {
	if m.exportProgressCh == nil {
		return nil
	}
	return func() tea.Msg {
		p, ok := <-m.exportProgressCh
		if !ok {
			return nil
		}
		msg := fmt.Sprintf("Exporting %d/%d", p.CompletedCases, p.TotalCases)
		progress := 0.0
		if p.TotalCases > 0 {
			progress = float64(p.CompletedCases) / float64(p.TotalCases)
		}
		return exportProgressMsg{progress: progress, message: msg}
	}
}

// Minimum terminal size for a usable layout
const (
	minTermWidth  = 40
	minTermHeight = 10
)

// View implements tea.Model
func (m *Model) View() string {
	if !m.ready {
		// Show a simple loading message with spinner before window size is known
		if m.loadingCases {
			return m.spinner.View() + " Loading cases..."
		}
		return "Loading..."
	}

	if m.width < minTermWidth || m.height < minTermHeight {
		msg := fmt.Sprintf("Terminal too small\nNeed at least %dx%d, have %dx%d",
			minTermWidth, minTermHeight, m.width, m.height)
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
	}

	// Header with version (and debug if enabled); sort state is shown by the
	// arrow in the case list column headers
	versionText := ""
	if m.opts.Version != "" {
		versionText = " " + m.opts.Version
	}
	headerText := "agcm" + versionText
	if m.layoutDebug != "" {
		headerText += m.styles.Muted.Render(m.layoutDebug)
	}
	header := m.styles.Header.
		Width(m.width).
		Render(headerText)
	header = trimTrailingNewlines(header)

	// Filter bar (conditional)
	filterBar := ""
	if m.filterBar.HasActiveFilter() {
		filterBar = trimTrailingNewlines(m.filterBar.View())
	}

	// Main content area
	var content string

	if m.showHelp {
		content = trimTrailingNewlines(m.renderHelp())
	} else {
		list := trimTrailingNewlines(m.caseList.View())
		detail := trimTrailingNewlines(m.caseDetail.View())

		// If loading a fresh case list (not a page append), overlay spinner on list pane
		if m.loadingCases && m.initialLoadDone && !m.loadingPage {
			list = m.overlaySpinner(list, "Loading cases...")
		}

		// If loading detail, overlay spinner on detail pane
		if m.loadingDetail {
			detail = m.overlaySpinner(detail, "Loading case details...")
		}

		content = lipgloss.JoinVertical(lipgloss.Left, list, detail)
	}

	// Footer/Status bar
	footer := trimTrailingNewlines(m.statusBar.View())

	// Build view with optional filter bar
	var view string
	if filterBar != "" {
		view = lipgloss.JoinVertical(lipgloss.Left, header, filterBar, content, footer)
	} else {
		view = lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
	}

	// Quick search overlay
	if m.quickSearchMode && m.quickSearch.IsVisible() {
		view = overlayCenter(view, m.quickSearch.View(), m.width, m.height)
	}

	// Filter dialog overlay; record where it lands so mouse hit-testing and
	// dropdown placement match the rendered position
	if m.filterDialog.IsVisible() {
		dialog := m.filterDialog.View()
		dialogX := max((m.width-lipgloss.Width(dialog))/2, 0)
		dialogY := max((m.height-lipgloss.Height(dialog))/2, 0)
		m.filterDialog.SetPosition(dialogX, dialogY)
		view = overlayAt(view, dialog, dialogX, dialogY, m.height)

		// Product dropdown overlay (separate so it truly overlays content)
		if m.filterDialog.ShouldShowProductDropdown() {
			dropdownX, dropdownY := m.filterDialog.GetDropdownPosition()
			view = overlayAt(view, m.filterDialog.RenderProductDropdown(), dropdownX, dropdownY, m.height)
		}
	}

	// File picker overlay
	if m.filePicker.IsVisible() {
		view = overlayCenter(view, m.filePicker.View(), m.width, m.height)
	}

	// Progress modal overlay
	if m.modal.IsVisible() {
		view = overlayCenter(view, m.modal.View(), m.width, m.height)
	}

	// About box overlay
	if m.showAbout {
		view = overlayCenter(view, m.renderAbout(), m.width, m.height)
	}

	// Text search bar overlay (bottom of screen)
	if m.textSearchMode && m.textSearch.IsVisible() {
		view = overlayBottom(view, m.textSearch.View(), m.width, m.height)
	}

	// Loading cases overlay (show until first successful case load)
	if !m.initialLoadDone {
		view = overlayCenter(view, m.spinnerBox("Loading cases..."), m.width, m.height)
	}

	// Enforce exact height to prevent terminal scrolling
	lines := strings.Split(view, "\n")
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	// Pad if needed
	for len(lines) < m.height {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

// overlayCenter places a dialog box centered on top of a background
func overlayCenter(background, dialog string, width, height int) string {
	x := (width - lipgloss.Width(dialog)) / 2
	y := (height - lipgloss.Height(dialog)) / 2
	return overlayAt(background, dialog, x, y, height)
}

// overlayBottom places an overlay at the bottom of the screen
func overlayBottom(background, overlay string, width, height int) string {
	bgLines := strings.Split(background, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Ensure background has enough lines
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}

	overlayHeight := len(overlayLines)

	// Calculate bottom position (above status bar, so height - overlayHeight - 1)
	startY := height - overlayHeight - 1
	if startY < 0 {
		startY = 0
	}

	// Replace lines at the bottom with overlay
	for i, overlayLine := range overlayLines {
		bgY := startY + i
		if bgY >= len(bgLines) {
			break
		}
		bgLines[bgY] = overlayLine
	}

	return strings.Join(bgLines[:height], "\n")
}

// overlayAt places an overlay at a specific x,y position on the screen
func overlayAt(background, overlay string, x, y, height int) string {
	bgLines := strings.Split(background, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Ensure background has enough lines
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}

	// Clamp position
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	// Overlay each line
	for i, overlayLine := range overlayLines {
		bgY := y + i
		if bgY >= len(bgLines) || bgY >= height {
			break
		}

		bgLine := bgLines[bgY]
		bgLineWidth := lipgloss.Width(bgLine)

		// Pad background line if needed to reach overlay start position
		if bgLineWidth < x {
			bgLine += strings.Repeat(" ", x-bgLineWidth)
		}

		// Split background line at overlay position
		var before, after string
		if x > 0 && bgLineWidth > 0 {
			before = components.AnsiCut(bgLine, 0, x)
		}
		afterStart := x + lipgloss.Width(overlayLine)
		if bgLineWidth > afterStart {
			after = components.AnsiCut(bgLine, afterStart, bgLineWidth)
		}

		bgLines[bgY] = before + overlayLine + after
	}

	return strings.Join(bgLines[:height], "\n")
}

func trimTrailingNewlines(s string) string {
	return strings.TrimRight(s, "\n")
}

// savePreset stores the current filter in the given preset slot
func (m *Model) savePreset(slot string) {
	if m.activeFilter == nil && len(m.opts.Accounts) == 0 {
		m.statusBar.SetMessage(m.styles.Warning.Render("No filter active to save"), 2*time.Second)
		return
	}
	preset := m.buildPresetFromCurrent()
	m.configMgr.SetPreset(slot, preset)
	if err := m.configMgr.Save(); err != nil {
		m.statusBar.SetMessage(m.styles.Error.Render(fmt.Sprintf("Failed to save preset: %v", err)), 3*time.Second)
		return
	}
	name := preset.Name
	if name == "" {
		name = "Preset " + slot
	}
	m.activePreset = slot
	m.filterBar.SetPreset(slot, preset.Name)
	m.statusBar.SetMessage(m.styles.Success.Render(fmt.Sprintf("Saved to preset %s: %s", slot, name)), 2*time.Second)
}

// buildPresetFromCurrent creates a FilterPreset from the current filter state
func (m *Model) buildPresetFromCurrent() *config.FilterPreset {
	preset := &config.FilterPreset{}

	// Get accounts from active filter or defaults
	if m.activeFilter != nil && len(m.activeFilter.Accounts) > 0 {
		preset.Accounts = m.activeFilter.Accounts
	} else if len(m.opts.Accounts) > 0 {
		preset.Accounts = m.opts.Accounts
	}

	if m.activeFilter != nil {
		preset.Status = m.activeFilter.Status
		preset.Severity = m.activeFilter.Severity
		preset.Products = m.activeFilter.Products
		preset.Keyword = m.activeFilter.Keyword
	}

	// Generate a name based on content
	var parts []string
	if len(preset.Accounts) > 0 {
		if len(preset.Accounts) == 1 {
			parts = append(parts, "Acct:"+preset.Accounts[0])
		} else {
			parts = append(parts, fmt.Sprintf("%d Accts", len(preset.Accounts)))
		}
	}
	if len(preset.Status) > 0 && len(preset.Status) < 4 {
		parts = append(parts, fmt.Sprintf("%d Status", len(preset.Status)))
	}
	if len(preset.Severity) > 0 && len(preset.Severity) < 4 {
		parts = append(parts, fmt.Sprintf("Sev:%d", len(preset.Severity)))
	}
	if len(preset.Products) > 0 {
		if len(preset.Products) == 1 {
			parts = append(parts, runewidth.Truncate(preset.Products[0], 12, "…"))
		} else {
			parts = append(parts, fmt.Sprintf("%d Products", len(preset.Products)))
		}
	}
	if len(parts) > 0 {
		preset.Name = strings.Join(parts, ", ")
	}

	return preset
}

// presetToFilter converts a FilterPreset to a CaseFilter
func (m *Model) presetToFilter(preset *config.FilterPreset) *api.CaseFilter {
	return &api.CaseFilter{
		Accounts: preset.Accounts,
		Status:   preset.Status,
		Severity: preset.Severity,
		Products: preset.Products,
		Keyword:  preset.Keyword,
		Count:    100,
	}
}

// spinnerBox renders a bordered spinner with a message
func (m *Model) spinnerBox(text string) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Header.GetBackground()).
		Padding(1, 3).
		Render(m.spinner.View() + " " + text)
}

// overlaySpinner centers a spinner box over a rendered pane
func (m *Model) overlaySpinner(pane, text string) string {
	return lipgloss.Place(lipgloss.Width(pane), lipgloss.Height(pane),
		lipgloss.Center, lipgloss.Center, m.spinnerBox(text))
}

// renderAbout renders the About box overlay
func (m *Model) renderAbout() string {
	version := m.opts.Version
	if version == "" {
		version = "dev"
	}

	var sb strings.Builder
	sb.WriteString(m.styles.Title.Render("agcm " + version))
	sb.WriteString("\n")
	sb.WriteString(m.styles.Muted.Render("A TUI for the Red Hat Support Portal"))
	sb.WriteString("\n\n")
	sb.WriteString(m.styles.Label.Render("Author:   ") + m.styles.Value.Render(about.Author) + "\n")
	sb.WriteString(m.styles.Label.Render("License:  ") + m.styles.Value.Render(about.License) + "\n")
	sb.WriteString(m.styles.Label.Render("Homepage: ") + m.styles.Subtitle.Render(about.Homepage) + "\n\n")
	sb.WriteString(m.styles.Value.Render("Please report bugs and feature requests at:") + "\n")
	sb.WriteString("  " + m.styles.Subtitle.Render(about.Issues) + "\n\n")
	sb.WriteString(m.styles.Muted.Render("Press any key to close"))

	// Cap the box so it fits even at the minimum terminal width
	boxWidth := 52
	if boxWidth > m.width-4 {
		boxWidth = m.width - 4
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Header.GetBackground()).
		Padding(1, 3).
		Width(boxWidth).
		Render(sb.String())
}

// renderHelp renders the help screen from the KeyMap so it can't drift from
// the actual bindings; mouse actions are appended as extra rows.
func (m *Model) renderHelp() string {
	type entry struct{ key, desc string }
	var entries []entry
	for _, group := range m.keys.FullHelp() {
		for _, b := range group {
			h := b.Help()
			entries = append(entries, entry{h.Key, h.Desc})
		}
	}
	entries = append(entries,
		entry{"Right-click", "open case in browser"},
		entry{"Click link", "open URL"},
	)

	renderEntry := func(e entry) string {
		return m.styles.HelpKey.Render(fmt.Sprintf("%-12s", e.key)) + " " +
			m.styles.HelpDesc.Render(e.desc)
	}
	pad := func(s string, w int) string {
		if d := w - lipgloss.Width(s); d > 0 {
			return s + strings.Repeat(" ", d)
		}
		return s
	}

	// Two columns when the terminal is wide enough
	cols := 1
	if m.width >= 84 {
		cols = 2
	}
	rows := (len(entries) + cols - 1) / cols

	sb := m.styles.Title.Render("Keyboard Shortcuts")
	sb += "\n\n"
	for r := 0; r < rows; r++ {
		line := renderEntry(entries[r])
		if cols == 2 && r+rows < len(entries) {
			line = pad(line, 40) + renderEntry(entries[r+rows])
		}
		sb += line + "\n"
	}
	sb += "\n" + m.styles.Muted.Render("Press any key to close")

	return m.styles.Border.
		Width(m.width-4).
		Padding(1, 2).
		Render(sb)
}

// Run starts the TUI
func Run(client *api.Client, opts Options, configMgr *config.Manager) error {
	p := tea.NewProgram(
		NewModel(client, opts, configMgr),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := p.Run()
	return err
}
