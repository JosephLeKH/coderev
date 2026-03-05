package git

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gobwas/glob"
)

// ErrNotGitRepo is returned when the command is run outside a git repository.
var ErrNotGitRepo = errors.New("not a git repository")

// ErrNothingToReview is returned when there are no staged changes and no --target was given.
var ErrNothingToReview = errors.New("nothing to review — stage changes or pass --target")

const maxTokensPerChunk = 8000

// FileChunk represents a single file's diff content.
type FileChunk struct {
	Filename string
	Content  string // Raw unified diff hunks for this file
	NumLines int    // Number of added+removed lines
}

// EstimatedTokens returns the approximate token count using 4 chars ≈ 1 token.
func (fc FileChunk) EstimatedTokens() int {
	return (len(fc.Content) + 3) / 4
}

// GetDiff shells out to git diff and returns parsed file chunks.
// If target is empty, diffs staged changes; otherwise diffs against the target revision.
func GetDiff(target string) ([]FileChunk, error) {
	args := []string{"diff"}
	if target == "" {
		args = append(args, "--staged")
	} else {
		args = append(args, target)
	}

	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "not a git repository") {
				return nil, ErrNotGitRepo
			}
			return nil, fmt.Errorf("git diff failed: %s", strings.TrimSpace(stderr))
		}
		return nil, fmt.Errorf("git diff failed: %w", err)
	}

	chunks, err := ParseDiff(string(out))
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 && target == "" {
		return nil, ErrNothingToReview
	}
	return chunks, nil
}

// ParseDiff parses a unified diff string into per-file FileChunks.
func ParseDiff(diff string) ([]FileChunk, error) {
	if strings.TrimSpace(diff) == "" {
		return nil, nil
	}

	var chunks []FileChunk
	lines := strings.Split(diff, "\n")

	var currentFile string
	var currentLines []string

	flush := func() {
		if currentFile == "" || len(currentLines) == 0 {
			return
		}
		content := strings.Join(currentLines, "\n")
		if isBinaryContent(content) {
			return
		}
		chunks = append(chunks, FileChunk{
			Filename: currentFile,
			Content:  content,
			NumLines: countChangedLines(currentLines),
		})
	}

	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git ") {
			flush()
			currentFile = extractFilename(line)
			currentLines = nil
		} else if currentFile != "" {
			currentLines = append(currentLines, line)
		}
	}
	flush()

	return chunks, nil
}

// FilterChunks removes chunks whose filenames match any ignore glob pattern.
func FilterChunks(chunks []FileChunk, ignorePatterns []string) []FileChunk {
	if len(ignorePatterns) == 0 {
		return chunks
	}

	// Pre-compile globs once
	globs := make([]glob.Glob, 0, len(ignorePatterns))
	for _, pattern := range ignorePatterns {
		g, err := glob.Compile(pattern, '/')
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: invalid ignore pattern %q: %v\n", pattern, err)
			continue
		}
		globs = append(globs, g)
	}

	var result []FileChunk
	for _, chunk := range chunks {
		if !matchesAnyGlob(chunk.Filename, globs) {
			result = append(result, chunk)
		}
	}
	return result
}

// TruncateChunk truncates a FileChunk to the token limit.
// Returns the (possibly truncated) chunk and a warning string (empty if not truncated).
func TruncateChunk(chunk FileChunk) (FileChunk, string) {
	if chunk.EstimatedTokens() <= maxTokensPerChunk {
		return chunk, ""
	}
	maxChars := maxTokensPerChunk * 4
	if maxChars > len(chunk.Content) {
		maxChars = len(chunk.Content)
	}
	truncated := chunk.Content[:maxChars]
	// Align to the last newline to avoid cutting mid-line
	if idx := strings.LastIndex(truncated, "\n"); idx > 0 {
		truncated = truncated[:idx]
	}
	chunk.Content = truncated
	return chunk, fmt.Sprintf("warning: %s exceeds token limit and was truncated", chunk.Filename)
}

func extractFilename(line string) string {
	// "diff --git a/<file> b/<file>" — take the b/ side (handles renames correctly)
	parts := strings.Fields(line)
	if len(parts) < 4 {
		return ""
	}
	return strings.TrimPrefix(parts[3], "b/")
}

func isBinaryContent(content string) bool {
	return strings.Contains(content, "Binary files")
}

func countChangedLines(lines []string) int {
	count := 0
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if (line[0] == '+' || line[0] == '-') &&
			!strings.HasPrefix(line, "+++") &&
			!strings.HasPrefix(line, "---") {
			count++
		}
	}
	return count
}

func matchesAnyGlob(filename string, globs []glob.Glob) bool {
	base := filepath.Base(filename)
	for _, g := range globs {
		if g.Match(filename) || g.Match(base) {
			return true
		}
	}
	return false
}
