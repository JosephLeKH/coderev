package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// DefaultModel is the Bedrock model ID used when none is specified in .coderev.yaml.
// DefaultModel uses the cross-region inference profile format required by Bedrock
// on-demand throughput (prefix "us." routes to the nearest available region).
const DefaultModel = "us.anthropic.claude-3-5-haiku-20241022-v1:0"

// Config holds the loaded .coderev.yaml configuration.
type Config struct {
	Model         string            `yaml:"model"`
	Focus         []string          `yaml:"focus"`
	Ignore        []string          `yaml:"ignore"`
	LanguageHints map[string]string `yaml:"language_hints"`
}

// LoadConfig loads .coderev.yaml from repoRoot.
// Returns a Config with sensible defaults if the file does not exist.
func LoadConfig(repoRoot string) (*Config, error) {
	cfg := &Config{
		Model: DefaultModel,
	}

	path := filepath.Join(repoRoot, ".coderev.yaml")
	const maxConfigBytes = 1 << 20 // 1 MiB

	fi, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, fmt.Errorf("reading .coderev.yaml: %w", err)
	}
	if fi.Size() > maxConfigBytes {
		return nil, fmt.Errorf(".coderev.yaml exceeds maximum allowed size of 1 MiB")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading .coderev.yaml: %w", err)
	}

	if len(data) == 0 {
		return cfg, nil
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing .coderev.yaml: %w", err)
	}

	if cfg.Model == "" {
		cfg.Model = DefaultModel
	}
	return cfg, nil
}
