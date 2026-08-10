package types

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseGeminiAPIKeys(t *testing.T) {
	t.Setenv("GEMINI", "  key-one   key-two\tkey-three  ")

	got := parseGeminiAPIKeys()
	want := []string{"key-one", "key-two", "key-three"}
	if len(got) != len(want) {
		t.Fatalf("len(parseGeminiAPIKeys()) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("parseGeminiAPIKeys()[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestGeminiProviderChatUsesPreviousInteractionID(t *testing.T) {
	var received map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"response":{"text":"ok"}}`))
	}))
	defer server.Close()

	provider := &geminiProvider{apiKey: "abc", endpoint: server.URL, client: server.Client(), conversationStates: map[ConversationKey]string{}}
	bot := &BotConfig{Name: "bot", Profile: "You are helpful"}
	user := &UserConfig{Name: "user"}
	conversationKey := ConversationKey{botName: bot.Name, userName: user.Name}
	provider.storePreviousInteractionID(conversationKey, "prev-1")

	r := provider.Chat(conversationKey, bot.Profile, "hello", nil)
	if r.Status != http.StatusOK {
		t.Fatalf("Chat() status = %d, error = %v", r.Status, r.Error)
	}

	if received["previous_interaction_id"] != "prev-1" {
		t.Fatalf("previous_interaction_id = %#v, want %q", received["previous_interaction_id"], "prev-1")
	}
}

func TestGeminiProviderParsesCandidatePartsText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"parsed from parts"}]}}]}`))
	}))
	defer server.Close()

	provider := &geminiProvider{apiKey: "abc", endpoint: server.URL, client: server.Client(), conversationStates: map[ConversationKey]string{}}
	message := provider.Query("You are helpful", "hello")
	if message.Status != http.StatusOK {
		t.Fatalf("Query() status = %d, error = %v", message.Status, message.Error)
	}
	if message.Reply != "parsed from parts" {
		t.Fatalf("Query() reply = %q, want %q", message.Reply, "parsed from parts")
	}
}
