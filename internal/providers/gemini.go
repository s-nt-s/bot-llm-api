package providers

import (
	"bot-api/common"
	"bot-api/internal/bot"
	"bot-api/internal/config"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	geminiDefaultEndpoint    = "https://generativelanguage.googleapis.com/v1beta/interactions"
	geminiDefaultModel       = "gemini-3.6-flash"
	geminiDefaultAPIRevision = "2026-05-20"
)

type GeminiProvider struct {
	name        string
	apiKey      string
	apiRevision string
	endpoint    string
	model       string
	readyAt     int64
	client      *http.Client

	mu                 sync.Mutex
	conversationStates map[bot.ConversationKey]string
}

type geminiResult struct {
	Response struct {
		Text string `json:"text"`
	} `json:"response"`
	Candidates []struct {
		Content geminiResponseContent `json:"content"`
	} `json:"candidates"`
	Steps                 []geminiResponseStep `json:"steps"`
	ID                    string               `json:"id"`
	PreviousInteractionID string               `json:"previous_interaction_id"`
}

type GeminiMessage struct {
	Reply                 string `json:"reply,omitempty"`
	Status                int    `json:"status"`
	Error                 string `json:"error,omitempty"`
	PreviousInteractionID string `json:"previousInteractionID,omitempty"`
}

type geminiResponsePart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type geminiResponseTextItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type geminiResponseContent struct {
	Parts []geminiResponsePart `json:"parts"`
}

type geminiResponseStep struct {
	Type    string                   `json:"type"`
	Content []geminiResponseTextItem `json:"content"`
}

func (c *geminiResponseContent) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || string(data) == "null" {
		return nil
	}

	type payload struct {
		Parts []geminiResponsePart `json:"parts"`
	}
	var obj payload
	if err := json.Unmarshal(data, &obj); err == nil {
		c.Parts = obj.Parts
		return nil
	}

	var parts []geminiResponsePart
	if err := json.Unmarshal(data, &parts); err == nil {
		c.Parts = parts
		return nil
	}

	return fmt.Errorf("unmarshal gemini content")
}

func (p *GeminiProvider) IsReady() bool {
	if p.readyAt == -1 {
		return true
	}
	return time.Now().Unix() > p.readyAt
}

func (p *GeminiProvider) Name() string {
	if p == nil {
		return "gemini"
	}
	if p.name != "" {
		return p.name
	}
	return "gemini"
}

func (p *GeminiProvider) Query(systemPrompt string, userPrompt string) *config.Message {
	r := p.ask(
		systemPrompt,
		userPrompt,
		"",
	)
	return &config.Message{
		Reply:  r.Reply,
		Status: r.Status,
		Error:  r.Error,
		Model:  p.Name(),
	}
}

func (p *GeminiProvider) Chat(
	conversationKey bot.ConversationKey,
	systemPrompt string,
	userPrompt string,
	history []bot.ConversationMessage,
) *config.Message {
	previousInteractionId := p.previousInteractionID(conversationKey)
	input := p.buildInteractionInput(
		conversationKey,
		history,
		userPrompt,
		previousInteractionId == "",
	)

	r := p.ask(systemPrompt, input, previousInteractionId)

	if r.PreviousInteractionID != "" {
		p.storePreviousInteractionID(conversationKey, r.PreviousInteractionID)
	}
	return &config.Message{
		Reply:  r.Reply,
		Status: r.Status,
		Error:  r.Error,
		Model:  p.Name(),
	}
}

func (p *GeminiProvider) buildInteractionInput(
	conversationKey bot.ConversationKey,
	history []bot.ConversationMessage,
	userPrompt string,
	includeHistory bool,
) any {
	if !includeHistory || len(history) == 0 {
		return userPrompt
	}

	input := make([]map[string]any, 0, len(history)+1)
	for _, msg := range history {
		typeName := "user_input"
		switch msg.Name {
		case conversationKey.UserName:
			typeName = "user_input"
		case conversationKey.BotName:
			typeName = "model_output"
		}
		input = append(input, map[string]any{
			"type": typeName,
			"content": []map[string]any{{
				"type": "text",
				"text": msg.Message,
			}},
		})
	}
	input = append(input, map[string]any{
		"type": "user_input",
		"content": []map[string]any{{
			"type": "text",
			"text": userPrompt,
		}},
	})
	return input
}

