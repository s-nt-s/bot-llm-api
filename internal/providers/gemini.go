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
	"os"
	"path/filepath"
	"sort"
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

type geminiConversationState struct {
	BotName       string `json:"botName"`
	UserName      string `json:"userName"`
	InteractionID string `json:"interactionID"`
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

func (r *GeminiMessage) ToMessage(p *GeminiProvider, schema json.RawMessage) *config.Message {
	m := &config.Message{
		Reply:  r.Reply,
		Status: r.Status,
		Error:  r.Error,
		Model:  p.Name(),
	}
	if len(schema) > 0 {
		m.Json = []byte(r.Reply)
		m.Reply = ""
	}
	return m
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

func (p *GeminiProvider) Query(input *bot.QueryInput) *config.Message {
	r := p.ask(
		input.Schema,
		input.SystemPrompt,
		input.UserPrompt,
		"",
	)
	return r.ToMessage(p, input.Schema)
}

func (p *GeminiProvider) Chat(input *bot.ChatInput) *config.Message {
	previousInteractionId := p.previousInteractionID(input.ConversationKey)
	i := p.buildInteractionInput(
		input.ConversationKey,
		input.History,
		input.UserPrompt,
		previousInteractionId == "",
	)

	r := p.ask(
		input.Schema,
		input.SystemPrompt,
		i,
		previousInteractionId,
	)

	if r.PreviousInteractionID != "" {
		p.storePreviousInteractionID(input.ConversationKey, r.PreviousInteractionID)
	}
	return r.ToMessage(p, input.Schema)
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

func (p *GeminiProvider) ask(schema json.RawMessage, systemPrompt string, input any, previousInteractionId string) GeminiMessage {
	r := p._ask(schema, systemPrompt, input, previousInteractionId)
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

func (p *GeminiProvider) _ask(schema json.RawMessage, systemPrompt string, input any, previousInteractionId string) GeminiMessage {
	if p == nil {
		return GeminiMessage{Status: http.StatusBadGateway, Error: "gemini provider is not configured"}
	}
	payload := map[string]any{
		"model": p.model,
		"input": input,
	}
	if systemPrompt != "" {
		payload["system_instruction"] = systemPrompt
	}
	if previousInteractionId != "" {
		payload["previous_interaction_id"] = previousInteractionId
	}
	if len(schema) > 0 {
		payload["response_format"] = map[string]any{
			"type":      "text",
			"mime_type": "application/json",
			"schema":    schema,
		}
	}
	/*
		payloadJSON, err := json.Marshal(payload)
		if err != nil {
			log.Printf("gemini request payload: failed to marshal JSON: %v", err)
		} else {
			log.Printf("gemini request payload: %s", payloadJSON)
		}
	*/
	resp, err := p.post(payload)
	if err != nil {
		return GeminiMessage{Status: http.StatusBadGateway, Error: err.Error()}
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return GeminiMessage{Status: http.StatusBadGateway, Error: fmt.Errorf("read gemini response body: %w", err).Error()}
	}
	//log.Printf("gemini response body: %s", bodyBytes)

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
		g.Load()
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

func (p *GeminiProvider) filePathStore() string {
	path := "data/gemini/" + p.apiKey + ".json"
	return path
}

func (p *GeminiProvider) Close() {
	if p == nil || p.apiKey == "" {
		return
	}
	if len(p.conversationStates) == 0 {
		return
	}

	p.mu.Lock()
	states := make([]geminiConversationState, 0, len(p.conversationStates))
	for key, interactionID := range p.conversationStates {
		states = append(states, geminiConversationState{
			BotName:       key.BotName,
			UserName:      key.UserName,
			InteractionID: interactionID,
		})
	}
	p.mu.Unlock()

	sort.Slice(states, func(i, j int) bool {
		if states[i].BotName != states[j].BotName {
			return states[i].BotName < states[j].BotName
		}
		return states[i].UserName < states[j].UserName
	})

	data, err := json.MarshalIndent(states, "", "  ")
	if err != nil {
		log.Printf("failed to encode Gemini conversation states: %v", err)
		return
	}
	path := p.filePathStore()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		log.Printf("failed to create Gemini data directory: %v", err)
		return
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		log.Printf("failed to save Gemini conversation states: %v", err)
	}
}

func (p *GeminiProvider) Load() {
	if p == nil || p.apiKey == "" {
		return
	}

	data, err := os.ReadFile(p.filePathStore())
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("failed to load Gemini conversation states: %v", err)
		}
		return
	}

	var states []geminiConversationState
	if err := json.Unmarshal(data, &states); err != nil {
		log.Printf("failed to decode Gemini conversation states: %v", err)
		return
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.conversationStates == nil {
		p.conversationStates = make(map[bot.ConversationKey]string)
	}
	for _, state := range states {
		key := bot.ConversationKey{
			BotName:  state.BotName,
			UserName: state.UserName,
		}
		if _, exists := p.conversationStates[key]; !exists {
			p.conversationStates[key] = state.InteractionID
		}
	}
}
