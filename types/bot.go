package types

import (
	"bot-api/util"
	"fmt"
	"log"
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

type providerRegistry struct {
	mu        sync.RWMutex
	providers []LLMProvider
}

func (r *providerRegistry) register(provider LLMProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, provider)
}

func (r *providerRegistry) snapshot() []LLMProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := make([]LLMProvider, len(r.providers))
	copy(snapshot, r.providers)
	return snapshot
}

var (
	providers = &providerRegistry{}
	botCache  = &botCacheStore{bots: make(map[ConversationKey]*Bot)}
)

func RegisterProvider(provider LLMProvider) {
	providers.register(provider)
}

func providersSnapshot() []LLMProvider {
	return providers.snapshot()
}

type Bot struct {
	Config *BotConfig
	User   *UserConfig

	chatProvider *LLMProvider
	history      []ConversationMessage
}

type botCacheStore struct {
	mu   sync.RWMutex
	bots map[ConversationKey]*Bot
}

func (c *botCacheStore) get(config *BotConfig, user *UserConfig) *Bot {
	k := ConversationKey{botName: config.Name}
	if user != nil {
		k.userName = user.Name
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	bot, ok := c.bots[k]
	if ok {
		log.Printf("Reuse bot for %v", k)
		bot.Config = config
		bot.User = user
		return bot
	}

	log.Printf("New bot for %v", k)
	bot = &Bot{
		Config: config,
		User:   user,
	}
	c.bots[k] = bot
	return bot
}

func GetBot(config *BotConfig, user *UserConfig) *Bot {
	return botCache.get(config, user)
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

func (c *Bot) GetSystemPrompt() string {
	tm := util.GetTime()
	rplc := map[string]string{
		"{{CURRENT_DATE_TIME}}": fmt.Sprintf(
			"%s (%s)",
			tm.Format(time.RFC1123),
			tm.Location().String(),
		),
		"{{USER_NAME}}": "desconocido",
	}
	systemPrompt := c.Config.Profile
	if c.User != nil  {
		if c.User.Profile != "" {
			systemPrompt = systemPrompt + "\n\n" + c.User.Profile
		}
		if c.User.Name != "" {
			rplc["{{USER_NAME}}"] = c.User.Name
		}
	}
	systemPrompt = util.Rpl(
		systemPrompt,
		rplc,
	)
	return systemPrompt
}

func (c *Bot) DoChat(ask string) *Message {
	if c == nil {
		return &Message{Status: http.StatusBadGateway, Error: "bot is nil"}
	}

	askTime := time.Now().Unix()
	providers := providersSnapshot()
	if len(providers) == 0 {
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
	systemPrompt := c.GetSystemPrompt()
	userPrompt := ask
	if c.chatProvider != nil && *c.chatProvider != nil {
		pr := (*c.chatProvider)
		r := pr.Chat(conversationKey, systemPrompt, userPrompt, c.history)
		if r == nil {
			c.chatProvider = nil
			log.Printf("provider %q returned no response, falling back to other providers", pr.Name())
		} else if r.Status == http.StatusOK {
			c.addIteration(askTime, ask, r.Reply)
			return r
		}
	}
	c.chatProvider = nil
	var lastErr *Message = nil
	for _, provider := range providers {
		if provider == nil || !provider.IsReady() {
			continue
		}
		r := provider.Chat(conversationKey, systemPrompt, userPrompt, c.history)

		if r != nil && r.Status == http.StatusOK {
			c.addIteration(askTime, ask, r.Reply)
			return r
		}
		if r == nil {
			r = &Message{Status: http.StatusBadGateway, Error: fmt.Sprintf("provider %q returned no response", provider.Name())}
		} else if r.Error == "" {
			r.Error = fmt.Sprintf("provider %q returned status %d", provider.Name(), r.Status)
		}
		log.Printf("provider %q returned no response, falling back to other providers", provider.Name())
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
	providers := providersSnapshot()
	if len(providers) == 0 {
		return &Message{
			Status: http.StatusServiceUnavailable,
			Error:  "no LLM providers configured",
		}
	}

	systemPrompt := c.GetSystemPrompt()
	var lastErr *Message
	for _, provider := range providers {
		if provider == nil || !provider.IsReady() {
			continue
		}
		r := provider.Query(systemPrompt, ask)
		if r != nil && r.Status == http.StatusOK {
			return r
		}
		if r == nil {
			r = &Message{Status: http.StatusBadGateway, Error: fmt.Sprintf("provider %q returned no response", provider.Name())}
		}
		log.Printf("provider %q returned no response, falling back to other providers", provider.Name())
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
