package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleRequestReadsAskFromJSONPost(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/blas/query", bytes.NewBufferString(`{"ask":"hello"}`))
	request.Header.Set("Content-Type", "application/json")

	if ask := getValues(request).Get("ask"); ask != "hello" {
		t.Fatalf("getValues().Get(ask) = %q, want %q", ask, "hello")
	}
}

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
