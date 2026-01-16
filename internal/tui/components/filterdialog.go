// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package components

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/green/agcm/internal/api"
	"github.com/green/agcm/internal/tui/styles"
)

// FilterApplyMsg is sent when filter is applied
type FilterApplyMsg struct {
	Filter *api.CaseFilter
}

// FilterClearMsg is sent when filters are cleared
type FilterClearMsg struct{}

// FilterCancelMsg is sent when filter dialog is cancelled
type FilterCancelMsg struct{}

// Field indices for navigation
const (
	fieldAccounts = iota
	fieldStatusOpen
	fieldStatusWaitingRH
	fieldStatusWaitingCust
	fieldStatusClosed
	fieldSev1
	fieldSev2
	fieldSev3
	fieldSev4
	fieldProduct
	fieldKeyword
	fieldApply
	fieldClear
	fieldCancel
	fieldCount
)

// FilterDialog is a multi-field filter dialog
type FilterDialog struct {
	styles *styles.Styles
	width  int
	height int

	// Form fields
	accountsInput    textinput.Model
	productInput     textinput.Model
	keywordInput     textinput.Model
	products         []string
	productMatches   []string
	productCursor    int
	productsLoading  bool
	productsError    string
	selectedProducts []string // Tag-based multi-product selection

	// Checkbox states
	statusOpen        bool
	statusWaitingRH   bool
	statusWaitingCust bool
	statusClosed      bool
	sev1              bool
	sev2              bool
	sev3              bool
	sev4              bool

	// Navigation
	focusedField int
	visible      bool

	// Position tracking for mouse support
	dialogX int // Left edge of dialog on screen
	dialogY int // Top edge of dialog on screen
}

// NewFilterDialog creates a new filter dialog
func NewFilterDialog(s *styles.Styles) *FilterDialog {
	accountsInput := textinput.New()
	accountsInput.Placeholder = "account number(s), comma-separated"
	accountsInput.CharLimit = 100
	accountsInput.Width = 40
	accountsInput.Prompt = ""

	productInput := textinput.New()
	productInput.Placeholder = "type to search products"
	productInput.CharLimit = 50
	productInput.Width = 40
	productInput.Prompt = ""

	keywordInput := textinput.New()
	keywordInput.Placeholder = "enter search keywords"
	keywordInput.CharLimit = 100
	keywordInput.Width = 40
	keywordInput.Prompt = ""

	return &FilterDialog{
		styles:            s,
		accountsInput:     accountsInput,
		productInput:      productInput,
		keywordInput:      keywordInput,
		statusOpen:        true, // Default to showing open cases
		statusWaitingRH:   true,
		statusWaitingCust: true,
		statusClosed:      false, // Closed off by default
		sev1:              true,
		sev2:              true,
		sev3:              true,
		sev4:              true,
	}
}

// Show displays the filter dialog
func (f *FilterDialog) Show() tea.Cmd {
	f.visible = true
	f.focusedField = fieldAccounts
	f.accountsInput.Focus()
	return nil
}

// ShowWithFilter displays the dialog with existing filter values
func (f *FilterDialog) ShowWithFilter(filter *api.CaseFilter) tea.Cmd {
	f.visible = true
	f.focusedField = fieldAccounts
	f.accountsInput.Focus()

	if filter != nil {
		// Accounts
		if len(filter.Accounts) > 0 {
			f.accountsInput.SetValue(strings.Join(filter.Accounts, ", "))
		}
		// Products (tag-based selection)
		f.selectedProducts = nil
		if len(filter.Products) > 0 {
			f.selectedProducts = append(f.selectedProducts, filter.Products...)
		}
		f.productInput.SetValue("")
		f.keywordInput.SetValue(filter.Keyword)

		// Parse status filter
		if len(filter.Status) > 0 {
			f.statusOpen = false
			f.statusWaitingRH = false
			f.statusWaitingCust = false
			f.statusClosed = false
			for _, s := range filter.Status {
				switch s {
				case "Open":
					f.statusOpen = true
				case "Waiting on Red Hat":
					f.statusWaitingRH = true
				case "Waiting on Customer":
					f.statusWaitingCust = true
				case "Closed":
					f.statusClosed = true
				}
			}
		}

		// Parse severity filter
		if len(filter.Severity) > 0 {
			f.sev1 = false
			f.sev2 = false
			f.sev3 = false
			f.sev4 = false
			for _, s := range filter.Severity {
				switch s {
				case "1 (Urgent)", "1":
					f.sev1 = true
				case "2 (High)", "2":
					f.sev2 = true
				case "3 (Normal)", "3":
					f.sev3 = true
				case "4 (Low)", "4":
					f.sev4 = true
				}
			}
		}
	}

	return nil
}

