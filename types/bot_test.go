package types

import (
	"net/http"
	"testing"
)

type stubProvider struct {
	name   string
	reply  string
	status int
	ready  bool
}

func (p *stubProvider) Name() string {
	return p.name
}

func (p *stubProvider) IsReady() bool {
	return p.ready
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
	originalProviders := providers.providers
	providers.providers = nil
	t.Cleanup(func() { providers.providers = originalProviders })

	bot := &Bot{
		Config: &BotConfig{Name: "bot"},
		User:   &UserConfig{Name: "user"},
	}

	providers.providers = append(providers.providers,
		&stubProvider{name: "quota-provider", status: http.StatusTooManyRequests, ready: true},
		&stubProvider{name: "fallback-provider", reply: "ok", status: http.StatusOK, ready: true},
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
	originalProviders := providers.providers
	providers.providers = nil
	t.Cleanup(func() { providers.providers = originalProviders })

	bot := &Bot{
		Config: &BotConfig{Name: "bot"},
		User:   &UserConfig{Name: "user"},
	}

	providers.providers = append(providers.providers,
		&stubProvider{name: "provider-1", status: http.StatusTooManyRequests, ready: true},
		&stubProvider{name: "provider-2", status: http.StatusTooManyRequests, ready: true},
	)

	msg := bot.DoChat("hello")
	if msg.Status != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want %d", msg.Status, http.StatusTooManyRequests)
	}
	if msg.Error == "" {
		t.Fatal("expected error message when all providers are exhausted")
	}
}

func TestProviderRegistrySnapshotOnlyReadyProviders(t *testing.T) {
	originalProviders := providers.providers
	providers.providers = nil
	t.Cleanup(func() { providers.providers = originalProviders })

	providers.providers = append(providers.providers,
		&stubProvider{name: "ready-1", ready: true},
		&stubProvider{name: "not-ready", ready: false},
		&stubProvider{name: "ready-2", ready: true},
	)

	got := providersSnapshot(nil)
	want := []string{"ready-1", "ready-2"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, provider := range got {
		if provider.Name() != want[i] {
			t.Fatalf("provider[%d].Name() = %q, want %q", i, provider.Name(), want[i])
		}
	}
}

func TestProviderRegistrySnapshotRotatesStartingAtCurrent(t *testing.T) {
	originalProviders := providers.providers
	providers.providers = nil
	t.Cleanup(func() { providers.providers = originalProviders })

	p1 := &stubProvider{name: "provider-1", ready: true}
	p2 := &stubProvider{name: "provider-2", ready: true}
	p3 := &stubProvider{name: "provider-3", ready: true}
	providers.providers = append(providers.providers, p1, p2, p3)

	current := LLMProvider(p2)
	got := providersSnapshot(&current)
	want := []string{"provider-2", "provider-3", "provider-1"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, provider := range got {
		if provider.Name() != want[i] {
			t.Fatalf("provider[%d].Name() = %q, want %q", i, provider.Name(), want[i])
		}
	}
}

func TestProviderRegistrySnapshotRotatesStartingAtNextReadyWhenCurrentNotReady(t *testing.T) {
	originalProviders := providers.providers
	providers.providers = nil
	t.Cleanup(func() { providers.providers = originalProviders })

	p1 := &stubProvider{name: "provider-1", ready: true}
	p2 := &stubProvider{name: "provider-2", ready: false}
	p3 := &stubProvider{name: "provider-3", ready: true}
	p4 := &stubProvider{name: "provider-4", ready: true}
	providers.providers = append(providers.providers, p1, p2, p3, p4)

	current := LLMProvider(p2)
	got := providersSnapshot(&current)
	want := []string{"provider-3", "provider-4", "provider-1"}
	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i, provider := range got {
		if provider.Name() != want[i] {
			t.Fatalf("provider[%d].Name() = %q, want %q", i, provider.Name(), want[i])
		}
	}
}

func TestBotDoChatHandlesMissingUser(t *testing.T) {
	originalProviders := providers.providers
	providers.providers = nil
	t.Cleanup(func() { providers.providers = originalProviders })

	providers.providers = append(providers.providers, &stubProvider{name: "provider", reply: "ok", status: http.StatusOK, ready: true})

	bot := &Bot{
		Config: &BotConfig{Name: "bot", Profile: "You are helpful"},
		User:   nil,
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("DoChat panicked: %v", r)
		}
	}()

	msg := bot.DoChat("hello")
	if msg.Status != http.StatusOK {
		t.Fatalf("status = %d, want %d", msg.Status, http.StatusOK)
	}
	if msg.Reply != "ok" {
		t.Fatalf("reply = %q, want %q", msg.Reply, "ok")
	}
}
