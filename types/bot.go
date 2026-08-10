package types

import (
	"errors"
	"fmt"
	"net/http"
	"sync"
)

// LLMProvider defines the minimal interface any LLM backend must implement.
// Query and Chat can have different internal implementations, but the bot pool uses
// the same fallback and quota-handling flow for both.
type LLMProvider interface {
	Name() string
	Query(ask string, bot *BotConfig, user *UserConfig) (string, error)
	Chat(ask string, bot *BotConfig, user *UserConfig) (string, error)
}

// QuotaExceededError is returned when a provider refuses the request due to quota exhaustion.
type QuotaExceededError struct {
	Provider string
	Cause    error
}

func (e *QuotaExceededError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause == nil {
		return fmt.Sprintf("provider %q exhausted its quota", e.Provider)
	}
	return fmt.Sprintf("provider %q exhausted its quota: %v", e.Provider, e.Cause)
}

func (e *QuotaExceededError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

var (
	providerMu sync.RWMutex
	providers  = map[string]func() LLMProvider{}
)

// RegisterProvider registers a provider factory so it can be used by the bot pool.
func RegisterProvider(name string, factory func() LLMProvider) {
	providerMu.Lock()
	defer providerMu.Unlock()
	providers[name] = factory
}

// UnregisterProvider removes a provider factory from the registry.
func UnregisterProvider(name string) {
	providerMu.Lock()
	defer providerMu.Unlock()
	delete(providers, name)
}

// DefaultProviders returns the registered provider factories in a stable order.
func DefaultProviders() []LLMProvider {
	providerMu.RLock()
	defer providerMu.RUnlock()

	ordered := make([]LLMProvider, 0, len(providers))
	for _, factory := range providers {
		ordered = append(ordered, factory())
	}
	return ordered
}

// Bot keeps a shared configuration and user, and can try multiple LLM providers in order.
type Bot struct {
	Config *BotConfig
	User   *UserConfig

	Providers      []LLMProvider
	activeProvider int
}

func (c *Bot) DoChat(ask string) *Message {
	return c.ask(ask, "chat")
}

func (c *Bot) DoQuery(md string) *Message {
	return c.ask(md, "query")
}

func (c *Bot) ask(ask string, mode string) *Message {
	if len(c.Providers) == 0 {
		return &Message{
			Status: http.StatusServiceUnavailable,
			Error:  "no LLM providers configured",
		}
	}

	var lastErr error
	for i := 0; i < len(c.Providers); i++ {
		providerIndex := (c.activeProvider + i) % len(c.Providers)
		provider := c.Providers[providerIndex]

		var (
			reply string
			err   error
		)
		if mode == "chat" {
			reply, err = provider.Chat(ask, c.Config, c.User)
		} else {
			reply, err = provider.Query(ask, c.Config, c.User)
		}
		if err == nil {
			c.activeProvider = providerIndex
			return &Message{
				Reply:  reply,
				Status: http.StatusOK,
			}
		}

		var quotaErr *QuotaExceededError
		if errors.As(err, &quotaErr) {
			lastErr = err
			continue
		}

		c.activeProvider = providerIndex
		return &Message{
			Status: http.StatusBadGateway,
			Error:  fmt.Sprintf("provider %q failed: %v", provider.Name(), err),
		}
	}

	if lastErr != nil {
		return &Message{
			Status: http.StatusTooManyRequests,
			Error:  lastErr.Error(),
		}
	}

	return &Message{
		Status: http.StatusBadGateway,
		Error:  "all providers failed",
	}
}
