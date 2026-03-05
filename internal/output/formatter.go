package output

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ReviewComment represents a single issue found by the model.
type ReviewComment struct {
	File     string `json:"file"`
	Severity string `json:"severity"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
}

// FileResult groups comments for a single file.
type FileResult struct {
	File     string          `json:"file"`
	Comments []ReviewComment `json:"comments"`
	Raw      string          `json:"-"` // raw model output, not serialized
}

// commentPattern matches lines like: [BUG] L5 Some message
var commentPattern = regexp.MustCompile(`^\[([A-Z]+)\]\s+L(\d+)\s+(.+)$`)

// ParseComments extracts ReviewComments from the raw model output for a file.
func ParseComments(filename, raw string) []ReviewComment {
	var comments []ReviewComment
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		m := commentPattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lineNum, _ := strconv.Atoi(m[2])
		comments = append(comments, ReviewComment{
			File:     filename,
			Severity: m[1],
			Line:     lineNum,
			Message:  strings.TrimSpace(m[3]),
		})
	}
	return comments
}

// severityColor maps severity to ANSI color codes.
var severityColor = map[string]string{
	"BUG":         "\033[31m", // red
	"SECURITY":    "\033[35m", // magenta
	"PERFORMANCE": "\033[33m", // yellow
}

const colorReset = "\033[0m"

// FormatTerminal renders results as human-readable terminal output with ANSI colors.
func FormatTerminal(results []FileResult) string {
	var sb strings.Builder
	totalIssues := 0
	severityCounts := make(map[string]int)

	for _, r := range results {
		sb.WriteString(fmt.Sprintf("\n\033[1m%s\033[0m\n", r.File))
		if len(r.Comments) == 0 {
			sb.WriteString("  ✓ No issues found\n")
			continue
		}
		for _, c := range r.Comments {
			color := severityColor[c.Severity]
			if color == "" {
				color = "\033[37m"
			}
			sb.WriteString(fmt.Sprintf("  %s[%s]%s L%d %s\n",
				color, c.Severity, colorReset, c.Line, c.Message))
			totalIssues++
			severityCounts[c.Severity]++
		}
	}

	sb.WriteString(fmt.Sprintf("\n%d issue(s) found across %d file(s)", totalIssues, len(results)))
	if totalIssues > 0 {
		sb.WriteString(" · ")
		parts := make([]string, 0, len(severityCounts))
		for _, sev := range []string{"BUG", "SECURITY", "PERFORMANCE"} {
			if n := severityCounts[sev]; n > 0 {
				parts = append(parts, fmt.Sprintf("%d %s", n, strings.ToLower(sev)))
			}
		}
		// Append any unknown severities at the end.
		for sev, n := range severityCounts {
			known := sev == "BUG" || sev == "SECURITY" || sev == "PERFORMANCE"
			if !known {
				parts = append(parts, fmt.Sprintf("%d %s", n, strings.ToLower(sev)))
			}
		}
		sb.WriteString(strings.Join(parts, " · "))
	}
	sb.WriteString("\n")
	return sb.String()
}

// FormatJSON serializes results to a JSON string.
func FormatJSON(results []FileResult) (string, error) {
	data, err := json.MarshalIndent(results, "", "  ")
	if err != nil {
		return "", fmt.Errorf("serializing results: %w", err)
	}
	return string(data), nil
}
