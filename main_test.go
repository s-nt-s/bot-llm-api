package main

import (
	"bot-api/types"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleRequestTreatsUserAsOptional(t *testing.T) {
	withBotRoot(t, func(root string) {
		writeMarkdownConfig(t, filepath.Join(root, "bot", "blas", "_.md"), "---\nname: Blas\n---\nHelpful bot\n")

		req := httptest.NewRequest(http.MethodGet, "/blas/query?ask=hello", nil)
		recorder := httptest.NewRecorder()

		handleRequest(recorder, req)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
		}
		if !strings.Contains(recorder.Body.String(), `"bot":"Blas"`) {
			t.Fatalf("body = %q, want it to contain the bot name", recorder.Body.String())
		}
		if strings.Contains(recorder.Body.String(), `"user":`) {
			t.Fatalf("body = %q, want user to be omitted when it is not provided", recorder.Body.String())
		}
	})
}

func TestHandleRequestUsesConfiguredUserWhenProvided(t *testing.T) {
	withBotRoot(t, func(root string) {
		writeMarkdownConfig(t, filepath.Join(root, "bot", "blas", "_.md"), "---\nname: Blas\n---\nHelpful bot\n")
		writeMarkdownConfig(t, filepath.Join(root, "bot", "blas", "session-1.md"), "---\nname: Session One\n---\nFriendly user\n")

		req := httptest.NewRequest(http.MethodGet, "/blas/chat/session-1?ask=hello", nil)
		recorder := httptest.NewRecorder()

		handleRequest(recorder, req)

		if recorder.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
		}
		if !strings.Contains(recorder.Body.String(), `"user":"Session One"`) {
			t.Fatalf("body = %q, want it to contain the configured user name", recorder.Body.String())
		}
	})
}

func TestNewBotLoadsMarkdownFrontMatter(t *testing.T) {
	withBotRoot(t, func(root string) {
		writeMarkdownConfig(t, filepath.Join(root, "bot", "blas", "_.md"), "---\nname: Blas\n---\nHelpful bot\n")

		config, err := types.NewBot("blas")
		if err != nil {
			t.Fatalf("NewBot() error = %v", err)
		}
		if config.Name != "Blas" {
			t.Fatalf("config.Name = %q, want %q", config.Name, "Blas")
		}
		if config.Profile != "Helpful bot" {
			t.Fatalf("config.Profile = %q, want %q", config.Profile, "Helpful bot")
		}
	})
}

func TestNewUserLoadsMarkdownFrontMatter(t *testing.T) {
	withBotRoot(t, func(root string) {
		writeMarkdownConfig(t, filepath.Join(root, "bot", "blas", "_.md"), "---\nname: Blas\n---\nHelpful bot\n")
		writeMarkdownConfig(t, filepath.Join(root, "bot", "blas", "session-1.md"), "---\nname: Session One\n---\nFriendly user\n")

		bot, err := types.NewBot("blas")
		if err != nil {
			t.Fatalf("NewBot() error = %v", err)
		}
		user, err := types.NewUser(bot, "session-1")
		if err != nil {
			t.Fatalf("NewUser() error = %v", err)
		}
		if user.Name != "Session One" {
			t.Fatalf("user.Name = %q, want %q", user.Name, "Session One")
		}
		if user.Profile != "Friendly user" {
			t.Fatalf("user.Profile = %q, want %q", user.Profile, "Friendly user")
		}
	})
}

func withBotRoot(t *testing.T, fn func(root string)) {
	t.Helper()

	root := t.TempDir()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	fn(root)
}

func writeMarkdownConfig(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}
