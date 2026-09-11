// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>

// Package about holds the project metadata shown in the CLI help
// and the TUI About box, so it lives in exactly one place.
package about

const (
	Author   = "Anthony Green <green@redhat.com>"
	License  = "GPL-3.0-or-later"
	Homepage = "https://github.com/atgreen/agcm"
	Issues   = "https://github.com/atgreen/agcm/issues"
)

// HelpFooter is appended to the CLI help output
const HelpFooter = `
Author:   ` + Author + `
License:  ` + License + `
Homepage: ` + Homepage + `

Please report bugs and feature requests at:
  ` + Issues + `
`