func (p *GeminiProvider) exhausted() {
	p.readyAt = time.Now().Add(1 * time.Hour).Unix()
}

func (p *GeminiProvider) ask(systemPrompt string, input any, previousInteractionId string) GeminiMessage {
	r := p._ask(systemPrompt, input, previousInteractionId)
	if r.Status != http.StatusOK {
		p.exhausted()
	}
	return r
}

func (p *GeminiProvider) post(payload map[string]any) (*http.Response, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}

	req, err := http.NewRequest(
		http.MethodPost,
		p.endpoint,
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("create gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", p.apiKey)
	req.Header.Set("Api-Revision", p.apiRevision)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini request failed: %w", err)
	}
	return resp, nil
}

func (p *GeminiProvider) _ask(systemPrompt string, input any, previousInteractionId string) GeminiMessage {
	if p == nil {
		return GeminiMessage{Status: http.StatusBadGateway, Error: "gemini provider is not configured"}
	}

	payload := map[string]any{"model": p.model, "input": input}
	if systemPrompt != "" {
		payload["system_instruction"] = systemPrompt
	}
	if previousInteractionId != "" {
		payload["previous_interaction_id"] = previousInteractionId
	}
	resp, err := p.post(payload)
	if err != nil {
		return GeminiMessage{Status: http.StatusBadGateway, Error: err.Error()}
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return GeminiMessage{Status: http.StatusBadGateway, Error: fmt.Errorf("read gemini response body: %w", err).Error()}
	}

	if resp.StatusCode >= http.StatusBadRequest {
		return GeminiMessage{Status: http.StatusBadGateway, Error: fmt.Errorf("gemini API returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes))).Error()}
	}

	var result geminiResult
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return GeminiMessage{Status: http.StatusBadGateway, Error: fmt.Errorf("decode gemini response: %w; body=%s", err, strings.TrimSpace(string(bodyBytes))).Error()}
	}

	reply := getReply(result)
	previousID := result.PreviousInteractionID
	if previousID == "" {
		previousID = result.ID
	}

	return GeminiMessage{Status: http.StatusOK, Reply: reply, PreviousInteractionID: previousID}
}

func getReply(result geminiResult) string {
	reply := strings.TrimSpace(result.Response.Text)
	if reply != "" {
		return reply
	}

	for _, candidate := range result.Candidates {
		for _, part := range candidate.Content.Parts {
			reply = strings.TrimSpace(part.Text)
			if reply != "" {
				return reply
			}
		}
	}

	for i := len(result.Steps) - 1; i >= 0; i-- {
		step := result.Steps[i]
		if step.Type == "model_output" {
			for _, content := range step.Content {
				if content.Type == "text" {
					return strings.TrimSpace(content.Text)
				}
			}
		}
	}
	return ""
}

func init() {
	for i, apiKey := range common.GetEnvList("GEMINI") {
		g := GeminiProvider{
			apiKey:             apiKey,
			apiRevision:        geminiDefaultAPIRevision,
			client:             http.DefaultClient,
			endpoint:           geminiDefaultEndpoint,
			model:              geminiDefaultModel,
			readyAt:            -1,
			conversationStates: map[bot.ConversationKey]string{},
		}
		g.name = fmt.Sprintf("%s (%d)", g.model, i+1)
		log.Printf("registering Gemini provider: %s", g.name)
		bot.RegisterProvider(&g)
	}
}

func (p *GeminiProvider) previousInteractionID(k bot.ConversationKey) string {
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

func (p *GeminiProvider) storePreviousInteractionID(k bot.ConversationKey, interactionID string) {
	if p == nil || interactionID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.conversationStates[k] = interactionID
}
