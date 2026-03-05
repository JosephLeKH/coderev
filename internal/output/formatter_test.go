package output

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseComments_SingleLine(t *testing.T) {
	raw := "[BUG] L5 Nil pointer dereference"
	comments := ParseComments("main.go", raw)
	require.Len(t, comments, 1)
	assert.Equal(t, "main.go", comments[0].File)
	assert.Equal(t, "BUG", comments[0].Severity)
	assert.Equal(t, 5, comments[0].Line)
	assert.Equal(t, "Nil pointer dereference", comments[0].Message)
}

func TestParseComments_MultipleLines(t *testing.T) {
	raw := "[BUG] L1 First issue\n[SECURITY] L10 SQL injection risk\n[PERFORMANCE] L20 Inefficient loop"
	comments := ParseComments("foo.go", raw)
	require.Len(t, comments, 3)
	assert.Equal(t, "BUG", comments[0].Severity)
	assert.Equal(t, 1, comments[0].Line)
	assert.Equal(t, "SECURITY", comments[1].Severity)
	assert.Equal(t, 10, comments[1].Line)
	assert.Equal(t, "PERFORMANCE", comments[2].Severity)
	assert.Equal(t, 20, comments[2].Line)
}

func TestParseComments_SkipsNonMatchingLines(t *testing.T) {
	raw := "Here is the review:\n[BUG] L3 Real issue\nSome explanation text.\n[STYLE] L7 Minor style"
	comments := ParseComments("x.go", raw)
	require.Len(t, comments, 2)
	assert.Equal(t, "BUG", comments[0].Severity)
	assert.Equal(t, "STYLE", comments[1].Severity)
}

func TestParseComments_EmptyResponse(t *testing.T) {
	comments := ParseComments("main.go", "")
	assert.Empty(t, comments)
}

func TestParseComments_NoIssues(t *testing.T) {
	comments := ParseComments("main.go", "No issues found.")
	assert.Empty(t, comments)
}

func TestParseComments_AllSeverities(t *testing.T) {
	raw := "[BUG] L1 bug\n[SECURITY] L2 sec\n[PERFORMANCE] L3 perf\n[STYLE] L4 style\n[NITPICK] L5 nit"
	comments := ParseComments("f.go", raw)
	require.Len(t, comments, 5)
	severities := []string{"BUG", "SECURITY", "PERFORMANCE", "STYLE", "NITPICK"}
	for i, c := range comments {
		assert.Equal(t, severities[i], c.Severity)
		assert.Equal(t, i+1, c.Line)
	}
}

func TestFormatTerminal_ContainsFilenameAndSeverity(t *testing.T) {
	result := FileResult{
		File: "main.go",
		Comments: []ReviewComment{
			{File: "main.go", Severity: "BUG", Line: 5, Message: "Nil pointer"},
		},
	}
	out := FormatTerminal([]FileResult{result})
	assert.Contains(t, out, "main.go")
	assert.Contains(t, out, "BUG")
	assert.Contains(t, out, "Nil pointer")
	assert.Contains(t, out, "L5")
}

func TestFormatTerminal_NoIssues(t *testing.T) {
	result := FileResult{File: "clean.go", Comments: nil}
	out := FormatTerminal([]FileResult{result})
	// Should still mention the file
	assert.Contains(t, out, "clean.go")
}

func TestFormatTerminal_MultipleFiles(t *testing.T) {
	results := []FileResult{
		{File: "a.go", Comments: []ReviewComment{{File: "a.go", Severity: "BUG", Line: 1, Message: "issue"}}},
		{File: "b.go", Comments: nil},
	}
	out := FormatTerminal(results)
	assert.Contains(t, out, "a.go")
	assert.Contains(t, out, "b.go")
}

func TestFormatJSON_ValidJSON(t *testing.T) {
	results := []FileResult{
		{
			File: "main.go",
			Comments: []ReviewComment{
				{File: "main.go", Severity: "BUG", Line: 3, Message: "nil deref"},
			},
		},
	}
	out, err := FormatJSON(results)
	require.NoError(t, err)
	var parsed []FileResult
	err = json.Unmarshal([]byte(out), &parsed)
	require.NoError(t, err)
	require.Len(t, parsed, 1)
	assert.Equal(t, "main.go", parsed[0].File)
	require.Len(t, parsed[0].Comments, 1)
	assert.Equal(t, "BUG", parsed[0].Comments[0].Severity)
}

func TestFormatJSON_EmptyResults(t *testing.T) {
	out, err := FormatJSON([]FileResult{})
	require.NoError(t, err)
	assert.True(t, strings.Contains(out, "[]") || strings.Contains(out, "[ ]"))
}
