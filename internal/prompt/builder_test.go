package prompt

import (
	"strings"
	"testing"

	"github.com/JosephLeKH/coderev/internal/config"
	"github.com/JosephLeKH/coderev/internal/git"
	"github.com/stretchr/testify/assert"
)

func TestBuildPrompt_ContainsDiff(t *testing.T) {
	chunk := git.FileChunk{
		Filename: "main.go",
		Content:  "+fmt.Println(\"hello\")\n-fmt.Println(\"world\")",
	}
	result := BuildPrompt(chunk, &config.Config{})
	assert.Contains(t, result, chunk.Content)
	assert.Contains(t, result, "main.go")
}

func TestBuildPrompt_ContainsOutputFormat(t *testing.T) {
	chunk := git.FileChunk{Filename: "foo.go", Content: "+x := 1"}
	result := BuildPrompt(chunk, &config.Config{})
	// Must instruct the model on the expected output format
	assert.Contains(t, result, "[BUG]")
	assert.Contains(t, result, "[SECURITY]")
}

func TestBuildPrompt_WithFocusAreas(t *testing.T) {
	chunk := git.FileChunk{Filename: "foo.go", Content: "+x := 1"}
	cfg := &config.Config{Focus: []string{"bugs", "security", "performance"}}
	result := BuildPrompt(chunk, cfg)
	assert.Contains(t, result, "bugs")
	assert.Contains(t, result, "security")
	assert.Contains(t, result, "performance")
}

func TestBuildPrompt_WithLanguageHint(t *testing.T) {
	chunk := git.FileChunk{Filename: "handler.go", Content: "+return nil"}
	cfg := &config.Config{
		LanguageHints: map[string]string{
			".go": "Follow standard Go error handling patterns",
		},
	}
	result := BuildPrompt(chunk, cfg)
	assert.Contains(t, result, "Follow standard Go error handling patterns")
}

func TestBuildPrompt_NoLanguageHintForUnknownExt(t *testing.T) {
	chunk := git.FileChunk{Filename: "data.csv", Content: "+a,b,c"}
	cfg := &config.Config{
		LanguageHints: map[string]string{
			".go": "Follow standard Go error handling patterns",
		},
	}
	result := BuildPrompt(chunk, cfg)
	assert.NotContains(t, result, "Follow standard Go error handling patterns")
}

func TestBuildPrompt_EmptyFocusNoFocusLine(t *testing.T) {
	chunk := git.FileChunk{Filename: "foo.go", Content: "+x := 1"}
	result := BuildPrompt(chunk, &config.Config{})
	// Should not have a dangling "Focus on:" with nothing after it
	lines := strings.Split(result, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Focus on:") {
			assert.NotEqual(t, "Focus on:", trimmed, "empty focus line should not appear")
		}
	}
}

func TestBuildPrompt_TSLanguageHint(t *testing.T) {
	chunk := git.FileChunk{Filename: "app.ts", Content: "+const x = 1"}
	cfg := &config.Config{
		LanguageHints: map[string]string{
			".ts": "Prefer strict TypeScript types",
		},
	}
	result := BuildPrompt(chunk, cfg)
	assert.Contains(t, result, "Prefer strict TypeScript types")
}

func TestBuildPrompt_MultipleLanguageHintsOnlyMatchingApplied(t *testing.T) {
	chunk := git.FileChunk{Filename: "main.go", Content: "+x := 1"}
	cfg := &config.Config{
		LanguageHints: map[string]string{
			".go": "Go hint",
			".ts": "TS hint",
		},
	}
	result := BuildPrompt(chunk, cfg)
	assert.Contains(t, result, "Go hint")
	assert.NotContains(t, result, "TS hint")
}