// SetProducts sets the available product list.
func (f *FilterDialog) SetProducts(products []string) {
	f.products = products
	f.productsLoading = false
	f.productsError = ""
	f.updateProductMatches()
}

// SetProductsLoading flags the product list as loading.
func (f *FilterDialog) SetProductsLoading() {
	f.productsLoading = true
	f.productsError = ""
}

// SetProductsError records a load error to show in the UI.
func (f *FilterDialog) SetProductsError(msg string) {
	f.productsLoading = false
	f.productsError = msg
}

// Hide hides the dialog
func (f *FilterDialog) Hide() {
	f.visible = false
	f.accountsInput.Blur()
	f.productInput.Blur()
	f.keywordInput.Blur()
}

// IsVisible returns whether the dialog is visible
func (f *FilterDialog) IsVisible() bool {
	return f.visible
}

// SetSize sets the screen size and calculates dialog position
func (f *FilterDialog) SetSize(width, height int) {
	f.width = width
	f.height = height

	// Calculate dialog position (centered, with space for dropdown on right)
	dialogWidth := 60 + 42 // main dialog + dropdown space
	dialogHeight := 28     // approximate dialog height
	f.dialogX = (width - dialogWidth) / 2
	f.dialogY = (height - dialogHeight) / 2
	if f.dialogX < 0 {
		f.dialogX = 0
	}
	if f.dialogY < 0 {
		f.dialogY = 0
	}
}

// buildFilter creates a CaseFilter from current dialog state
func (f *FilterDialog) buildFilter() *api.CaseFilter {
	filter := &api.CaseFilter{
		Count: 100,
	}

	// Accounts
	if accts := strings.TrimSpace(f.accountsInput.Value()); accts != "" {
		accounts := strings.Split(accts, ",")
		for i := range accounts {
			accounts[i] = strings.TrimSpace(accounts[i])
		}
		// Filter out empty strings
		var validAccounts []string
		for _, a := range accounts {
			if a != "" {
				validAccounts = append(validAccounts, a)
			}
		}
		filter.Accounts = validAccounts
	}

	// Products (from tag-based selection)
	if len(f.selectedProducts) > 0 {
		filter.Products = append(filter.Products, f.selectedProducts...)
	}

	// Keyword
	if kw := strings.TrimSpace(f.keywordInput.Value()); kw != "" {
		filter.Keyword = kw
	}

	// Status - only add if not all selected
	if !(f.statusOpen && f.statusWaitingRH && f.statusWaitingCust && f.statusClosed) {
		var statuses []string
		if f.statusOpen {
			statuses = append(statuses, "Open")
		}
		if f.statusWaitingRH {
			statuses = append(statuses, "Waiting on Red Hat")
		}
		if f.statusWaitingCust {
			statuses = append(statuses, "Waiting on Customer")
		}
		if f.statusClosed {
			statuses = append(statuses, "Closed")
		}
		filter.Status = statuses
	}

	// Severity - only add if not all selected
	if !(f.sev1 && f.sev2 && f.sev3 && f.sev4) {
		var severities []string
		if f.sev1 {
			severities = append(severities, "1 (Urgent)")
		}
		if f.sev2 {
			severities = append(severities, "2 (High)")
		}
		if f.sev3 {
			severities = append(severities, "3 (Normal)")
		}
		if f.sev4 {
			severities = append(severities, "4 (Low)")
		}
		filter.Severity = severities
	}

	// Set IncludeClosed based on whether Closed status is selected
	filter.IncludeClosed = f.statusClosed

	return filter
}

