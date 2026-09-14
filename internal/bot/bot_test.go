package bot

import (
	"bot-api/internal/config"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestQueryInputGetPathCacheIsDeterministic(t *testing.T) {
	input := &QueryInput{
		SystemPrompt: "system",
		UserPrompt:   "question",
		Schema:       []byte(`{"type":"object"}`),
		Temperature:  1,
	}

	path := input.GetPathCache()
	if path == "" {
		t.Fatal("GetPathCache() returned an empty path")
	}
	if path != input.GetPathCache() {
		t.Fatal("GetPathCache() was not deterministic")
	}
	if !strings.HasSuffix(path, ".cache") {
		t.Fatalf("path = %q, want .cache suffix", path)
	}
	if len(strings.TrimSuffix(path, ".cache")) != 64 {
		t.Fatalf("path = %q, want SHA-256 filename", path)
	}
}

func TestQueryInputGetPathCacheChangesWithContent(t *testing.T) {
	first := (&QueryInput{UserPrompt: "question"}).GetPathCache()
	second := (&QueryInput{UserPrompt: "different question"}).GetPathCache()

	if first == second {
		t.Fatalf("different inputs produced the same path: %q", first)
	}
}

func TestGetUserPromptReplacesSingleURLWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte("contenido remoto"))
	}))
	t.Cleanup(server.Close)

	bot := &Bot{Config: &config.BotConfig{Fetch: true}}
	prompt := bot.getUserPrompt("  " + server.URL + "  ")

	if prompt != "contenido remoto" {
		t.Fatalf("prompt = %q, want fetched body", prompt)
	}
}

func TestGetUserPromptDoesNotFetchWhenAskIsNotOnlyURL(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		called = true
	}))
	t.Cleanup(server.Close)

	ask := "Consulta " + server.URL
	bot := &Bot{Config: &config.BotConfig{Fetch: true}}

	if prompt := bot.getUserPrompt(ask); prompt != ask {
		t.Fatalf("prompt = %q, want %q", prompt, ask)
	}
	if called {
		t.Fatal("fetch request was made when ask was not only a URL")
	}
}

func TestGetUserPromptDoesNotFetchWhenDisabled(t *testing.T) {
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		called = true
	}))
	t.Cleanup(server.Close)

	ask := "Consulta " + server.URL
	bot := &Bot{Config: &config.BotConfig{Fetch: false}}

	if prompt := bot.getUserPrompt(ask); prompt != ask {
		t.Fatalf("prompt = %q, want %q", prompt, ask)
	}
	if called {
		t.Fatal("fetch request was made with Fetch disabled")
	}
}
