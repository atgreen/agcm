// SPDX-License-Identifier: GPL-3.0-or-later
package tui

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/green/agcm/internal/api"
	"github.com/green/agcm/internal/export"
	"github.com/green/agcm/internal/tui/components"
	"github.com/green/agcm/internal/tui/styles"
)

var debugFile *os.File

func init() {
	debugFile, _ = os.OpenFile("/tmp/agcm-export-debug.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
}

func debugLog(format string, args ...interface{}) {
	if debugFile != nil {
		fmt.Fprintf(debugFile, time.Now().Format("15:04:05.000")+" "+format+"\n", args...)
		debugFile.Sync()
	}
}

var urlRegex = regexp.MustCompile(`https?://[^\s<>"{}|\\^` + "`" + `\[\]]+`)

// maskText replaces letters and digits with asterisks for privacy
func maskText(s string) string {
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result.WriteRune('*')
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

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
	AccountNumber string
	GroupNumber   string
	MaskMode      bool
	Version       string
}

// CachedCaseDetail holds cached case details
type CachedCaseDetail struct {
	Case        *api.Case
	Comments    []api.Comment
	Attachments []api.Attachment
}

// Model is the main TUI model
type Model struct {
	client *api.Client
	opts   Options
	styles *styles.Styles
	keys   *styles.KeyMap
	width  int
	height int
	ready  bool

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
	cases            []api.Case
	sortField        SortField
	sortReverse      bool
	err              error
	loadingCases     bool
	loadingDetail    bool
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
	totalCases   int // Total cases before filtering (for display)
	products     []string

	// Text search within case
	textSearch     *components.TextSearch
	textSearchMode bool
}

// Messages
type casesLoadedMsg struct {
	cases      []api.Case
	totalCount int
	startIndex int
	append     bool
	err        error
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

type statusMsg struct {
	message string
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
func NewModel(client *api.Client, opts Options) *Model {
	s := styles.DefaultStyles()
	keys := styles.DefaultKeyMap()

	sp := spinner.New()
	sp.Spinner = spinner.Dot

	caseList := components.NewCaseList(s, keys)
	caseList.SetMaskMode(opts.MaskMode)

	caseDetail := components.NewCaseDetail(s, keys)
	caseDetail.SetMaskMode(opts.MaskMode)

	return &Model{
		client:       client,
		opts:         opts,
		styles:       s,
		keys:         keys,
		caseList:     caseList,
		caseDetail:   caseDetail,
		statusBar:    components.NewStatusBar(s),
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
		detailCache:  make(map[string]*CachedCaseDetail),
	}
}

// Init implements tea.Model
func (m *Model) Init() tea.Cmd {
	m.loadingCases = true
	return tea.Batch(
		m.loadCasesPage(0, false),
		tea.EnterAltScreen,
		m.spinner.Tick,
	)
}

// loadCasesWithFilter loads cases using a custom filter
func (m *Model) loadCasesWithFilter(filter *api.CaseFilter) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		reqFilter := m.withDefaults(filter, 0, casePageSize)
		result, err := m.client.ListCases(ctx, reqFilter)
		if err != nil {
			return casesLoadedMsg{err: err}
		}
		return casesLoadedMsg{
			cases:      result.Items,
			totalCount: result.TotalCount,
			startIndex: result.StartIndex,
			append:     false,
		}
	}
}

// loadCasesPage loads a page of cases, optionally appending.
func (m *Model) loadCasesPage(start int, append bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		reqFilter := m.withDefaults(m.activeFilter, start, casePageSize)
		result, err := m.client.ListCases(ctx, reqFilter)
		if err != nil {
			return casesLoadedMsg{err: err}
		}
		return casesLoadedMsg{
			cases:      result.Items,
			totalCount: result.TotalCount,
			startIndex: result.StartIndex,
			append:     append,
		}
	}
}