// clearFilters resets all fields to defaults
func (f *FilterDialog) clearFilters() {
	f.accountsInput.SetValue("")
	f.productInput.SetValue("")
	f.selectedProducts = nil
	f.keywordInput.SetValue("")
	f.statusOpen = true
	f.statusWaitingRH = true
	f.statusWaitingCust = true
	f.statusClosed = false
	f.sev1 = true
	f.sev2 = true
	f.sev3 = true
	f.sev4 = true
	f.updateProductMatches()
}

func (f *FilterDialog) focusField(field int) {
	f.accountsInput.Blur()
	f.productInput.Blur()
	f.keywordInput.Blur()

	switch field {
	case fieldAccounts:
		f.accountsInput.Focus()
	case fieldProduct:
		f.productInput.Focus()
		f.updateProductMatches()
	case fieldKeyword:
		f.keywordInput.Focus()
	}
}

func (f *FilterDialog) toggleCurrentCheckbox() {
	switch f.focusedField {
	case fieldStatusOpen:
		f.statusOpen = !f.statusOpen
	case fieldStatusWaitingRH:
		f.statusWaitingRH = !f.statusWaitingRH
	case fieldStatusWaitingCust:
		f.statusWaitingCust = !f.statusWaitingCust
	case fieldStatusClosed:
		f.statusClosed = !f.statusClosed
	case fieldSev1:
		f.sev1 = !f.sev1
	case fieldSev2:
		f.sev2 = !f.sev2
	case fieldSev3:
		f.sev3 = !f.sev3
	case fieldSev4:
		f.sev4 = !f.sev4
	}
}

func (f *FilterDialog) updateProductMatches() {
	query := strings.ToLower(strings.TrimSpace(f.productInput.Value()))
	f.productMatches = f.productMatches[:0]
	if len(f.products) == 0 {
		f.productCursor = 0
		return
	}

	// Build set of already-selected products for fast lookup
	selected := make(map[string]bool)
	for _, sp := range f.selectedProducts {
		selected[sp] = true
	}

	// Filter products: match query and exclude already-selected
	for _, p := range f.products {
		if selected[p] {
			continue // Skip already selected
		}
		if query == "" || strings.Contains(strings.ToLower(p), query) {
			f.productMatches = append(f.productMatches, p)
		}
	}

	if f.productCursor >= len(f.productMatches) {
		f.productCursor = 0
	}
	if f.productCursor < 0 {
		f.productCursor = 0
	}
}

// Update handles input
func (f *FilterDialog) Update(msg tea.Msg) (*FilterDialog, tea.Cmd) {
	if !f.visible {
		return f, nil
	}

	switch msg := msg.(type) {
	case tea.MouseMsg:
		if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
			return f, nil
		}
		return f.handleMouseClick(msg.X, msg.Y)

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			f.Hide()
			return f, func() tea.Msg { return FilterCancelMsg{} }

		case "down":
			if f.focusedField == fieldProduct && len(f.productMatches) > 0 {
				f.productCursor++
				if f.productCursor >= len(f.productMatches) {
					f.productCursor = len(f.productMatches) - 1
				}
				return f, nil
			}
			f.focusedField = (f.focusedField + 1) % fieldCount
			f.focusField(f.focusedField)
			return f, nil

		case "tab":
			f.focusedField = (f.focusedField + 1) % fieldCount
			f.focusField(f.focusedField)
			return f, nil

		case "up":
			if f.focusedField == fieldProduct && len(f.productMatches) > 0 {
				f.productCursor--
				if f.productCursor < 0 {
					f.productCursor = 0
				}
				return f, nil
			}
			f.focusedField = (f.focusedField - 1 + fieldCount) % fieldCount
			f.focusField(f.focusedField)
			return f, nil

		case "shift+tab":
			f.focusedField = (f.focusedField - 1 + fieldCount) % fieldCount
			f.focusField(f.focusedField)
			return f, nil

		case " ":
			// Space toggles checkboxes
			if f.focusedField >= fieldStatusOpen && f.focusedField <= fieldSev4 {
				f.toggleCurrentCheckbox()
				return f, nil
			}

		case "backspace":
			// Remove last selected product when backspace pressed on empty product input
			if f.focusedField == fieldProduct && f.productInput.Value() == "" && len(f.selectedProducts) > 0 {
				f.selectedProducts = f.selectedProducts[:len(f.selectedProducts)-1]
				f.updateProductMatches()
				return f, nil
			}

		case "enter":
			// Add selected product to tags (instead of replacing input value)
			if f.focusedField == fieldProduct && len(f.productMatches) > 0 {
				if f.productCursor >= 0 && f.productCursor < len(f.productMatches) {
					f.selectedProducts = append(f.selectedProducts, f.productMatches[f.productCursor])
					f.productInput.SetValue("") // Clear input for next search
					f.updateProductMatches()
				}
				return f, nil // Stay on product field to allow adding more
			}
			switch f.focusedField {
			case fieldApply:
				filter := f.buildFilter()
				f.Hide()
				return f, func() tea.Msg { return FilterApplyMsg{Filter: filter} }
			case fieldClear:
				f.clearFilters()
				return f, func() tea.Msg { return FilterClearMsg{} }
			case fieldCancel:
				f.Hide()
				return f, func() tea.Msg { return FilterCancelMsg{} }
			default:
				// Move to next field on Enter for text inputs
				f.focusedField = (f.focusedField + 1) % fieldCount
				f.focusField(f.focusedField)
				return f, nil
			}
		}

		// Update focused text input
		var cmd tea.Cmd
		switch f.focusedField {
		case fieldAccounts:
			f.accountsInput, cmd = f.accountsInput.Update(msg)
		case fieldProduct:
			f.productInput, cmd = f.productInput.Update(msg)
			f.updateProductMatches()
		case fieldKeyword:
			f.keywordInput, cmd = f.keywordInput.Update(msg)
		}
		return f, cmd
	}

	return f, nil
}

