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

type geminiProvider struct {
	name     string
	apiKey   string
	endpoint string
	readyAt  int64
	client   *http.Client

	mu                 sync.Mutex
	conversationStates map[ConversationKey]string
}

type GeminiMessage struct {
	Reply                 string `json:"reply,omitempty"`
	Status                int    `json:"status"`
	Error                 string `json:"error,omitempty"`
	PreviousInteractionID string `json:"previousInteractionID,omitempty"`
}

func (p *geminiProvider) IsReady() bool {
	if p.readyAt == -1 {
		return true
	}
	return time.Now().Unix() > p.readyAt
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

func (p *geminiProvider) Query(
	systemPrompt string,
	userPrompt string,
) *Message {
	r := p.ask(
		systemPrompt,
		userPrompt,
		"",
	)
	return &Message{
		Reply:  r.Reply,
		Status: r.Status,
		Error: r.Error,
	}
}

func (p *geminiProvider) Chat(
	conversationKey ConversationKey,
	systemPrompt string,
	userPrompt string,
	history []ConversationMessage,
) *Message {
	previousInteractionId := p.previousInteractionID(conversationKey)
	if previousInteractionId == "" && len(history) > 0 {
		systemPrompt = systemPrompt + "\n\nPrevious conversation:\n"
		for i := 0; i < len(history); i++ {
			systemPrompt = systemPrompt + "\n**" + history[i].Name + "**: " + history[i].Message
		}
	}
	r := p.ask(systemPrompt, userPrompt, previousInteractionId)

	if r.PreviousInteractionID != "" {
		p.storePreviousInteractionID(conversationKey, r.PreviousInteractionID)
	}
	return &Message{
		Reply:  r.Reply,
		Status: r.Status,
		Error: r.Error,
	}
}

func (p *geminiProvider) exhausted() {
	p.readyAt = time.Now().Add(1 * time.Hour).Unix()
	p.conversationStates = map[ConversationKey]string{}
}

func (p *geminiProvider) ask(systemPrompt string, userPrompt string, previousInteractionId string) GeminiMessage {
	r := p._ask(systemPrompt, userPrompt, previousInteractionId)
	if r.Status != http.StatusOK {
		p.exhausted()
	}
	return r
}

func (p *geminiProvider) _ask(systemPrompt string, userPrompt string, previousInteractionId string) GeminiMessage {
	if p == nil {
		return GeminiMessage{
			Status: http.StatusBadGateway,
			Error: "gemini provider is not configured",
		}
	}

	payload := map[string]any{
		"contents": []map[string]any{{
			"parts": []map[string]any{{"text": userPrompt}},
		}},
	}
	if systemPrompt != "" {
		payload["system_instruction"] = map[string]any{
			"parts": []map[string]any{{"text": systemPrompt}},
		}
	}
	if previousInteractionId != "" {
		payload["previous_interaction_id"] = previousInteractionId
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return GeminiMessage{
			Status: http.StatusBadGateway,
			Error: fmt.Errorf("marshal gemini request: %w", err).Error()
		}
	}

	url := fmt.Sprintf("%s?key=%s", p.endpoint, p.apiKey)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return GeminiMessage{
			Status: http.StatusBadGateway,
			Error: fmt.Errorf("create gemini request: %w", err).Error()
		}
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return GeminiMessage{
			Status: http.StatusBadGateway,
			Error: fmt.Errorf("gemini request failed: %w", err).Error()
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return GeminiMessage{
			Status: http.StatusBadGateway,
			Error: fmt.Errorf("gemini API returned status %d", resp.StatusCode).Error()
		}
	}

	var result struct {
		Response struct {
			Text string `json:"text"`
		} `json:"response"`
		PreviousInteractionID string `json:"previous_interaction_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return GeminiMessage{
			Status: http.StatusBadGateway,
			Error: fmt.Errorf("decode gemini response: %w", err).Error()
		}
	}

	return GeminiMessage{
		Status: http.StatusOK,
		Reply: strings.TrimSpace(result.Response.Text),
		PreviousInteractionID: result.PreviousInteractionID,
	}
}

func init() {
	for _, apiKey := range parseGeminiAPIKeys() {
		RegisterProvider(&geminiProvider{
			apiKey:             apiKey,
			client:             http.DefaultClient,
			endpoint:           geminiDefaultEndpoint,
			readyAt:            -1,
			conversationStates: map[ConversationKey]string{},
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

func (p *geminiProvider) previousInteractionID(k ConversationKey) string {
	if p == nil || p.conversationStates == nil {
		return ""
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if id, ok := p.conversationStates[k]; ok {
		return id
	}
	return ""
}

func (p *geminiProvider) storePreviousInteractionID(k ConversationKey, interactionID string) {
	if p == nil || interactionID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conversationStates[k] = interactionID
}