func (m *Model) withDefaults(filter *api.CaseFilter, start, count int) *api.CaseFilter {
	req := &api.CaseFilter{
		Count:         count,
		StartIndex:    start,
		AccountNumber: m.opts.AccountNumber,
		GroupNumber:   m.opts.GroupNumber,
	}
	if filter == nil {
		return req
	}
	req.Status = append(req.Status, filter.Status...)
	req.Severity = append(req.Severity, filter.Severity...)
	req.Product = filter.Product
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

func (m *Model) fetchAllCaseNumbers(ctx context.Context) ([]string, error) {
	start := 0
	total := -1
	var caseNumbers []string

	for {
		reqFilter := m.withDefaults(m.activeFilter, start, casePageSize)
		result, err := m.client.ListCases(ctx, reqFilter)
		if err != nil {
			return nil, err
		}
		if total < 0 {
			total = result.TotalCount
		}
		for _, c := range result.Items {
			caseNumbers = append(caseNumbers, c.CaseNumber)
		}
		if len(result.Items) == 0 || len(caseNumbers) >= total {
			break
		}
		start += len(result.Items)
	}

	return caseNumbers, nil
}

func (m *Model) loadProducts() tea.Cmd {
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

	// Get current case
	c := m.caseList.SelectedCase()
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

// sortCases sorts the cases based on current sort settings
func (m *Model) sortCases() {
	sort.Slice(m.cases, func(i, j int) bool {
		var less bool
		switch m.sortField {
		case SortByLastModified:
			less = m.cases[i].LastModified.Before(m.cases[j].LastModified)
		case SortByCreated:
			less = m.cases[i].CreatedDate.Before(m.cases[j].CreatedDate)
		case SortBySeverity:
			less = m.cases[i].Severity < m.cases[j].Severity
		case SortByCaseNumber:
			less = m.cases[i].CaseNumber < m.cases[j].CaseNumber
		default:
			less = m.cases[i].LastModified.Before(m.cases[j].LastModified)
		}
		if m.sortReverse {
			return !less
		}
		return less
	})
	m.caseList.SetCases(m.cases)
	m.caseList.SetSort(components.SortField(m.sortField), m.sortReverse)
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
			debugLog("File picker closed. pendingExport=%s exportPath=%s", m.pendingExport, m.exportPath)
			if m.pendingExport != "" && m.exportPath != "" {
				debugLog("Starting export...")
				var exportCmd tea.Cmd
				if m.pendingExport == "single" {
					exportCmd = m.startSingleExport(m.exportCaseNumber, m.exportPath)
				} else if m.pendingExport == "bulk" {
					exportCmd = m.startBulkExport(m.exportPath)
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
		textSearch, cmd := m.textSearch.Update(msg)
		m.textSearch = textSearch
		if !m.textSearch.IsVisible() {
			m.textSearchMode = false
			m.caseDetail.ClearSearchHighlight()
		}
		return m, cmd
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
		m.modal.Hide()
		if msg.err != nil {
			m.statusBar.SetMessage(m.styles.Error.Render("Export failed: "+msg.err.Error()), 5*time.Second)
		} else {
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
		m.loadingCases = true
		m.detailCache = make(map[string]*CachedCaseDetail)
		m.totalCases = 0
		m.caseList.SetTotalCount(0)
		m.statusBar.SetMessage(m.styles.Muted.Render("Applying filter..."), 0)
		return m, tea.Batch(m.loadCasesWithFilter(msg.Filter), m.spinner.Tick)

	case components.FilterClearMsg:
		m.activeFilter = nil
		m.loadingCases = true
		m.detailCache = make(map[string]*CachedCaseDetail)
		m.totalCases = 0
		m.caseList.SetTotalCount(0)
		m.statusBar.SetMessage(m.styles.Muted.Render("Clearing filter..."), 0)
		return m, tea.Batch(m.loadCasesPage(0, false), m.spinner.Tick)

	case components.FilterCancelMsg:
		// Dialog closed without changes

	case productsLoadedMsg:
		if msg.err != nil {
			m.statusBar.SetMessage(m.styles.Warning.Render("Failed to load products"), 3*time.Second)
			m.filterDialog.SetProductsError("load failed")
		} else {
			m.products = msg.products
			m.filterDialog.SetProducts(m.products)
		}

	case components.TextSearchCloseMsg:
		m.textSearchMode = false
		m.caseDetail.ClearSearchHighlight()

	case components.TextSearchQueryMsg:
		// Search in case content and update matches
		if msg.Query != "" {
			matches := m.searchInCase(msg.Query)
			m.textSearch.SetMatches(matches)
			m.caseDetail.SetSearchHighlight(msg.Query)
		} else {
			m.textSearch.SetMatches(nil)
			m.caseDetail.ClearSearchHighlight()
		}

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

		if key.Matches(msg, m.keys.Help) {
			m.showHelp = !m.showHelp
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
			m.detailCache = make(map[string]*CachedCaseDetail) // Clear cache
			return m, tea.Batch(m.loadCasesPage(0, false), m.spinner.Tick)
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
		if msg.String() == "F" && m.activeFilter != nil {
			m.activeFilter = nil
			m.filterBar.Clear()
			m.loadingCases = true
			m.detailCache = make(map[string]*CachedCaseDetail)
			m.totalCases = 0
			m.caseList.SetTotalCount(0)
			m.statusBar.SetMessage(m.styles.Muted.Render("Filter cleared"), 2*time.Second)
			return m, tea.Batch(m.loadCasesPage(0, false), m.spinner.Tick)
		}

		// Sort controls
		if key.Matches(msg, m.keys.Sort) {
			m.cycleSortField()
			return m, nil
		}
		if msg.String() == "S" {
			m.toggleSortOrder()
			return m, nil
		}

		// Export current case (e)
		if key.Matches(msg, m.keys.Export) {
			debugLog("Export key pressed")
			if c := m.caseDetail.GetCase(); c != nil {
				debugLog("Case found: %s", c.CaseNumber)
				m.pendingExport = "single"
				m.exportCaseNumber = c.CaseNumber
				defaultName := fmt.Sprintf("case-%s.md", c.CaseNumber)
				cmd := m.filePicker.Show(
					"Export Case",
					fmt.Sprintf("Export case %s to markdown file", c.CaseNumber),
					components.FilePickerModeFile,
					defaultName,
					func(filename string) {
						debugLog("File picker callback: filename=%s", filename)
						m.exportPath = filename
					},
					func() {
						debugLog("File picker cancelled")
						m.pendingExport = ""
					},
				)
				return m, cmd
			} else {
				debugLog("No case selected")
				m.statusBar.SetMessage(m.styles.Warning.Render("No case selected"), 2*time.Second)
			}
			return m, nil
		}

		// Export all cases (E)
		if msg.String() == "E" {
			if len(m.cases) > 0 {
				m.pendingExport = "bulk"
				cmd := m.filePicker.Show(
					"Export All Cases",
					fmt.Sprintf("Select directory for %d cases", len(m.cases)),
					components.FilePickerModeDir,
					"./exports",
					func(dir string) {
						m.exportPath = dir
					},
					func() {
						m.pendingExport = ""
					},
				)
				return m, cmd
			} else {
				m.statusBar.SetMessage(m.styles.Warning.Render("No cases loaded"), 2*time.Second)
			}
			return m, nil
		}

		// Global left/right for tab switching in detail pane
		if key.Matches(msg, m.keys.Left) || key.Matches(msg, m.keys.Right) {
			caseDetail, cmd := m.caseDetail.Update(msg)
			m.caseDetail = caseDetail
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}
		// Global next/previous comment shortcuts (only in Comments tab)
		if (msg.String() == "n" || msg.String() == "p") && m.caseDetail.ActiveTab() == 1 {
			caseDetail, cmd := m.caseDetail.Update(msg)
			m.caseDetail = caseDetail
			cmds = append(cmds, cmd)
			return m, tea.Batch(cmds...)
		}

		// Pass to focused component
		switch m.currentPane {
		case PaneList:
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
			if msg.String() == "ctrl+f" {
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
		m.statusBar.SetConnected(true)
		selectedCase := ""
		savedOffset := m.caseList.GetOffset()
		if sel := m.caseList.SelectedCase(); sel != nil {
			selectedCase = sel.CaseNumber
		}
		if msg.err != nil {
			m.err = msg.err
			m.statusBar.SetMessage(m.styles.Error.Render("Error: "+msg.err.Error()), 5*time.Second)
		} else {
			if msg.append {
				m.cases = append(m.cases, msg.cases...)
			} else {
				m.cases = msg.cases
			}
			if msg.totalCount > 0 {
				m.totalCases = msg.totalCount
			} else if !msg.append {
				m.totalCases = len(m.cases)
			}
			m.sortCases()
			if msg.append {
				if selectedCase != "" {
					for i, c := range m.cases {
						if c.CaseNumber == selectedCase {
							m.caseList.SetOffset(savedOffset)
							m.caseList.SetCursor(i)
							break
						}
					}
				} else {
					m.caseList.SetOffset(savedOffset)
				}
			}
			m.caseList.SetTotalCount(m.totalCases)
			// Update filter bar
			if m.activeFilter != nil {
				m.filterBar.SetFilter(m.activeFilter, len(m.cases), m.totalCases)
			} else {
				m.filterBar.Clear()
			}
			// Trigger initial highlight check
			cmds = append(cmds, m.checkHighlightChange())
		}

	case caseDetailLoadedMsg:
		m.loadingDetail = false
		if msg.err != nil {
			m.err = msg.err
			m.statusBar.SetMessage(m.styles.Error.Render("Error: "+msg.err.Error()), 5*time.Second)
		} else {
			// Cache the result
			m.detailCache[msg.caseNumber] = &CachedCaseDetail{
				Case:        msg.case_,
				Comments:    msg.comments,
				Attachments: msg.attachments,
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
		m.err = msg.err
		m.statusBar.SetMessage(m.styles.Error.Render("Error: "+msg.err.Error()), 5*time.Second)

	case statusMsg:
		m.statusBar.SetMessage(msg.message, 3*time.Second)
	}

	// Update status bar
	m.statusBar.SetLoading(m.loadingCases, "Loading cases...")
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

func (m *Model) startSingleExport(caseNumber, filename string) tea.Cmd {
	m.exporting = true
	m.modal.ShowProgress("Exporting Case", "Preparing export...")

	return func() tea.Msg {
		// Debug: log export attempt
		debugLog("startSingleExport: caseNumber=%s filename=%s", caseNumber, filename)

		opts := export.DefaultOptions()
		opts.OutputFile = filename

		exporter, err := export.NewExporter(m.client, opts)
		if err != nil {
			debugLog("startSingleExport: NewExporter error: %v", err)
			return exportCompleteMsg{err: err}
		}

		ctx := context.Background()
		err = exporter.ExportCaseToFile(ctx, caseNumber, filename)
		if err != nil {
			debugLog("startSingleExport: ExportCaseToFile error: %v", err)
			return exportCompleteMsg{err: err}
		}

		absPath, _ := filepath.Abs(filename)
		debugLog("startSingleExport: success, outputPath=%s", absPath)
		return exportCompleteMsg{outputPath: absPath}
	}
}

func (m *Model) startBulkExport(outputDir string) tea.Cmd {
	m.exporting = true
	ctx, cancel := context.WithCancel(context.Background())
	m.exportCancel = cancel
	m.modal.ShowProgress("Exporting Cases", "Starting export...")
	progressCh := make(chan export.Progress, 10)
	m.exportProgressCh = progressCh
	exportCmd := func() tea.Msg {
		opts := export.DefaultOptions()
		opts.OutputDir = outputDir

		exporter, err := export.NewExporter(m.client, opts)
		if err != nil {
			return exportCompleteMsg{err: err}
		}

		caseNumbers, err := m.fetchAllCaseNumbers(ctx)
		if err != nil {
			return exportCompleteMsg{err: err}
		}

		_, err = exporter.ExportCases(ctx, caseNumbers, progressCh)
		close(progressCh)

		absPath, _ := filepath.Abs(outputDir)
		return exportCompleteMsg{outputPath: absPath, err: err}
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
		debugLog("layout: term=%dx%d content=%d list=%d detail=%d header=%d footer=%d filter=%d total=%d",
			m.width, m.height, contentHeight, listHeight, detailHeight, headerHeight, footerHeight, filterBarHeight, totalUsed)
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
			// Continue dragging list scrollbar thumb
			if msg.Y > listTop && msg.Y < m.detailY {
				relY := msg.Y - (listTop + 1)
				m.caseList.ScrollToRelativeLine(relY)
				if cmd := m.maybeLoadMoreCases(); cmd != nil {
					return cmd
				}
			}
			return nil
		}
		if msg.Action != tea.MouseActionPress {
			return nil
		}

		// Click on column headers in case list
		if msg.Y == listHeaderY {
			// Column positions: CASE(0-10), MODIFIED(11-23), SEV(24-28), STATUS(29-49), SUMMARY(50+)
			x := msg.X - 1 // Account for border
			if x >= 0 && x < 10 {
				// CASE column - sort by case number
				if m.sortField == SortByCaseNumber {
					m.toggleSortOrder()
				} else {
					m.sortField = SortByCaseNumber
					m.sortCases()
				}
			} else if x >= 11 && x < 24 {
				// MODIFIED column - toggle between LastModified and Created
				if m.sortField == SortByLastModified {
					m.sortField = SortByCreated
					m.sortCases()
				} else if m.sortField == SortByCreated {
					m.toggleSortOrder()
				} else {
					m.sortField = SortByLastModified
					m.sortCases()
				}
			} else if x >= 24 && x < 29 {
				// SEV column - sort by severity
				if m.sortField == SortBySeverity {
					m.toggleSortOrder()
				} else {
					m.sortField = SortBySeverity
					m.sortCases()
				}
			}
			return nil
		}

		// Click on list scrollbar area (right-most 2 columns inside border)
		if msg.Y > listTop && msg.Y < m.detailY && msg.X >= m.width-3 && msg.X <= m.width-2 {
			m.listScrollDrag = true
			relY := msg.Y - (listTop + 1)
			m.caseList.ScrollToRelativeLine(relY)
			return m.maybeLoadMoreCases()
		}

		// Click in case list data area
		if msg.Y > listHeaderY+1 && msg.Y < m.detailY {
			m.currentPane = PaneList
			m.updateFocus()

			// Calculate which row was clicked (header=1, border=1, colheader=1, separator=1)
			rowOffset := msg.Y - headerHeight - 3
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
			rowOffset := msg.Y - headerHeight - 3
			if rowOffset >= 0 {
				clickedIdx := m.caseList.GetOffset() + rowOffset
				if clickedIdx >= 0 && clickedIdx < len(m.cases) {
					caseNumber := m.cases[clickedIdx].CaseNumber
					url := fmt.Sprintf("https://access.redhat.com/support/cases/#/case/%s", caseNumber)
					return openURL(url)
				}
			}
		}
	}

	return nil
}

func openURL(url string) tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("xdg-open", url)
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

// View implements tea.Model
func (m *Model) View() string {
	if !m.ready {
		return "Loading..."
	}

	// Header with version and sort info (and debug if enabled)
	versionText := ""
	if m.opts.Version != "" {
		versionText = " " + m.opts.Version
	}
	sortInfo := fmt.Sprintf(" [Sort: %s]", m.sortField.String())
	headerText := "agcm" + m.styles.Muted.Render(versionText) + m.styles.Muted.Render(sortInfo)
	if m.layoutDebug != "" {
		headerText += m.styles.Muted.Render(m.layoutDebug)
	}
	header := m.styles.Header.
		Width(m.width).
		Render(headerText)
	header = trimTrailingNewlines(header)
	headerLines := countLines(header)

	// Filter bar (conditional)
	filterBar := ""
	if m.filterBar.HasActiveFilter() {
		filterBar = trimTrailingNewlines(m.filterBar.View())
	}
	filterBarLines := countLines(filterBar)

	// Main content area
	var content string

	if m.showHelp {
		content = trimTrailingNewlines(m.renderHelp())
	} else {
		list := trimTrailingNewlines(m.caseList.View())
		detail := trimTrailingNewlines(m.caseDetail.View())
		if os.Getenv("AGCM_DEBUG_LAYOUT") != "" {
			debugLog("component lines: list=%d detail=%d", countLines(list), countLines(detail))
		}

		// If loading detail, overlay spinner on detail pane
		if m.loadingDetail {
			detail = m.renderDetailWithSpinner(detail)
		}

		content = lipgloss.JoinVertical(lipgloss.Left, list, detail)
	}
	contentLines := countLines(content)

	// Footer/Status bar
	footer := trimTrailingNewlines(m.statusBar.View())
	footerLines := countLines(footer)

	// Build view with optional filter bar
	var view string
	if filterBar != "" {
		view = lipgloss.JoinVertical(lipgloss.Left, header, filterBar, content, footer)
	} else {
		view = lipgloss.JoinVertical(lipgloss.Left, header, content, footer)
	}
	if os.Getenv("AGCM_DEBUG_LAYOUT") != "" {
		debugLog("component totals: header=%d filter=%d content=%d footer=%d sum=%d",
			headerLines, filterBarLines, contentLines, footerLines,
			headerLines+filterBarLines+contentLines+footerLines)
	}

	// Quick search overlay
	if m.quickSearchMode && m.quickSearch.IsVisible() {
		view = overlayCenter(view, m.quickSearch.View(), m.width, m.height)
	}

	// Filter dialog overlay
	if m.filterDialog.IsVisible() {
		view = overlayCenter(view, m.filterDialog.View(), m.width, m.height)
	}

	// File picker overlay
	if m.filePicker.IsVisible() {
		view = overlayCenter(view, m.filePicker.View(), m.width, m.height)
	}

	// Progress modal overlay
	if m.modal.IsVisible() {
		view = overlayCenter(view, m.modal.View(), m.width, m.height)
	}

	// Enforce exact height to prevent terminal scrolling
	lines := strings.Split(view, "\n")
	if os.Getenv("AGCM_DEBUG_LAYOUT") != "" && len(lines) != m.height {
		debugLog("render: lines=%d height=%d width=%d", len(lines), m.height, m.width)
	}
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
	bgLines := strings.Split(background, "\n")
	dialogLines := strings.Split(dialog, "\n")

	// Ensure background has enough lines
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}

	dialogHeight := len(dialogLines)
	dialogWidth := lipgloss.Width(dialog)

	// Calculate center position
	startY := (height - dialogHeight) / 2
	startX := (width - dialogWidth) / 2

	if startY < 0 {
		startY = 0
	}
	if startX < 0 {
		startX = 0
	}

	// Overlay dialog onto background
	for i, dialogLine := range dialogLines {
		bgY := startY + i
		if bgY >= len(bgLines) {
			break
		}

		bgLine := bgLines[bgY]
		// Pad background line if needed
		bgLineWidth := lipgloss.Width(bgLine)
		if bgLineWidth < startX {
			bgLine += strings.Repeat(" ", startX-bgLineWidth)
		}

		// Split background line at overlay position
		var before, after string
		if startX > 0 && bgLineWidth > 0 {
			before = ansiCut(bgLine, 0, startX)
		}
		afterStart := startX + lipgloss.Width(dialogLine)
		if bgLineWidth > afterStart {
			after = ansiCut(bgLine, afterStart, bgLineWidth)
		}

		bgLines[bgY] = before + dialogLine + after
	}

	return strings.Join(bgLines[:height], "\n")
}

func trimTrailingNewlines(s string) string {
	return strings.TrimRight(s, "\n")
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// ansiCut cuts a string with ANSI codes at the given visual positions
func ansiCut(s string, start, end int) string {
	// Simple implementation - for complex ANSI, use lipgloss or ansi package
	result := ""
	visualPos := 0
	inEscape := false

	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			if visualPos >= start && visualPos < end {
				result += string(r)
			}
			continue
		}
		if inEscape {
			if visualPos >= start && visualPos < end {
				result += string(r)
			}
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}

		if visualPos >= start && visualPos < end {
			result += string(r)
		}
		visualPos++
		if visualPos >= end {
			break
		}
	}
	return result
}

// renderDetailWithSpinner overlays a spinner box on the detail pane
func (m *Model) renderDetailWithSpinner(detail string) string {
	spinnerText := m.spinner.View() + " Loading case details..."

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(m.styles.Header.GetBackground()).
		Padding(1, 3)

	spinnerBox := boxStyle.Render(spinnerText)

	detailWidth := lipgloss.Width(detail)
	detailHeight := lipgloss.Height(detail)

	return lipgloss.Place(
		detailWidth,
		detailHeight,
		lipgloss.Center,
		lipgloss.Center,
		spinnerBox,
	)
}

func (m *Model) renderHelp() string {
	var sb string

	help := []struct {
		key  string
		desc string
	}{
		{"↑/k, ↓/j", "Navigate up/down"},
		{"←/→", "Switch detail tabs"},
		{"gg, G", "Go to top/bottom"},
		{"PgUp/PgDn", "Page up/down"},
		{"Tab", "Switch between list/detail"},
		{"Esc", "Back to list"},
		{"/", "Quick search by case number"},
		{"f", "Filter dialog"},
		{"F", "Clear filter"},
		{"ctrl+f", "Search within case"},
		{"n, p", "Next/prev comment (Comments tab)"},
		{"s", "Cycle sort field"},
		{"S", "Toggle sort order"},
		{"r", "Refresh"},
		{"e", "Export current case"},
		{"E", "Export all cases"},
		{"Right-click", "Open case in browser"},
		{"Click link", "Open URL"},
		{"?", "Toggle help"},
		{"q", "Quit"},
	}

	sb = m.styles.Title.Render("Keyboard Shortcuts")
	sb += "\n\n"

	for _, h := range help {
		sb += fmt.Sprintf("%s  %s\n",
			m.styles.HelpKey.Render(fmt.Sprintf("%-15s", h.key)),
			m.styles.HelpDesc.Render(h.desc))
	}

	sb += "\n" + m.styles.Muted.Render("Press ? to close")

	return m.styles.Border.
		Width(m.width-4).
		Padding(1, 2).
		Render(sb)
}

// Run starts the TUI
func Run(client *api.Client, opts Options) error {
	p := tea.NewProgram(
		NewModel(client, opts),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	_, err := p.Run()
	return err
}