// handleMouseClick processes mouse clicks within the dialog
func (f *FilterDialog) handleMouseClick(x, y int) (*FilterDialog, tea.Cmd) {
	// Calculate relative position within main dialog
	// Dialog has border (1) + padding (1) = 2 offset on each side
	relX := x - f.dialogX - 3 // 1 border + 2 padding
	relY := y - f.dialogY - 2 // 1 border + 1 padding

	// Check if click is in the product dropdown (to the right of main dialog)
	dropdownX := f.dialogX + 62 // main dialog width (60) + gap (2)
	if f.focusedField == fieldProduct && x >= dropdownX && x < dropdownX+40 {
		// Click in dropdown area
		dropdownRelY := y - f.dialogY - 4 // account for dropdown title and padding
		if dropdownRelY >= 0 && dropdownRelY < len(f.productMatches) {
			// Adjust for scrolling
			maxItems := 10
			start := 0
			if f.productCursor >= maxItems {
				start = f.productCursor - maxItems + 1
			}
			clickedIndex := start + dropdownRelY
			if clickedIndex >= 0 && clickedIndex < len(f.productMatches) {
				// Select this product
				f.selectedProducts = append(f.selectedProducts, f.productMatches[clickedIndex])
				f.productInput.SetValue("")
				f.updateProductMatches()
				return f, nil
			}
		}
		return f, nil
	}

	// Ignore clicks outside main dialog content area
	if relX < 0 || relX > 56 || relY < 0 {
		return f, nil
	}

	// Map Y position to fields (approximate line numbers in content)
	// Line 0: Title
	// Line 2: Accounts
	// Line 4-5: Status label + checkboxes start
	// Line 5: Open
	// Line 6: Waiting on Red Hat
	// Line 7: Waiting on Customer
	// Line 8: Closed
	// Line 10: Severity label
	// Line 11: Severity checkboxes (1,2,3,4)
	// Line 13+: Products tags (if any)
	// Line 14 or 15: Add product input
	// Line 16+: Keyword
	// Line 19+: Buttons

	hasProductTags := len(f.selectedProducts) > 0
	productAddLine := 13
	keywordLine := 15
	buttonLine := 20
	if hasProductTags {
		productAddLine = 14
		keywordLine = 16
		buttonLine = 21
	}

	switch {
	case relY == 2:
		// Accounts field
		f.focusedField = fieldAccounts
		f.focusField(fieldAccounts)

	case relY == 5:
		// Status: Open
		f.focusedField = fieldStatusOpen
		f.focusField(fieldStatusOpen)
		f.statusOpen = !f.statusOpen

	case relY == 6:
		// Status: Waiting on Red Hat
		f.focusedField = fieldStatusWaitingRH
		f.focusField(fieldStatusWaitingRH)
		f.statusWaitingRH = !f.statusWaitingRH

	case relY == 7:
		// Status: Waiting on Customer
		f.focusedField = fieldStatusWaitingCust
		f.focusField(fieldStatusWaitingCust)
		f.statusWaitingCust = !f.statusWaitingCust

	case relY == 8:
		// Status: Closed
		f.focusedField = fieldStatusClosed
		f.focusField(fieldStatusClosed)
		f.statusClosed = !f.statusClosed

	case relY == 11:
		// Severity checkboxes - determine which one based on X
		// Layout: "    [x] 1  [x] 2  [x] 3  [x] 4"
		// Approximate X positions: 1@4-10, 2@12-18, 3@20-26, 4@28-34
		if relX >= 4 && relX < 12 {
			f.focusedField = fieldSev1
			f.focusField(fieldSev1)
			f.sev1 = !f.sev1
		} else if relX >= 12 && relX < 20 {
			f.focusedField = fieldSev2
			f.focusField(fieldSev2)
			f.sev2 = !f.sev2
		} else if relX >= 20 && relX < 28 {
			f.focusedField = fieldSev3
			f.focusField(fieldSev3)
			f.sev3 = !f.sev3
		} else if relX >= 28 && relX < 40 {
			f.focusedField = fieldSev4
			f.focusField(fieldSev4)
			f.sev4 = !f.sev4
		}

	case relY == productAddLine || relY == productAddLine-1:
		// Product add field
		f.focusedField = fieldProduct
		f.focusField(fieldProduct)

	case relY == keywordLine || relY == keywordLine+1:
		// Keyword field
		f.focusedField = fieldKeyword
		f.focusField(fieldKeyword)

	case relY >= buttonLine && relY <= buttonLine+1:
		// Buttons row - determine which button based on X
		// Layout: "  [ Apply ]  [ Clear ]  [ Cancel ]"
		if relX >= 2 && relX < 14 {
			// Apply button
			filter := f.buildFilter()
			f.Hide()
			return f, func() tea.Msg { return FilterApplyMsg{Filter: filter} }
		} else if relX >= 16 && relX < 28 {
			// Clear button
			f.clearFilters()
			return f, func() tea.Msg { return FilterClearMsg{} }
		} else if relX >= 30 && relX < 44 {
			// Cancel button
			f.Hide()
			return f, func() tea.Msg { return FilterCancelMsg{} }
		}
	}

	return f, nil
}

