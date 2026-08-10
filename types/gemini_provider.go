package types

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

const geminiDefaultEndpoint = "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent"

type conversationKey struct {
	botName  string
	userName string
}

type geminiProvider struct {
	name     string
	apiKey   string
	endpoint string
	client   *http.Client

	mu                 sync.Mutex
	conversationStates map[conversationKey]string
	conversations      map[conversationKey][]ConversationMessage
}

func (p *geminiProvider) Name() string {
	if p == nil {
		return "gemini"
	}
	if p.name != "" {
		return p.name
	}
	return "gemini"
}

func (p *geminiProvider) Query(ask string, bot *BotConfig, user *UserConfig) (string, error) {
	return p.ask(ask, bot, user, false)
}

func (p *geminiProvider) Chat(ask string, bot *BotConfig, user *UserConfig) (string, error) {
	return p.ask(ask, bot, user, true)
}

func (p *geminiProvider) Conversation(bot *BotConfig, user *UserConfig) []ConversationMessage {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conversations == nil {
		return nil
	}
	key := conversationKey{botName: bot.Name, userName: user.Name}
	if len(p.conversations) == 0 {
		return nil
	}
	for k, msgs := range p.conversations {
		if len(msgs) > 0 {
			key = k
			break
		}
	}
	messages := append([]ConversationMessage(nil), p.conversations[key]...)
	return messages
}

func (p *geminiProvider) ClearConversation(bot *BotConfig, user *UserConfig) {
	if p == nil || bot == nil || user == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.conversations, conversationKey{botName: bot.Name, userName: user.Name})
	delete(p.conversationStates, conversationKey{botName: bot.Name, userName: user.Name})
}

func (p *geminiProvider) PopConversation(bot *BotConfig, user *UserConfig) []ConversationMessage {
	c := p.Conversation(bot, user)
	p.ClearConversation(bot, user)
	return c
}

func (p *geminiProvider) ask(ask string, bot *BotConfig, user *UserConfig, isChat bool) (string, error) {
	if p == nil {
		return "", fmt.Errorf("gemini provider is not configured")
	}

	var systemPrompt string
	if bot != nil && strings.TrimSpace(bot.Profile) != "" {
		systemPrompt = strings.TrimSpace(bot.Profile)
	}
	if user != nil && strings.TrimSpace(user.Profile) != "" {
		if systemPrompt != "" {
			systemPrompt = fmt.Sprintf("%s\n\nUser profile: %s", systemPrompt, strings.TrimSpace(user.Profile))
		} else {
			systemPrompt = strings.TrimSpace(user.Profile)
		}
	}

	payload := map[string]any{
		"contents": []map[string]any{{
			"parts": []map[string]any{{"text": ask}},
		}},
	}
	if systemPrompt != "" {
		payload["system_instruction"] = map[string]any{
			"parts": []map[string]any{{"text": systemPrompt}},
		}
	}
	if isChat {
		if interactionID := p.previousInteractionID(bot, user); interactionID != "" {
			payload["previous_interaction_id"] = interactionID
		}
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal gemini request: %w", err)
	}

	url := fmt.Sprintf("%s?key=%s", p.endpoint, p.apiKey)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return "", &QuotaExceededError{Provider: p.Name(), Cause: fmt.Errorf("gemini API returned status %d", resp.StatusCode)}
	}

	var result struct {
		Response struct {
			Text string `json:"text"`
		} `json:"response"`
		PreviousInteractionID string `json:"previous_interaction_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode gemini response: %w", err)
	}

	if isChat {
		p.appendConversation(bot, user, ask, strings.TrimSpace(result.Response.Text))
		if result.PreviousInteractionID != "" {
			p.storePreviousInteractionID(bot, user, result.PreviousInteractionID)
		}
	}

	return strings.TrimSpace(result.Response.Text), nil
}

func init() {
	for _, apiKey := range parseGeminiAPIKeys() {
		RegisterProvider("gemini", func() LLMProvider {
			return &geminiProvider{
				apiKey:             apiKey,
				client:             http.DefaultClient,
				endpoint:           geminiDefaultEndpoint,
				conversationStates: map[conversationKey]string{},
				conversations:      map[conversationKey][]ConversationMessage{},
			}
		})
	}
}

func parseGeminiAPIKeys() []string {
	value := strings.TrimSpace(os.Getenv("GEMINI"))
	if value == "" {
		return nil
	}
	parts := strings.Fields(value)
	keys := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			keys = append(keys, part)
		}
	}
	return keys
}

func (p *geminiProvider) previousInteractionID(bot *BotConfig, user *UserConfig) string {
	if p == nil || p.conversationStates == nil || user == nil || bot == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if id, ok := p.conversationStates[conversationKey{botName: bot.Name, userName: user.Name}]; ok {
		return id
	}
	return ""
}

func (p *geminiProvider) storePreviousInteractionID(bot *BotConfig, user *UserConfig, interactionID string) {
	if p == nil || bot == nil || user == nil || interactionID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conversationStates[conversationKey{botName: bot.Name, userName: user.Name}] = interactionID
}

func (p *geminiProvider) appendConversation(bot *BotConfig, user *UserConfig, userMessage string, botReply string) {
	if p == nil || bot == nil || user == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conversations == nil {
		p.conversations = map[conversationKey][]ConversationMessage{}
	}
	key := conversationKey{botName: bot.Name, userName: user.Name}
	now := time.Now().UnixMilli()
	p.conversations[key] = append(p.conversations[key],
		ConversationMessage{Name: user.Name, Message: userMessage, Time: now},
		ConversationMessage{Name: bot.Name, Message: botReply, Time: now},
	)
}
