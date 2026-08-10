package types

import (
	"errors"
	"net/http"
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

func (p *stubProvider) Query(ask string, bot *BotConfig, user *UserConfig) (string, error) {
	if p.err != nil {
		return "", p.err
	}
	return p.reply, nil
}

func (p *stubProvider) Chat(ask string, bot *BotConfig, user *UserConfig) (string, error) {
	if p.err != nil {
		return "", p.err
	}
	return p.reply, nil
}

func (p *stubProvider) Conversation(bot *BotConfig, user *UserConfig) []ConversationMessage {
	return nil
}

func (p *stubProvider) PopConversation(bot *BotConfig, user *UserConfig) []ConversationMessage {
	return nil
}

func (p *stubProvider) ClearConversation(bot *BotConfig, user *UserConfig) {}

func TestBotFallsBackWhenCurrentProviderExhaustsQuota(t *testing.T) {
	bot := &Bot{
		Config: &BotConfig{Name: "bot"},
		User:   &UserConfig{Name: "user"},
		Providers: []LLMProvider{
			&stubProvider{name: "quota-provider", err: &QuotaExceededError{Provider: "quota-provider", Cause: errors.New("quota exceeded")}},
			&stubProvider{name: "fallback-provider", reply: "ok"},
		},
	}

	msg := bot.DoQuery("hello")
	if msg.Status != http.StatusOK {
		t.Fatalf("status = %d, want %d", msg.Status, http.StatusOK)
	}
	if msg.Reply != "ok" {
		t.Fatalf("reply = %q, want %q", msg.Reply, "ok")
	}
	if bot.queryProvider != 1 {
		t.Fatalf("queryProvider = %d, want %d", bot.queryProvider, 1)
	}
}

func TestBotReturnsErrorWhenAllProvidersExhaustQuota(t *testing.T) {
	bot := &Bot{
		Config: &BotConfig{Name: "bot"},
		User:   &UserConfig{Name: "user"},
		Providers: []LLMProvider{
			&stubProvider{name: "provider-1", err: &QuotaExceededError{Provider: "provider-1", Cause: errors.New("quota exceeded")}},
			&stubProvider{name: "provider-2", err: &QuotaExceededError{Provider: "provider-2", Cause: errors.New("quota exceeded")}},
		},
	}

	msg := bot.DoChat("hello")
	if msg.Status != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", msg.Status, http.StatusTooManyRequests)
	}
	if msg.Error == "" {
		t.Fatal("expected error message when all providers are exhausted")
	}
}