func (f *FilterDialog) renderCheckbox(label string, checked bool, focused bool) string {
	box := "[ ]"
	if checked {
		box = "[x]"
	}

	prefix := "  "
	if focused {
		prefix = "> "
	}

	return fmt.Sprintf("%s%s %s", prefix, box, label)
}

func (f *FilterDialog) renderButton(label string, focused bool) string {
	if focused {
		return lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("33")).
			Render("[ " + label + " ]")
	}
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Render("  " + label + "  ")
}

func (f *FilterDialog) renderProductTags() string {
	if len(f.selectedProducts) == 0 {
		return ""
	}

	tagStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("255")).
		Padding(0, 1)

	var tags []string
	for _, p := range f.selectedProducts {
		// Truncate long product names in tags
		name := p
		if len(name) > 20 {
			name = name[:17] + "..."
		}
		tags = append(tags, tagStyle.Render(name+" ×"))
	}

	return strings.Join(tags, " ")
}

// renderProductDropdownBox renders the product dropdown as a separate bordered box
func (f *FilterDialog) renderProductDropdownBox() string {
	var content strings.Builder
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))

	content.WriteString(titleStyle.Render("Select Product"))
	content.WriteString("\n\n")

	if f.productsLoading {
		content.WriteString(helpStyle.Render("Loading products..."))
	} else if f.productsError != "" {
		content.WriteString(helpStyle.Render("Failed to load products"))
	} else if len(f.products) == 0 {
		content.WriteString(helpStyle.Render("No products available"))
	} else if len(f.productMatches) == 0 {
		content.WriteString(helpStyle.Render("No matches"))
	} else {
		maxItems := 10
		start := 0
		if f.productCursor >= maxItems {
			start = f.productCursor - maxItems + 1
		}
		end := start + maxItems
		if end > len(f.productMatches) {
			end = len(f.productMatches)
		}

		listWidth := 50
		for i := start; i < end; i++ {
			line := truncateSimpleFD(f.productMatches[i], listWidth)
			if i == f.productCursor {
				line = f.styles.Selected.Render("> " + line)
			} else {
				line = "  " + line
			}
			content.WriteString(line)
			content.WriteString("\n")
		}

		// Show count indicator
		content.WriteString("\n")
		content.WriteString(helpStyle.Render(fmt.Sprintf("(%d/%d) ↑↓ navigate, Enter select", f.productCursor+1, len(f.productMatches))))
	}

	// Box style for dropdown
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		Padding(1, 2).
		Width(58)

	return boxStyle.Render(content.String())
}

