// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Anthony Green <green@redhat.com>
package components

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// AnsiCut cuts a string at the given visual positions, keeping CSI and OSC
// escape sequences intact so styling and OSC-8 hyperlinks survive the cut.
func AnsiCut(s string, start, end int) string {
	var result strings.Builder
	visualPos := 0

	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			seqEnd := i + 1
			if i+1 < len(s) && s[i+1] == '[' {
				// CSI sequence: ESC [ ... final byte (0x40-0x7E)
				seqEnd = i + 2
				for seqEnd < len(s) && (s[seqEnd] < 0x40 || s[seqEnd] > 0x7e) {
					seqEnd++
				}
				if seqEnd < len(s) {
					seqEnd++
				}
			} else if i+1 < len(s) && s[i+1] == ']' {
				// OSC sequence: ESC ] ... BEL or ST (ESC \)
				seqEnd = i + 2
				for seqEnd < len(s) {
					if s[seqEnd] == 0x07 {
						seqEnd++
						break
					}
					if s[seqEnd] == 0x1b && seqEnd+1 < len(s) && s[seqEnd+1] == '\\' {
						seqEnd += 2
						break
					}
					seqEnd++
				}
			} else if i+1 < len(s) {
				seqEnd = i + 2
			}
			if visualPos >= start && visualPos < end {
				result.WriteString(s[i:seqEnd])
			}
			i = seqEnd
			continue
		}

		r, size := utf8.DecodeRuneInString(s[i:])
		w := runewidth.RuneWidth(r)
		// Keep a rune only if it fits entirely inside the window, so a
		// double-width rune straddling the boundary is dropped, not split
		if visualPos >= start && visualPos+w <= end {
			result.WriteString(s[i : i+size])
		}
		visualPos += w
		i += size
		if visualPos >= end {
			break
		}
	}
	return result.String()
}

// ansiCutWidth trims a string to the given visual width, preserving escapes.
func ansiCutWidth(s string, width int) string {
	return AnsiCut(s, 0, width)
}

// stripANSI removes CSI and OSC escape sequences, leaving plain text.
func stripANSI(s string) string {
	var out strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b {
			if i+1 < len(s) && s[i+1] == '[' {
				end := i + 2
				for end < len(s) && (s[end] < 0x40 || s[end] > 0x7e) {
					end++
				}
				if end < len(s) {
					end++
				}
				i = end
				continue
			}
			if i+1 < len(s) && s[i+1] == ']' {
				end := i + 2
				for end < len(s) {
					if s[end] == 0x07 {
						end++
						break
					}
					if s[end] == 0x1b && end+1 < len(s) && s[end+1] == '\\' {
						end += 2
						break
					}
					end++
				}
				i = end
				continue
			}
			i++
			continue
		}
		out.WriteByte(s[i])
		i++
	}
	return out.String()
}
