package bot

import (
	"bot-api/common"
	"bot-api/internal/config"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// LLMProvider defines the minimal interface any LLM backend must implement.
type LLMProvider interface {
	Name() string
	IsReady() bool
	Close()
	Query(input *QueryInput) *config.Message
	Chat(input *ChatInput) *config.Message
}

type ConversationMessage struct {
	Name    string `json:"name"`
	Message string `json:"message"`
	Time    int64  `json:"time"`
}

type ConversationKey struct {
	BotName  string
	UserName string
}

type providerRegistry struct {
	mu        sync.RWMutex
	providers []LLMProvider
}

type ChatInput struct {
	ConversationKey ConversationKey
	SystemPrompt    string
	UserPrompt      string
	History         []ConversationMessage
	Schema          json.RawMessage
	Temperature     int
}

type QueryInput struct {
	SystemPrompt string
	UserPrompt   string
	Schema       json.RawMessage
	Temperature  int
}

func (q *QueryInput) dump(conversationKey *ConversationKey, r *config.Message) {
	cachePath := q.getPathCache(conversationKey)
	if cachePath == "" || r == nil {
		return
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		log.Printf("failed to encode query cache %s: %v", cachePath, err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(cachePath), 0700); err != nil {
		log.Printf("failed to create query cache directory for %s: %v", cachePath, err)
		return
	}
	if err := os.WriteFile(cachePath, data, 0600); err != nil {
		log.Printf("failed to write query cache %s: %v", cachePath, err)
		return
	}
	log.Printf("Save cache message in %s", cachePath)
}
func (q *QueryInput) load(conversationKey *ConversationKey) *config.Message {
	cachePath := q.getPathCache(conversationKey)
	if cachePath == "" {
		return nil
	}

	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}
	var message config.Message
	if err := json.Unmarshal(data, &message); err != nil {
		log.Printf("failed to decode query cache %s: %v", cachePath, err)
		return nil
	}
	log.Printf("Load cache message from %s", cachePath)
	return &message
}

func (q *QueryInput) getPathCache(k *ConversationKey) string {
	if q == nil || q.Temperature != 0 {
		return ""
	}
	data, err := json.Marshal(q)
	if err != nil {
		return ""
	}
	hash := sha256.Sum256(data)
	path := fmt.Sprintf("%x.json", hash)
	if k.UserName != "" {
		path = k.UserName + "/" + path
	}
	if k.BotName != "" {
		path = k.BotName + "/" + path
	}
	return "data/cache/" + path
}

const maxFetchedContentSize = 1 << 20

var userPromptHTTPClient = &http.Client{Timeout: 10 * time.Second}

func (r *providerRegistry) register(provider LLMProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers = append(r.providers, provider)
}

func (r *providerRegistry) unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, provider := range r.providers {
		if provider != nil && provider.Name() == name {
			r.providers = append(r.providers[:i], r.providers[i+1:]...)
			return
		}
	}
}

func (r *providerRegistry) snapshot(current *LLMProvider) []LLMProvider {
	r.mu.RLock()
	defer r.mu.RUnlock()

	start := -1
	ready := make([]LLMProvider, 0, len(r.providers))
	for _, provider := range r.providers {
		if provider == nil {
			continue
		}
		if current != nil && provider == *current {
			start = len(ready)
		}
		if provider.IsReady() {
			ready = append(ready, provider)
		}
	}

	if len(ready) == 0 || start <= 0 || start >= len(ready) {
		return ready
	}

	rotated := append(ready[start:], ready[:start]...)
	return rotated
}

const (
	defaultBotHistoryMaxMessages = 100
	defaultBotCacheMaxEntries    = 1000
)

var (
	providers             = &providerRegistry{}
	botCache              = &botCacheStore{bots: make(map[ConversationKey]*Bot)}
	botHistoryMaxMessages int
	botCacheMaxEntries    int
)

func init() {
	botHistoryMaxMessages = common.GetEnvInt("BOT_HISTORY_MAX_MESSAGES", defaultBotHistoryMaxMessages)
	botCacheMaxEntries = common.GetEnvInt("BOT_CACHE_MAX_ENTRIES", defaultBotCacheMaxEntries)
}

func RegisterProvider(provider LLMProvider) {
	providers.register(provider)
}

func UnregisterProvider(name string) {
	providers.unregister(name)
}

func ProvidersSnapshot(current *LLMProvider) []LLMProvider {
	return providers.snapshot(current)
}

func CloseProviders() {
	providers.mu.RLock()
	snapshot := append([]LLMProvider(nil), providers.providers...)
	providers.mu.RUnlock()

	for _, provider := range snapshot {
		provider.Close()
	}
}

type Bot struct {
	Config *config.BotConfig
	User   *config.UserConfig

	chatProvider *LLMProvider
	history      []ConversationMessage
}

type botCacheStore struct {
	mu    sync.RWMutex
	bots  map[ConversationKey]*Bot
	order []ConversationKey
}

