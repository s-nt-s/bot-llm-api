package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWriteJSONWritesSerializedBytesDirectly(t *testing.T) {
	recorder := httptest.NewRecorder()

	writeJSON(recorder, http.StatusOK, json.RawMessage(`{"reply":{"answer":42}}`))

	var value map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
		t.Fatalf("writeJSON() returned invalid JSON: %v", err)
	}
	if _, ok := value["reply"].(map[string]any); !ok {
		t.Fatalf("reply = %T, want JSON object", value["reply"])
	}
}
