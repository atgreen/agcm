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
	accountsInput   textinput.Model
	productInput    textinput.Model
	keywordInput    textinput.Model
	products        []string
	productMatches  []string
	productCursor   int
	productsLoading bool
	productsError   string

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
}

// NewFilterDialog creates a new filter dialog
func NewFilterDialog(s *styles.Styles) *FilterDialog {
	accountsInput := textinput.New()
	accountsInput.Placeholder = "account number(s), comma-separated"
	accountsInput.CharLimit = 100
	accountsInput.Width = 30
	accountsInput.Prompt = ""

	productInput := textinput.New()
	productInput.Placeholder = "enter product name"
	productInput.CharLimit = 50
	productInput.Width = 30
	productInput.Prompt = ""

	keywordInput := textinput.New()
	keywordInput.Placeholder = "enter search keywords"
	keywordInput.CharLimit = 100
	keywordInput.Width = 30
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
		f.productInput.SetValue(filter.Product)
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

// SetSize sets the dialog size
func (f *FilterDialog) SetSize(width, height int) {
	f.width = width
	f.height = height
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

	// Product
	if prod := strings.TrimSpace(f.productInput.Value()); prod != "" {
		filter.Product = prod
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
	if query == "" {
		f.productMatches = append(f.productMatches, f.products...)
	} else {
		for _, p := range f.products {
			if strings.Contains(strings.ToLower(p), query) {
				f.productMatches = append(f.productMatches, p)
			}
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

		case "enter":
			if f.focusedField == fieldProduct && len(f.productMatches) > 0 {
				if f.productCursor >= 0 && f.productCursor < len(f.productMatches) {
					f.productInput.SetValue(f.productMatches[f.productCursor])
					f.updateProductMatches()
				}
				f.focusedField = (f.focusedField + 1) % fieldCount
				f.focusField(f.focusedField)
				return f, nil
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

	// Product field
	prefix = "  "
	if f.focusedField == fieldProduct {
		prefix = "> "
	}
	content.WriteString(fmt.Sprintf("%sProduct:  %s", prefix, f.productInput.View()))
	content.WriteString("\n")
	if f.focusedField == fieldProduct {
		content.WriteString(f.renderProductMatches())
		content.WriteString("\n")
	}
	content.WriteString("\n")

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
