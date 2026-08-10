package main

import (
	"bot-api/types"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

type stubProvider struct {
	name  string
	reply string
	err   error
}

func (p *stubProvider) Name() string {
	return p.name
}

func (p *stubProvider) Ask(ask string, bot *types.BotConfig, user *types.UserConfig) (string, error) {
	if p.err != nil {
		return "", p.err
	}
	return p.reply, nil
}

func TestHandleRequestTreatsUserAsOptional(t *testing.T) {
	withBotRoot(t, func(root string) {
		writeMarkdownConfig(t, filepath.Join(root, "bot", "blas", "_.md"), "---\nname: Blas\n---\nHelpful bot\n")

		types.RegisterProvider("test-provider", func() types.LLMProvider {
			return &stubProvider{name: "test-provider", reply: "ok"}
		})
		t.Cleanup(func() {
			types.UnregisterProvider("test-provider")
		})

		req := httptest.NewRequest(http.MethodGet, "/blas/query?ask=hello", nil)
		recorder := httptest.NewRecorder()

		handleRequest(recorder, req)

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}

		var msg types.Message
		if err := json.NewDecoder(recorder.Body).Decode(&msg); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if msg.Status != http.StatusOK {
			t.Fatalf("message status = %d, want %d", msg.Status, http.StatusOK)
		}
	})
}

func TestNewOptionalUserReturnsConfiguredUserWhenProvided(t *testing.T) {
	withBotRoot(t, func(root string) {
		writeMarkdownConfig(t, filepath.Join(root, "bot", "blas", "_.md"), "---\nname: Blas\n---\nHelpful bot\n")
		writeMarkdownConfig(t, filepath.Join(root, "bot", "blas", "session-1.md"), "---\nname: Session One\n---\nFriendly user\n")

		botConfig, err := types.NewBot("blas")
		if err != nil {
			t.Fatalf("NewBot() error = %v", err)
		}

		userConfig := types.NewOptionalUser(botConfig, "session-1")
		if userConfig == nil {
			t.Fatal("NewOptionalUser() = nil, want a configured user")
		}
		if userConfig.Name != "Session One" {
			t.Fatalf("userConfig.Name = %q, want %q", userConfig.Name, "Session One")
		}
	})
}

func TestNewOptionalUserReturnsNilWhenUserIsMissing(t *testing.T) {
	withBotRoot(t, func(root string) {
		writeMarkdownConfig(t, filepath.Join(root, "bot", "blas", "_.md"), "---\nname: Blas\n---\nHelpful bot\n")

		botConfig, err := types.NewBot("blas")
		if err != nil {
			t.Fatalf("NewBot() error = %v", err)
		}

		userConfig := types.NewOptionalUser(botConfig, "")
		if userConfig != nil {
			t.Fatalf("NewOptionalUser() = %+v, want nil when no user is provided", userConfig)
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
