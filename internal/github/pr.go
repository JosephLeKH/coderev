package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	"github.com/JosephLeKH/coderev/internal/git"
	"github.com/JosephLeKH/coderev/internal/output"
)

// ErrNoPR is returned when no open PR is found for the current branch.
var ErrNoPR = errors.New("no open pull request found for current branch")

var (
	httpsPattern = regexp.MustCompile(`^https?://github\.com/([^/]+)/([^/]+?)(?:\.git)?$`)
	sshPattern   = regexp.MustCompile(`^git@github\.com:([^/]+)/([^/]+?)(?:\.git)?$`)
	hunkHeader   = regexp.MustCompile(`^@@ -\d+(?:,\d+)? \+(\d+)(?:,\d+)? @@`)
)

const githubAPIBase = "https://api.github.com"

// reviewComment is a single inline comment for the GitHub Pull Reviews API.
type reviewComment struct {
	Path     string `json:"path"`
	Position int    `json:"position"`
	Body     string `json:"body"`
}

// PostReview posts inline review comments on the open PR for the current branch.
// Returns ErrNoPR if no open PR is found (caller should skip gracefully).
func PostReview(ctx context.Context, results []output.FileResult, chunks []git.FileChunk) error {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return errors.New("GITHUB_TOKEN environment variable not set")
	}

	remoteURL, err := getRemoteURL()
	if err != nil {
		return fmt.Errorf("getting git remote: %w", err)
	}

	owner, repo, err := parseRemoteURL(remoteURL)
	if err != nil {
		return err
	}

	branch, err := getCurrentBranch()
	if err != nil {
		return fmt.Errorf("getting current branch: %w", err)
	}

	prNumber, err := getPRNumber(ctx, token, owner, repo, branch)
	if err != nil {
		return err // includes ErrNoPR
	}

	// Index chunk content by filename for position mapping.
	chunkMap := make(map[string]string, len(chunks))
	for _, c := range chunks {
		chunkMap[c.Filename] = c.Content
	}

	var comments []reviewComment
	for _, r := range results {
		if len(r.Comments) == 0 {
			continue
		}
		content, ok := chunkMap[r.File]
		if !ok {
			continue
		}
		posMap := buildPositionMap(content)
		for _, c := range r.Comments {
			pos, ok := posMap[c.Line]
			if !ok {
				continue // line not found in diff (e.g. removed line or truncated), skip
			}
			comments = append(comments, reviewComment{
				Path:     r.File,
				Position: pos,
				Body:     fmt.Sprintf("[%s] %s", c.Severity, c.Message),
			})
		}
	}

	if len(comments) == 0 {
		return nil // nothing to post
	}

	return postPRReview(ctx, token, owner, repo, prNumber, comments)
}

// parseRemoteURL extracts owner and repo from a GitHub remote URL (HTTPS or SSH).
func parseRemoteURL(remote string) (owner, repo string, err error) {
	remote = strings.TrimSpace(remote)
	if m := httpsPattern.FindStringSubmatch(remote); m != nil {
		return m[1], m[2], nil
	}
	if m := sshPattern.FindStringSubmatch(remote); m != nil {
		return m[1], m[2], nil
	}
	return "", "", fmt.Errorf("remote %q is not a recognized GitHub URL", remote)
}

func getRemoteURL() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", fmt.Errorf("git remote get-url origin: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func getCurrentBranch() (string, error) {
	out, err := exec.Command("git", "branch", "--show-current").Output()
	if err != nil {
		return "", fmt.Errorf("git branch --show-current: %w", err)
	}
	branch := strings.TrimSpace(string(out))
	if branch == "" {
		return "", errors.New("detached HEAD state — cannot infer PR branch")
	}
	return branch, nil
}

// getPRNumber queries the GitHub API for the open PR number for the given branch.
func getPRNumber(ctx context.Context, token, owner, repo, branch string) (int, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/pulls?head=%s:%s&state=open&per_page=1",
		githubAPIBase, owner, repo, owner, branch)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("building PR list request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("listing PRs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("GitHub API returned %d listing PRs", resp.StatusCode)
	}

	var prs []struct {
		Number int `json:"number"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&prs); err != nil {
		return 0, fmt.Errorf("decoding PR list: %w", err)
	}
	if len(prs) == 0 {
		return 0, ErrNoPR
	}
	return prs[0].Number, nil
}

// buildPositionMap returns a map from new-file line numbers to GitHub diff positions.
// GitHub positions are 1-indexed from the first line after each @@ hunk header,
// and the counter continues across multiple hunks within a file.
// See: https://docs.github.com/en/rest/pulls/reviews
func buildPositionMap(content string) map[int]int {
	posMap := make(map[int]int)
	position := 0
	newLineNum := 0
	inHunk := false

	for _, line := range strings.Split(content, "\n") {
		if m := hunkHeader.FindStringSubmatch(line); m != nil {
			start, _ := strconv.Atoi(m[1])
			newLineNum = start
			inHunk = true
			// The @@ line itself is not counted; position 1 is the line below it.
			continue
		}
		if !inHunk {
			// Skip diff header lines (index, ---, +++) before the first hunk.
			continue
		}

		position++
		switch {
		case strings.HasPrefix(line, "+"):
			// Added line in new file.
			posMap[newLineNum] = position
			newLineNum++
		case strings.HasPrefix(line, "-"):
			// Removed line; new file line number doesn't advance.
		default:
			// Context line (space-prefixed or empty) — present in both old and new file.
			newLineNum++
		}
	}
	return posMap
}

// postPRReview submits a review with inline comments via the GitHub REST API.
func postPRReview(ctx context.Context, token, owner, repo string, prNumber int, comments []reviewComment) error {
	body := struct {
		Event    string          `json:"event"`
		Comments []reviewComment `json:"comments"`
	}{
		Event:    "COMMENT",
		Comments: comments,
	}

	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshaling review: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/pulls/%d/reviews", githubAPIBase, owner, repo, prNumber)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("building review request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("posting review: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("GitHub API returned %d posting review", resp.StatusCode)
	}
	return nil
}