// renderProductMatchesCompact renders the dropdown as a compact box for side overlay
func (f *FilterDialog) renderProductMatchesCompact() string {
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))

	if f.productsLoading {
		return helpStyle.Render("Loading...")
	}
	if f.productsError != "" {
		return helpStyle.Render("Load failed")
	}
	if len(f.products) == 0 {
		return helpStyle.Render("No products")
	}
	if len(f.productMatches) == 0 {
		return helpStyle.Render("No matches")
	}

	var lines []string
	maxItems := 6
	start := 0
	if f.productCursor >= maxItems {
		start = f.productCursor - maxItems + 1
	}
	end := start + maxItems
	if end > len(f.productMatches) {
		end = len(f.productMatches)
	}

	listWidth := 28
	for i := start; i < end; i++ {
		line := truncateSimpleFD(f.productMatches[i], listWidth)
		if i == f.productCursor {
			line = f.styles.Selected.Render("> " + line)
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}

	// Show count indicator if there are more items
	if len(f.productMatches) > maxItems {
		countStr := helpStyle.Render(fmt.Sprintf("(%d/%d)", f.productCursor+1, len(f.productMatches)))
		lines = append(lines, countStr)
	}

	return strings.Join(lines, "\n")
}

func (f *FilterDialog) renderProductMatches() string {
	var sb strings.Builder
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	sb.WriteString("  ")
	sb.WriteString(helpStyle.Render("Products:"))
	sb.WriteString("\n")

	if f.productsLoading {
		sb.WriteString("    ")
		sb.WriteString(helpStyle.Render("Loading products..."))
		sb.WriteString("\n")
		return sb.String()
	}
	if f.productsError != "" {
		sb.WriteString("    ")
		sb.WriteString(helpStyle.Render("Failed to load products"))
		sb.WriteString("\n")
		sb.WriteString("    ")
		sb.WriteString(helpStyle.Render(truncateSimpleFD(f.productsError, 46)))
		sb.WriteString("\n")
		return sb.String()
	}
	if len(f.products) == 0 {
		sb.WriteString("    ")
		sb.WriteString(helpStyle.Render("No products available"))
		sb.WriteString("\n")
		return sb.String()
	}
	if len(f.productMatches) == 0 {
		sb.WriteString("    ")
		sb.WriteString(helpStyle.Render("No matches"))
		sb.WriteString("\n")
		return sb.String()
	}

	maxItems := 6
	start := 0
	if f.productCursor >= maxItems {
		start = f.productCursor - maxItems + 1
	}
	end := start + maxItems
	if end > len(f.productMatches) {
		end = len(f.productMatches)
	}

	listWidth := 46
	for i := start; i < end; i++ {
		line := truncateSimpleFD(f.productMatches[i], listWidth)
		if i == f.productCursor {
			line = f.styles.Selected.Render("  > " + line)
		} else {
			line = "    " + line
		}
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	return sb.String()
}

func truncateSimpleFD(s string, width int) string {
	if width <= 0 {
		return ""
	}
	if len(s) <= width {
		return s
	}
	if width <= 3 {
		return s[:width]
	}
	return s[:width-3] + "..."
}

// View renders the dialog
func (f *FilterDialog) View() string {
	if !f.visible {
		return ""
	}

	var content strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("33"))
	content.WriteString(titleStyle.Render("Filter Cases"))
	content.WriteString("\n\n")

	// Accounts field
	prefix := "  "
	if f.focusedField == fieldAccounts {
		prefix = "> "
	}
	content.WriteString(fmt.Sprintf("%sAccounts: %s", prefix, f.accountsInput.View()))
	content.WriteString("\n\n")

	// Status checkboxes
	content.WriteString("  Status:\n")
	content.WriteString(f.renderCheckbox("Open", f.statusOpen, f.focusedField == fieldStatusOpen))
	content.WriteString("\n")
	content.WriteString(f.renderCheckbox("Waiting on Red Hat", f.statusWaitingRH, f.focusedField == fieldStatusWaitingRH))
	content.WriteString("\n")
	content.WriteString(f.renderCheckbox("Waiting on Customer", f.statusWaitingCust, f.focusedField == fieldStatusWaitingCust))
	content.WriteString("\n")
	content.WriteString(f.renderCheckbox("Closed", f.statusClosed, f.focusedField == fieldStatusClosed))
	content.WriteString("\n\n")

	// Severity checkboxes (in a row)
	content.WriteString("  Severity:\n")
	content.WriteString("  ")
	content.WriteString(f.renderCheckbox("1", f.sev1, f.focusedField == fieldSev1))
	content.WriteString(" ")
	content.WriteString(f.renderCheckbox("2", f.sev2, f.focusedField == fieldSev2))
	content.WriteString(" ")
	content.WriteString(f.renderCheckbox("3", f.sev3, f.focusedField == fieldSev3))
	content.WriteString(" ")
	content.WriteString(f.renderCheckbox("4", f.sev4, f.focusedField == fieldSev4))
	content.WriteString("\n\n")

	// Product field with tag-based selection
	prefix = "  "
	if f.focusedField == fieldProduct {
		prefix = "> "
	}

	// Show selected products as tags
	if len(f.selectedProducts) > 0 {
		content.WriteString("  Products: ")
		content.WriteString(f.renderProductTags())
		content.WriteString("\n")
	}

	// Product input (dropdown rendered separately as overlay)
	content.WriteString(fmt.Sprintf("%sAdd:      %s", prefix, f.productInput.View()))
	content.WriteString("\n\n")

	// Keyword field
	prefix = "  "
	if f.focusedField == fieldKeyword {
		prefix = "> "
	}
	content.WriteString(fmt.Sprintf("%sKeyword:  %s", prefix, f.keywordInput.View()))
	content.WriteString("\n\n")

	// Separator
	content.WriteString(strings.Repeat("─", 54))
	content.WriteString("\n\n")

	// Buttons - render horizontally using lipgloss.JoinHorizontal
	applyBtn := f.renderButton("Apply", f.focusedField == fieldApply)
	clearBtn := f.renderButton("Clear", f.focusedField == fieldClear)
	cancelBtn := f.renderButton("Cancel", f.focusedField == fieldCancel)
	buttons := lipgloss.JoinHorizontal(lipgloss.Center, applyBtn, "  ", clearBtn, "  ", cancelBtn)
	content.WriteString("  ")
	content.WriteString(buttons)
	content.WriteString("\n\n")

	// Help text
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	content.WriteString(helpStyle.Render("Tab/↑↓: Navigate  Space: Toggle  Enter: Select  Esc: Cancel"))

	// Modal box style - no background, just border
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("33")).
		Padding(1, 2).
		Width(60)

	return boxStyle.Render(content.String())
}

// ShouldShowProductDropdown returns true if the product dropdown should be displayed
func (f *FilterDialog) ShouldShowProductDropdown() bool {
	return f.visible && f.focusedField == fieldProduct
}

// RenderProductDropdown renders just the product dropdown box for overlay
func (f *FilterDialog) RenderProductDropdown() string {
	return f.renderProductDropdownBox()
}

// GetDropdownPosition returns the X,Y position where the dropdown should be placed
func (f *FilterDialog) GetDropdownPosition() (int, int) {
	// Position dropdown to the right of the main dialog
	dropdownX := f.dialogX + 62 // main dialog width (60) + small gap
	dropdownY := f.dialogY + 8  // align roughly with the product field
	return dropdownX, dropdownY
}
