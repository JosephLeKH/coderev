package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Defaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, DefaultModel, cfg.Model)
	assert.Empty(t, cfg.Focus)
	assert.Empty(t, cfg.Ignore)
	assert.Empty(t, cfg.LanguageHints)
}

func TestLoadConfig_FullConfig(t *testing.T) {
	dir := t.TempDir()
	content := `
model: anthropic.claude-3-5-sonnet-20241022-v2:0
focus:
  - bugs
  - security
ignore:
  - "*.lock"
  - "vendor/**"
language_hints:
  .go: "Follow standard Go error handling patterns"
  .ts: "Prefer strict TypeScript types"
`
	err := os.WriteFile(filepath.Join(dir, ".coderev.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, "anthropic.claude-3-5-sonnet-20241022-v2:0", cfg.Model)
	assert.Equal(t, []string{"bugs", "security"}, cfg.Focus)
	assert.Equal(t, []string{"*.lock", "vendor/**"}, cfg.Ignore)
	assert.Equal(t, "Follow standard Go error handling patterns", cfg.LanguageHints[".go"])
	assert.Equal(t, "Prefer strict TypeScript types", cfg.LanguageHints[".ts"])
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, ".coderev.yaml"), []byte("invalid: yaml: ["), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(dir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), ".coderev.yaml")
}

func TestLoadConfig_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	err := os.WriteFile(filepath.Join(dir, ".coderev.yaml"), []byte(""), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(dir)
	require.NoError(t, err)
	assert.Equal(t, DefaultModel, cfg.Model)
}

func TestLoadConfig_PartialConfig(t *testing.T) {
	dir := t.TempDir()
	content := `focus:
  - performance
`
	err := os.WriteFile(filepath.Join(dir, ".coderev.yaml"), []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(dir)
	require.NoError(t, err)
	// Model should fall back to default when not specified
	assert.Equal(t, DefaultModel, cfg.Model)
	assert.Equal(t, []string{"performance"}, cfg.Focus)
}
