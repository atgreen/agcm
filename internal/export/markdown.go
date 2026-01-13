// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package export

import (
	"bytes"
	"fmt"
	"html"
	"regexp"
	"strings"
	"text/template"
	"time"

	"github.com/green/agcm/internal/api"
)

// DefaultTemplate is the default markdown template for case export
const DefaultTemplate = `# Case {{.Case.CaseNumber}}: {{.Case.Summary}}

## Metadata

| Field | Value |
|-------|-------|
| Case Number | {{.Case.CaseNumber}} |
| Status | {{.Case.Status}} |
| Severity | {{.Case.Severity}} |
| Product | {{.Case.Product}} {{if .Case.Version}}{{.Case.Version}}{{end}} |
| Type | {{.Case.Type}} |
| Created | {{formatTime .Case.CreatedDate}} |
| Last Updated | {{formatTime .Case.LastModified}} |
{{- if .Case.ClosedDate}}
| Closed | {{formatTime .Case.ClosedDate}} |
{{- end}}
| Owner | {{.Case.Owner}} |
| Contact | {{.Case.ContactName}} ({{.Case.ContactEmail}}) |
| Account | {{.Case.AccountName}} ({{.Case.AccountNumber}}) |

## Summary

{{.Case.Summary}}

## Description

{{cleanHTML .Case.Description}}

## Conversation

{{range $i, $c := .Comments}}
### Comment {{add $i 1}}
**From:** {{$c.Author}}{{if $c.AuthorEmail}} ({{$c.AuthorEmail}}){{end}}
**Date:** {{formatTime $c.CreatedDate}}
**Type:** {{if $c.IsPublicComment}}Public{{else}}Internal{{end}}

{{cleanHTML $c.GetText}}

---

{{end}}
{{if .Attachments}}
## Attachments

| Filename | Size | UUID | Uploaded |
|----------|------|------|----------|
{{range .Attachments -}}
| {{.Filename}} | {{formatSize .Length}} | {{truncUUID .UUID}} | {{formatTime .CreatedDate}} |
{{end}}
{{end}}
---
*Exported by agcm on {{formatTime .ExportedAt}}*
`

// CaseExport contains all data for exporting a case
type CaseExport struct {
	Case       *api.Case
	Comments   []api.Comment
	Attachments []api.Attachment
	ExportedAt time.Time
}

// Formatter handles markdown formatting
type Formatter struct {
	tmpl *template.Template
}

// NewFormatter creates a new markdown formatter with the default template
func NewFormatter() (*Formatter, error) {
	return NewFormatterWithTemplate(DefaultTemplate)
}

// NewFormatterWithTemplate creates a formatter with a custom template
func NewFormatterWithTemplate(tmplStr string) (*Formatter, error) {
	funcMap := template.FuncMap{
		"formatTime": formatTime,
		"formatSize": formatSize,
		"cleanHTML":  cleanHTML,
		"truncUUID":  truncUUID,
		"add":        func(a, b int) int { return a + b },
	}

	tmpl, err := template.New("case").Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse template: %w", err)
	}

	return &Formatter{tmpl: tmpl}, nil
}

// FormatCase formats a case export to markdown
func (f *Formatter) FormatCase(export *CaseExport) (string, error) {
	var buf bytes.Buffer
	if err := f.tmpl.Execute(&buf, export); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.String(), nil
}

// FormatCases formats multiple cases into a single markdown document
func (f *Formatter) FormatCases(exports []*CaseExport) (string, error) {
	var parts []string
	for _, export := range exports {
		md, err := f.FormatCase(export)
		if err != nil {
			return "", err
		}
		parts = append(parts, md)
	}
	return strings.Join(parts, "\n\n---\n\n"), nil
}

// formatTime formats a time value for display
func formatTime(t interface{}) string {
	switch v := t.(type) {
	case time.Time:
		if v.IsZero() {
			return "N/A"
		}
		return v.UTC().Format("2006-01-02 15:04:05 UTC")
	case *time.Time:
		if v == nil || v.IsZero() {
			return "N/A"
		}
		return v.UTC().Format("2006-01-02 15:04:05 UTC")
	default:
		return "N/A"
	}
}

// formatSize formats a byte size for display
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// cleanHTML converts HTML to plain text/markdown
func cleanHTML(s string) string {
	// Decode HTML entities
	s = html.UnescapeString(s)

	// Replace common HTML tags with markdown equivalents
	replacements := []struct {
		pattern string
		replace string
	}{
		{`<br\s*/?>`, "\n"},
		{`<p>`, "\n"},
		{`</p>`, "\n"},
		{`<strong>|<b>`, "**"},
		{`</strong>|</b>`, "**"},
		{`<em>|<i>`, "*"},
		{`</em>|</i>`, "*"},
		{`<code>`, "`"},
		{`</code>`, "`"},
		{`<pre>`, "\n```\n"},
		{`</pre>`, "\n```\n"},
		{`<li>`, "\n- "},
		{`</li>`, ""},
		{`<ul>|<ol>`, "\n"},
		{`</ul>|</ol>`, "\n"},
		{`<h1>`, "\n# "},
		{`</h1>`, "\n"},
		{`<h2>`, "\n## "},
		{`</h2>`, "\n"},
		{`<h3>`, "\n### "},
		{`</h3>`, "\n"},
		{`<blockquote>`, "\n> "},
		{`</blockquote>`, "\n"},
		{`<a\s+href="([^"]+)"[^>]*>([^<]+)</a>`, "[$2]($1)"},
	}

	for _, r := range replacements {
		re := regexp.MustCompile("(?i)" + r.pattern)
		s = re.ReplaceAllString(s, r.replace)
	}

	// Remove any remaining HTML tags
	tagRe := regexp.MustCompile(`<[^>]+>`)
	s = tagRe.ReplaceAllString(s, "")

	// Clean up excessive whitespace
	s = regexp.MustCompile(`\n{3,}`).ReplaceAllString(s, "\n\n")
	s = strings.TrimSpace(s)

	return s
}

// truncUUID truncates a UUID for display
func truncUUID(uuid string) string {
	if len(uuid) > 13 {
		return uuid[:8] + "..."
	}
	return uuid
}

// QuickFormat is a convenience function for simple case formatting
func QuickFormat(c *api.Case, comments []api.Comment, attachments []api.Attachment) (string, error) {
	f, err := NewFormatter()
	if err != nil {
		return "", err
	}

	export := &CaseExport{
		Case:        c,
		Comments:    comments,
		Attachments: attachments,
		ExportedAt:  time.Now(),
	}

	return f.FormatCase(export)
}
