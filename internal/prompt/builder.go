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
// Supported severities: BUG, SECURITY, PERFORMANCE, STYLE, NITPICK.
func BuildPrompt(chunk git.FileChunk, cfg *config.Config) string {
	var sb strings.Builder

	sb.WriteString("You are an expert code reviewer. Review the following unified diff and identify issues.\n\n")

	sb.WriteString("Output each finding on its own line in exactly this format:\n")
	sb.WriteString("  [SEVERITY] L<line_number> <short description>\n\n")
	sb.WriteString("Valid severities: [BUG] [SECURITY] [PERFORMANCE] [STYLE] [NITPICK]\n\n")
	sb.WriteString("Rules:\n")
	sb.WriteString("- Only report a finding if you are confident it is a problem. If you are unsure, say nothing.\n")
	sb.WriteString("- Do not flag improvements. If a change adds documentation, improves naming, removes dead code,\n")
	sb.WriteString("  or is clearly a cleanup, do not report it — even as NITPICK.\n")
	sb.WriteString("- Do not speculate about intent. If a change could be correct or incorrect depending on context\n")
	sb.WriteString("  you do not have, say nothing.\n")
	sb.WriteString("- Do not describe what the diff does. Only report actual defects.\n")
	sb.WriteString("- Use the line number of the added (+) or removed (-) line in the diff.\n")
	sb.WriteString("- If there are no issues, output nothing.\n\n")

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
