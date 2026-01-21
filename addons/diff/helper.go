package diff

import (
	"bufio"
	"bytes"
	"log"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/oligo/gvcode/gutter/providers"
)

// ParseGitDiff runs git diff on the given file and parses the output into DiffHunks.
func ParseGitDiff(filePath string) []*providers.DiffHunk {
	if _, err := exec.LookPath("git"); err != nil {
		return nil
	}

	// Get the absolute path and directory
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		log.Printf("Failed to get absolute path: %v", err)
		return nil
	}
	dir := filepath.Dir(absPath)
	fileName := filepath.Base(absPath)

	// Run git diff
	cmd := exec.Command("git", "diff", "--no-color", "-U0", "--", fileName)
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		// git diff returns exit code 1 if there are changes, which is not an error
		if exitErr, ok := err.(*exec.ExitError); ok {
			if len(exitErr.Stderr) > 0 {
				log.Printf("git diff stderr: %s", exitErr.Stderr)
			}
		}
	}

	if len(output) == 0 {
		return nil
	}

	return parseDiffOutput(output)
}

// parseDiffOutput parses unified diff output into DiffHunks.
func parseDiffOutput(output []byte) []*providers.DiffHunk {
	var hunks []*providers.DiffHunk

	scanner := bufio.NewScanner(bytes.NewReader(output))
	var currentHunk *providers.DiffHunk
	var inHunk bool

	// Regex to match hunk headers like @@ -10,3 +10,5 @@
	hunkHeaderRe := regexp.MustCompile(`^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@`)

	for scanner.Scan() {
		line := scanner.Text()

		// Check for hunk header
		if matches := hunkHeaderRe.FindStringSubmatch(line); matches != nil {
			// Save previous hunk if exists
			if currentHunk != nil {
				hunks = append(hunks, currentHunk)
			}

			oldStart, _ := strconv.Atoi(matches[1])
			oldCount := 1
			if matches[2] != "" {
				oldCount, _ = strconv.Atoi(matches[2])
			}
			newStart, _ := strconv.Atoi(matches[3])
			newCount := 1
			if matches[4] != "" {
				newCount, _ = strconv.Atoi(matches[4])
			}

			// Convert to 0-based line numbers
			oldStart--
			newStart--

			// Determine hunk type
			var diffType providers.DiffType
			if oldCount == 0 {
				diffType = providers.DiffAdded
			} else if newCount == 0 {
				diffType = providers.DiffDeleted
			} else {
				diffType = providers.DiffModified
			}

			currentHunk = &providers.DiffHunk{
				Type:      diffType,
				StartLine: newStart,
				EndLine:   newStart + max(newCount-1, 0),
				OldLines:  make([]string, 0),
				NewLines:  make([]string, 0),
			}

			// For deleted hunks, the line number is where the deletion occurred
			if diffType == providers.DiffDeleted {
				currentHunk.StartLine = newStart
				currentHunk.EndLine = newStart
			}

			inHunk = true
			continue
		}

		// Skip diff headers
		if strings.HasPrefix(line, "diff ") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") {
			continue
		}

		// Process hunk content
		if inHunk && currentHunk != nil {
			if strings.HasPrefix(line, "-") {
				currentHunk.OldLines = append(currentHunk.OldLines, strings.TrimPrefix(line, "-"))
			} else if strings.HasPrefix(line, "+") {
				currentHunk.NewLines = append(currentHunk.NewLines, strings.TrimPrefix(line, "+"))
			}
		}
	}

	// Don't forget the last hunk
	if currentHunk != nil {
		hunks = append(hunks, currentHunk)
	}

	return hunks
}
