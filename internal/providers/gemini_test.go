package providers

import (
	"bot-api/internal/bot"
	"os"
	"path/filepath"
	"testing"
)

func TestGeminiProviderLoadMergesWithoutOverwriting(t *testing.T) {
	workingDirectory := t.TempDir()
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workingDirectory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDirectory) })

	provider := &GeminiProvider{
		apiKey: "test-key",
		conversationStates: map[bot.ConversationKey]string{
			{BotName: "bot", UserName: "existing"}: "keep-me",
		},
	}
	data := []byte(`[
  {"botName":"bot","userName":"existing","interactionID":"replace-me"},
  {"botName":"bot","userName":"new","interactionID":"load-me"}
]`)
	if err := os.MkdirAll(filepath.Join(workingDirectory, "data/gemini"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(workingDirectory, "data/gemini/test-key.json"), data, 0600); err != nil {
		t.Fatal(err)
	}

	provider.Load()

	if got := provider.conversationStates[bot.ConversationKey{BotName: "bot", UserName: "existing"}]; got != "keep-me" {
		t.Fatalf("existing state was overwritten: got %q", got)
	}
	if got := provider.conversationStates[bot.ConversationKey{BotName: "bot", UserName: "new"}]; got != "load-me" {
		t.Fatalf("new state was not loaded: got %q", got)
	}
}

func TestGeminiProviderLoadInvalidJSONDoesNotChangeStates(t *testing.T) {
	workingDirectory := t.TempDir()
	previousDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(workingDirectory); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previousDirectory) })

	provider := &GeminiProvider{
		apiKey:             "test-key",
		conversationStates: map[bot.ConversationKey]string{},
	}
	if err := os.MkdirAll("data/gemini", 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("data/gemini/test-key.json", []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}

	provider.Load()

	if len(provider.conversationStates) != 0 {
		t.Fatalf("invalid JSON changed states: %#v", provider.conversationStates)
	}
}
