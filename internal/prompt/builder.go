package prompt

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/JosephLeKH/coderev/internal/config"
	"github.com/JosephLeKH/coderev/internal/git"
)

// BuildPrompt assembles a review prompt from a FileChunk and Config.
// The prompt instructs the model to output findings in the format:
//
//	[SEVERITY] L<line> <description>
//
// Supported severities: BUG, SECURITY, PERFORMANCE.
func BuildPrompt(chunk git.FileChunk, cfg *config.Config) string {
	var sb strings.Builder

	sb.WriteString("You are an expert code reviewer. Review the following unified diff and identify issues.\n\n")

	sb.WriteString("Output each finding on its own line in exactly this format:\n")
	sb.WriteString("  [SEVERITY] L<line_number> <short description>\n\n")
	sb.WriteString("Valid severities: [BUG] [SECURITY] [PERFORMANCE]\n\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- Only report a DEFECT: a bug, a security hole, a correctness issue, or a dangerous pattern.\n")
	sb.WriteString("- Do NOT report suggestions, preferences, style opinions, or ways the code could be improved.\n")
	sb.WriteString("- Do NOT report anything about removed code unless the removal itself introduces a defect.\n")
	sb.WriteString("- Do NOT report code that was added if it is correct and works — even if you'd write it differently.\n")
	sb.WriteString("- Do NOT speculate. If you cannot prove a defect from the diff alone, say nothing.\n")
	sb.WriteString("- Do NOT describe what the diff does. Only report defects.\n")
	sb.WriteString("- Use the line number of the added (+) or removed (-) line in the diff.\n")
	sb.WriteString("- If there are no defects, output nothing.\n\n")

	if len(cfg.Focus) > 0 {
		sb.WriteString(fmt.Sprintf("Focus on: %s\n\n", strings.Join(cfg.Focus, ", ")))
	}

	ext := filepath.Ext(chunk.Filename)
	if hint, ok := cfg.LanguageHints[ext]; ok && hint != "" {
		sb.WriteString(fmt.Sprintf("Language note: %s\n\n", hint))
	}

	sb.WriteString(fmt.Sprintf("File: %s\n", chunk.Filename))
	sb.WriteString("```diff\n")
	sb.WriteString(chunk.Content)
	sb.WriteString("\n```\n")

	return sb.String()
}
