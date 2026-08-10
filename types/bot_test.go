package types

import (
	"net/http"
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

func (p *stubProvider) Query(systemPrompt string, userPrompt string) *Message {
	return &Message{
		Reply:  p.reply,
		Status: p.status,
	}
}

func (p *stubProvider) Chat(conversationKey ConversationKey, systemPrompt string, userPrompt string, history []ConversationMessage) *Message {
	return &Message{
		Reply:  p.reply,
		Status: p.status,
	}
}

func TestBotFallsBackWhenCurrentProviderExhaustsQuota(t *testing.T) {
	originalProviders := PROVIDERS
	PROVIDERS = nil
	t.Cleanup(func() { PROVIDERS = originalProviders })

	bot := &Bot{
		Config: &BotConfig{Name: "bot"},
		User:   &UserConfig{Name: "user"},
	}

	PROVIDERS = append(PROVIDERS,
		&stubProvider{name: "quota-provider", status: http.StatusTooManyRequests},
		&stubProvider{name: "fallback-provider", reply: "ok", status: http.StatusOK},
	)

	msg := bot.DoQuery("hello")
	if msg.Status != http.StatusOK {
		t.Fatalf("status = %d, want %d", msg.Status, http.StatusOK)
	}
	if msg.Reply != "ok" {
		t.Fatalf("reply = %q, want %q", msg.Reply, "ok")
	}
}

func TestBotReturnsErrorWhenAllProvidersExhaustQuota(t *testing.T) {
	originalProviders := PROVIDERS
	PROVIDERS = nil
	t.Cleanup(func() { PROVIDERS = originalProviders })

	bot := &Bot{
		Config: &BotConfig{Name: "bot"},
		User:   &UserConfig{Name: "user"},
	}

	PROVIDERS = append(PROVIDERS,
		&stubProvider{name: "provider-1", status: http.StatusTooManyRequests},
		&stubProvider{name: "provider-2", status: http.StatusTooManyRequests},
	)

	msg := bot.DoChat("hello")
	if msg.Status != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", msg.Status, http.StatusTooManyRequests)
	}
	if msg.Error == "" {
		t.Fatal("expected error message when all providers are exhausted")
	}
}
