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
	name   string
	reply  string
	status int
}

func (p *stubProvider) Name() string {
	return p.name
}

func (p *stubProvider) IsReady() bool {
	return true
}

func (p *stubProvider) Query(systemPrompt string, userPrompt string) *types.Message {
	return &types.Message{
		Reply:  p.reply,
		Status: p.status,
	}
}

func (p *stubProvider) Chat(conversationKey types.ConversationKey, systemPrompt string, userPrompt string, history []types.ConversationMessage) *types.Message {
	return &types.Message{
		Reply:  p.reply,
		Status: p.status,
	}
}

func TestHandleRequestTreatsUserAsOptional(t *testing.T) {
	withBotRoot(t, func(root string) {
		writeMarkdownConfig(t, filepath.Join(root, "bot", "blas", "_.md"), "---\nname: Blas\n---\nHelpful bot\n")

		types.RegisterProvider(&stubProvider{name: "test-provider", reply: "ok", status: http.StatusOK})
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

		botConfig, err := types.NewBotConfig("blas")
		if err != nil {
			t.Fatalf("NewBotConfig() error = %v", err)
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

		botConfig, err := types.NewBotConfig("blas")
		if err != nil {
			t.Fatalf("NewBotConfig() error = %v", err)
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

		config, err := types.NewBotConfig("blas")
		if err != nil {
			t.Fatalf("NewBotConfig() error = %v", err)
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

		bot, err := types.NewBotConfig("blas")
		if err != nil {
			t.Fatalf("NewBotConfig() error = %v", err)
		}
		user, err := types.NewUserConfig(bot, "session-1")
		if err != nil {
			t.Fatalf("NewUserConfig() error = %v", err)
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
