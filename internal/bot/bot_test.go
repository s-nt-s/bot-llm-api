package bot

import (
	"bot-api/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