func (c *botCacheStore) get(configItem *config.BotConfig, user *config.UserConfig) *Bot {
	k := ConversationKey{BotName: configItem.Name}
	if user != nil {
		k.UserName = user.Name
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	botItem, ok := c.bots[k]
	if ok {
		log.Printf("Reuse bot for %v", k)
		botItem.Config = configItem
		botItem.User = user
		return botItem
	}

	log.Printf("New bot for %v", k)
	botItem = &Bot{Config: configItem, User: user}

	if len(c.bots)+1 > botCacheMaxEntries {
		if len(c.order) > 0 {
			oldest := c.order[0]
			delete(c.bots, oldest)
			c.order = c.order[1:]
			log.Printf("Evicting bot cache entry for %v to enforce max %d entries", oldest, botCacheMaxEntries)
		}
	}

	c.bots[k] = botItem
	c.order = append(c.order, k)
	return botItem
}

func GetBot(configItem *config.BotConfig, user *config.UserConfig) *Bot {
	return botCache.get(configItem, user)
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

	if botHistoryMaxMessages > 0 && len(c.history) > botHistoryMaxMessages {
		excess := len(c.history) - botHistoryMaxMessages
		c.history = c.history[excess:]
		log.Printf("Truncating history for bot=%q user=%q from %d to %d messages", botName, userName, len(c.history)+excess, len(c.history))
	}
}

func (c *Bot) GetSystemPrompt() string {
	tm := common.GetTime()
	rplc := map[string]string{
		"{{CURRENT_DATE_TIME}}": fmt.Sprintf("%s (%s)", tm.Format(time.RFC1123), tm.Location().String()),
		"{{USER_NAME}}":         "desconocido",
	}
	systemPrompt := c.Config.Profile
	if c.User != nil {
		if c.User.Profile != "" {
			systemPrompt = systemPrompt + "\n\n" + c.User.Profile
		}
		if c.User.Name != "" {
			rplc["{{USER_NAME}}"] = c.User.Name
		}
	}
	systemPrompt = common.Rpl(systemPrompt, rplc)
	return systemPrompt
}

func (c *Bot) GetConversationKey() ConversationKey {
	k := ConversationKey{BotName: c.Config.Name}
	if c.User != nil {
		k.UserName = c.User.Name
	}
	return k
}

func (c *Bot) getUserPrompt(ask string) string {
	if c == nil || c.Config == nil || !c.Config.Fetch {
		return ask
	}

	address := strings.TrimSpace(ask)
	if address == "" || strings.ContainsAny(address, " \t\r\n") {
		return ask
	}

	parsedURL, err := url.Parse(address)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return ask
	}

	response, err := userPromptHTTPClient.Get(address)
	if err != nil {
		return ask
	}
	defer response.Body.Close()

	body, readErr := io.ReadAll(io.LimitReader(response.Body, maxFetchedContentSize))
	if readErr != nil || response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return ask
	}
	log.Printf("Fetched %d bytes from %s for user prompt", len(body), address)
	return string(body)
}

func (c *Bot) DoChat(ask string) *config.Message {
	if c == nil {
		return &config.Message{Status: http.StatusBadGateway, Error: "bot is nil"}
	}
	if c.Config == nil {
		return &config.Message{Status: http.StatusBadRequest, Error: "bot configuration is missing"}
	}

	askTime := time.Now().Unix()

	conversationKey := c.GetConversationKey()
	providers := ProvidersSnapshot(c.chatProvider)
	if len(providers) == 0 {
		return &config.Message{Status: http.StatusServiceUnavailable, Error: "no LLM providers configured"}
	}
	chatInput := ChatInput{
		ConversationKey: conversationKey,
		SystemPrompt:    c.GetSystemPrompt(),
		UserPrompt:      c.getUserPrompt(ask),
		History:         c.history,
		Schema:          c.Config.Schema,
		Temperature:     c.Config.Temperature,
	}
	var lastErr *config.Message = nil
	for _, provider := range providers {
		if provider == nil || !provider.IsReady() {
			continue
		}
		r := provider.Chat(&chatInput)

		if r != nil && r.Status == http.StatusOK {
			c.addIteration(askTime, ask, r.Reply)
			c.chatProvider = &provider
			log.Printf("%v use %s", conversationKey, provider.Name())
			return r
		}
		if r == nil {
			r = &config.Message{Status: http.StatusBadGateway, Error: fmt.Sprintf("provider %q returned no response", provider.Name())}
		} else if r.Error == "" {
			r.Error = fmt.Sprintf("provider %q returned status %d", provider.Name(), r.Status)
		}
		log.Printf("provider %q returned no response, falling back to other providers", provider.Name())
		lastErr = r
	}

	if lastErr != nil {
		return lastErr
	}

	return &config.Message{Status: http.StatusBadGateway, Error: "all providers failed"}
}

func (c *Bot) DoQuery(ask string) *config.Message {
	if c == nil {
		return &config.Message{Status: http.StatusBadGateway, Error: "bot is nil"}
	}
	if c.Config == nil {
		return &config.Message{Status: http.StatusBadRequest, Error: "bot configuration is missing"}
	}
	providers := ProvidersSnapshot(nil)
	if len(providers) == 0 {
		return &config.Message{Status: http.StatusServiceUnavailable, Error: "no LLM providers configured"}
	}

	conversationKey := c.GetConversationKey()
	queryInput := QueryInput{
		SystemPrompt: c.GetSystemPrompt(),
		UserPrompt:   c.getUserPrompt(ask),
		Schema:       c.Config.Schema,
		Temperature:  c.Config.Temperature,
	}
	if cachedReply := queryInput.load(&conversationKey); cachedReply != nil {
		return cachedReply
	}

	var lastErr *config.Message
	for _, provider := range providers {
		if provider == nil || !provider.IsReady() {
			continue
		}
		r := provider.Query(&queryInput)
		if r != nil && r.Status == http.StatusOK {
			queryInput.dump(&conversationKey, r)
			log.Printf("%v use %s", conversationKey, provider.Name())
			return r
		}
		if r == nil {
			r = &config.Message{Status: http.StatusBadGateway, Error: fmt.Sprintf("provider %q returned no response", provider.Name())}
		}
		log.Printf("provider %q returned no response, falling back to other providers", provider.Name())
		lastErr = r
	}

	if lastErr != nil {
		return lastErr
	}

	return &config.Message{Status: http.StatusBadGateway, Error: "all providers failed"}
}
