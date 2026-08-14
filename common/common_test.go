package common

import (
	"os"
	"path/filepath"
	"testing"
)

type markdownConfig struct {
	Path    string `yaml:"path"`
	Content string `yaml:"content"`
	Name    string `yaml:"name"`
	Profile string `yaml:"profile"`
}

func (c markdownConfig) SetPath(path string) markdownConfig {
	c.Path = path
	return c
}

func (c markdownConfig) SetMarkdown(content string) markdownConfig {
	c.Content = content
	return c
}

func TestLoadMarkYamlReadsMarkdownFrontMatter(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "example.md")
	content := []byte("---\nname: Example\nprofile: helpful assistant\n---\n# Heading\nBody content\n")
	if err := os.WriteFile(configPath, content, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	config, markdownContent, err := LoadMarkYaml[markdownConfig](configPath)
	if err != nil {
		t.Fatalf("LoadMarkYaml() error = %v", err)
	}
	if config.Path != "" {
		t.Fatalf("Path = %q, want empty value because the loader does not set it automatically", config.Path)
	}
	if markdownContent != "# Heading\nBody content\n" {
		t.Fatalf("Markdown content = %q, want %q", markdownContent, "# Heading\nBody content\n")
	}
	if config.Name != "Example" {
		t.Fatalf("Name = %q, want %q", config.Name, "Example")
	}
	if config.Profile != "helpful assistant" {
		t.Fatalf("Profile = %q, want %q", config.Profile, "helpful assistant")
	}
}
