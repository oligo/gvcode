package gvcode

import (
	"bufio"
	"strings"
)

type TabStyle uint8

const (
	Tabs TabStyle = iota
	Spaces
)

// GuessIndentation guesses which kind of indentation the editor is
// using, returing the kind, if mixed indent is used, and the indent
// size in the case if spaces indentation.
func GuessIndentation(text string) (TabStyle, bool, int) {
	scanner := bufio.NewScanner(strings.NewReader(text))

	var tabs, spaces int
	var spaceWidths = make(map[int]int)

	indentScanner := func(batchSize int) bool {
		for scanner.Scan() {
			line := scanner.Text()
			if strings.TrimSpace(line) == "" {
				continue // Ignore empty lines
			}

			// Count tabs and spaces
			if strings.HasPrefix(line, "\t") {
				tabs++
			} else if strings.HasPrefix(line, " ") {
				// Count leading spaces
				leading := len(line) - len(strings.TrimLeft(line, " "))
				spaces++
				spaceWidths[leading]++
			}

			// Stop early if we've analyzed enough lines
			if spaces+tabs > batchSize {
				return true
			}
		}

		return false
	}

	for {
		hasMore := indentScanner(100)
		if (tabs+spaces <= 5 || spaces == tabs) && hasMore {
			continue
		}

		mixedIndent := tabs > 0 && spaces > 0
		mainIndent := Tabs
		if tabs > spaces {
			mainIndent = Tabs
		} else if spaces > tabs {
			mainIndent = Spaces
		}

		// If there are spaces, find the most common space width
		bestWidth, maxFreq := 4, 0
		for width, freq := range spaceWidths {
			if width > 0 && freq > maxFreq {
				bestWidth, maxFreq = width, freq
			}
		}

		return mainIndent, mixedIndent, bestWidth
	}
}

type LineEnding string

const (
	LF   LineEnding = "\n"
	CRLF LineEnding = "\r\n"
)

const (
	// maxFullScanSize defines the limit (64KB) under which we scan the whole file
	maxFullScanSize = 32 * 1024
	// sniffBufferSize is the fallback size for very large files
	sniffBufferSize = 4096
)

// DetectLineEnding intelligently determines the line ending convention.
// It scans the whole file for the most common ending if small,
// otherwise it sniffs the first 4KB for the first occurrence.
func DetectLineEnding(text string) LineEnding {
	size := len(text)
	if size == 0 {
		return LF
	}

	// For small file do a full scan for majority rule
	if size <= maxFullScanSize {
		crlfCount := strings.Count(text, "\r\n")
		// Count total \n and subtract those that are part of \r\n
		lfCount := strings.Count(text, "\n") - crlfCount

		if crlfCount > lfCount {
			return CRLF
		}
		return LF
	}

	// For large file, quick sniff of the first 4KB
	content := text[:min(sniffBufferSize, size)]
	index := strings.IndexByte(content, '\n')

	// If the byte before the first \n is \r, assume CRLF for the file
	if index > 0 && content[index-1] == '\r' {
		return CRLF
	}

	return LF
}

func StripLineEnding(text string) string {
	normalized := strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(normalized, "\r", "\n")
}

func ToCRLF(text string) string {
	pureLF := strings.ReplaceAll(text, "\r\n", "\n")
	return strings.ReplaceAll(pureLF, "\n", "\r\n")
}

// A helper method to convert line endings based on previously detected one.
func (e *Editor) PrepareForSave(normalizedContent string) string {
	if e.lineEnding == CRLF {
		// Remove any stray \r (just in case)
		clean := strings.ReplaceAll(normalizedContent, "\r", "")
		// Perform the safe conversion
		return strings.ReplaceAll(clean, "\n", "\r\n")
	}
	return normalizedContent
}
