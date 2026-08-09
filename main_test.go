package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleRequest(t *testing.T) {
	tests := []struct {
		name        string
		method      string
		target      string
		body        string
		contentType string
		status      int
		want        string
	}{
		{name: "query GET", method: http.MethodGet, target: "/blas/query?ask=hello", status: http.StatusOK, want: `"bot":"blas"`},
		{name: "chat POST JSON", method: http.MethodPost, target: "/blas/chat/session-1", body: `{"ask":"help"}`, contentType: "application/json", status: http.StatusOK, want: `"user":"session-1"`},
		{name: "query POST form", method: http.MethodPost, target: "/blas/query", body: "ask=hello", contentType: "application/x-www-form-urlencoded", status: http.StatusOK, want: `"ask":"hello"`},
		{name: "missing ask", method: http.MethodGet, target: "/blas/query", status: http.StatusBadRequest, want: "ask is required"},
		{name: "missing user", method: http.MethodGet, target: "/blas/chat/", status: http.StatusBadRequest, want: "user is required"},
		{name: "empty bot", method: http.MethodGet, target: "//query?ask=hola", status: http.StatusBadRequest, want: "bot is required"},
		{name: "unknown bot", method: http.MethodGet, target: "/unknown/query?ask=hello", status: http.StatusNotFound, want: "bot does not exist"},
		{name: "unknown user", method: http.MethodGet, target: "/blas/chat/unknown?ask=hello", status: http.StatusNotFound, want: "user does not exist"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(test.method, test.target, strings.NewReader(test.body))
			req.Header.Set("Content-Type", test.contentType)
			recorder := httptest.NewRecorder()

			handleRequest(recorder, req)

			if recorder.Code != test.status {
				t.Fatalf("status = %d, want %d", recorder.Code, test.status)
			}
			if !strings.Contains(recorder.Body.String(), test.want) {
				t.Fatalf("body = %q, want it to contain %q", recorder.Body.String(), test.want)
			}
		})
	}
}

func TestValidateBotReturnsConfiguration(t *testing.T) {
	root := t.TempDir()
	botDirectory = root
	botName := "configured"
	path := filepath.Join(root, botName, "0.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("name: Blas\nprofile: helpful assistant\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	config, err := validateBot(botName)
	if err != nil {
		t.Fatalf("validateBot() error = %v", err)
	}
	if config == nil || config.Name != "Blas" || config.Profile != "helpful assistant" {
		t.Fatalf("validateBot() config = %+v, want name and profile from YAML", config)
	}
}

func TestValidateBotConfiguration(t *testing.T) {
	root := t.TempDir()
	botDirectory = root

	tests := []struct {
		name   string
		body   string
		status string
	}{
		{name: "invalid YAML", body: "name: [", status: "bot configuration is not valid YAML"},
		{name: "missing name", body: "profile: valid", status: "bot configuration requires a non-empty name"},
		{name: "empty profile", body: "name: Valid\nprofile: \"  \"", status: "bot configuration requires a non-empty profile"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(root, strings.ToLower(strings.ReplaceAll(test.name, " ", "-")), "0.yaml")
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(test.body), 0o644); err != nil {
				t.Fatal(err)
			}
			if _, err := validateBot(filepath.Base(filepath.Dir(path))); err == nil || err.Error() != test.status {
				t.Fatalf("error = %v, want %q", err, test.status)
			}
		})
	}
}

func TestValidateUserReturnsConfiguration(t *testing.T) {
	root := t.TempDir()
	botDirectory = root
	path := filepath.Join(root, "blas", "session-1.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("name: Session One\nprofile: helpful assistant\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	config, err := validateUser("blas", "session-1")
	if err != nil {
		t.Fatalf("validateUser() error = %v", err)
	}
	if config == nil || config.Name != "Session One" || config.Profile != "helpful assistant" {
		t.Fatalf("validateUser() config = %+v, want name and profile from YAML", config)
	}
}
