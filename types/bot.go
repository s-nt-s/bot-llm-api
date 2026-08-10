package types

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// LLMProvider defines the minimal interface any LLM backend must implement.
// Query and Chat can have different internal implementations, but the bot pool uses
// the same fallback and quota-handling flow for both.
type ConversationMessage struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Time    int64  `json:"time"`
}

type ConversationKey struct {
	botName  string
	userName string
}

type LLMProvider interface {
	Name() string
	IsReady() bool
	Query(
		systemPrompt string,
		userPrompt string,
	) *Message
	Chat(
		conversationKey ConversationKey,
		systemPrompt string,
		userPrompt string,
		history []ConversationMessage,
	) *Message
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
	PROVIDERS  []LLMProvider
)

func RegisterProvider(provider LLMProvider) {
	providerMu.Lock()
	defer providerMu.Unlock()
	PROVIDERS = append(PROVIDERS, provider)
}

// Bot keeps a shared configuration and user, and can try multiple LLM providers in order.
type Bot struct {
	Config *BotConfig
	User   *UserConfig

	chatProvider *LLMProvider
	history      []ConversationMessage
}

func (c *Bot) addIteration(askTime int64, ask string, reply string) {
	if c == nil {
		return
	}

	botName := ""
	if c.Config != nil {
		botName = c.Config.Name
	}

	userName := ""
	if c.User != nil {
		userName = c.User.Name
	}

	c.history = append(c.history, ConversationMessage{
		Name:    userName,
		Message: ask,
		Time:    askTime,
	})
	c.history = append(c.history, ConversationMessage{
		Name:    botName,
		Message: reply,
		Time:    time.Now().Unix(),
	})
}

func (c *Bot) DoChat(ask string) *Message {
	if c == nil {
		return &Message{Status: http.StatusBadGateway, Error: "bot is nil"}
	}

	askTime := time.Now().Unix()
	if len(PROVIDERS) == 0 {
		return &Message{
			Status: http.StatusServiceUnavailable,
			Error:  "no LLM providers configured",
		}
	}
	if c.Config == nil {
		return &Message{
			Status: http.StatusBadRequest,
			Error:  "bot configuration is missing",
		}
	}

	conversationKey := ConversationKey{botName: c.Config.Name}
	if c.User != nil {
		conversationKey.userName = c.User.Name
	}
	systemPrompt := c.Config.Profile
	userPrompt := ask
	if c.chatProvider != nil && *c.chatProvider != nil {
		pr := (*c.chatProvider)
		r := pr.Chat(conversationKey, systemPrompt, userPrompt, c.history)
		if r == nil {
			c.chatProvider = nil
		} else if r.Status == http.StatusOK {
			c.addIteration(askTime, ask, r.Reply)
			return r
		}
	}
	c.chatProvider = nil
	var lastErr *Message = nil
	for i := 0; i < len(PROVIDERS); i++ {
		provider := PROVIDERS[i]
		if provider == nil || !provider.IsReady() {
			continue
		}
		r := provider.Chat(conversationKey, systemPrompt, userPrompt, c.history)
		if r == nil {
			lastErr = &Message{Status: http.StatusBadGateway, Error: fmt.Sprintf("provider %q returned no response", provider.Name())}
			continue
		}
		if r.Status == http.StatusOK {
			c.addIteration(askTime, ask, r.Reply)
			return r
		}
		if r.Error == "" {
			r.Error = fmt.Sprintf("provider %q failed with status %d", provider.Name(), r.Status)
		}
		lastErr = r
	}

	if lastErr != nil {
		return lastErr
	}

	return &Message{
		Status: http.StatusBadGateway,
		Error:  "all providers failed",
	}
}

func (c *Bot) DoQuery(ask string) *Message {
	if c == nil {
		return &Message{Status: http.StatusBadGateway, Error: "bot is nil"}
	}
	if c.Config == nil {
		return &Message{Status: http.StatusBadRequest, Error: "bot configuration is missing"}
	}
	if len(PROVIDERS) == 0 {
		return &Message{
			Status: http.StatusServiceUnavailable,
			Error:  "no LLM providers configured",
		}
	}

	var lastErr *Message
	for i := 0; i < len(PROVIDERS); i++ {
		provider := PROVIDERS[i]
		if provider == nil || !provider.IsReady() {
			continue
		}
		r := provider.Query(c.Config.Profile, ask)
		if r == nil {
			lastErr = &Message{Status: http.StatusBadGateway, Error: fmt.Sprintf("provider %q returned no response", provider.Name())}
			continue
		}
		if r.Status == http.StatusOK {
			return r
		}
		if r.Error == "" {
			r.Error = fmt.Sprintf("provider %q failed with status %d", provider.Name(), r.Status)
		}
		lastErr = r
	}

	if lastErr != nil {
		return lastErr
	}

	return &Message{
		Status: http.StatusBadGateway,
		Error:  "all providers failed",
	}
}
