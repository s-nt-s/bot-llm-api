package config

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestBotConfigTemperatureDefaultsToNegativeOne(t *testing.T) {
	var config BotConfig
	if err := yaml.Unmarshal([]byte("name: test\n"), &config); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if config.Temperature != -1 {
		t.Fatalf("Temperature = %d, want -1", config.Temperature)
	}
}

func TestBotConfigTemperaturePreservesExplicitValue(t *testing.T) {
	var config BotConfig
	if err := yaml.Unmarshal([]byte("temperature: 0\n"), &config); err != nil {
		t.Fatalf("yaml.Unmarshal() error = %v", err)
	}
	if config.Temperature != 0 {
		t.Fatalf("Temperature = %d, want 0", config.Temperature)
	}
}

func TestMessageToJSONUsesRawReplyWhenIsJson(t *testing.T) {
	data := (&Message{
		Reply:  `{"answer":42}`,
		Status: 200,
		Model:  "test-model",
		IsJson: true,
	}).ToJSON()

	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("ToJSON() returned invalid JSON: %v", err)
	}
	if _, ok := value["isJson"]; ok {
		t.Fatal(`ToJSON() included "isJson"`)
	}
	if value["reply"].(map[string]any)["answer"] != float64(42) {
		t.Fatalf("reply = %v, want raw JSON object", value["reply"])
	}
}

func TestMessageToJSONUsesStringReplyWhenIsJsonIsFalse(t *testing.T) {
	data := (&Message{Reply: `{"answer":42}`, Status: 200}).ToJSON()

	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatalf("ToJSON() returned invalid JSON: %v", err)
	}
	if value["reply"] != `{"answer":42}` {
		t.Fatalf("reply = %v, want original string", value["reply"])
	}
	if _, ok := value["isJson"]; ok {
		t.Fatal(`ToJSON() included "isJson"`)
	}
}
