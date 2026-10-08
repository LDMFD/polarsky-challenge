package openaitools

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
)

func TestCallStrictTool(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body struct {
			ToolChoice struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tool_choice"`
			Tools []struct {
				Function struct {
					Name   string `json:"name"`
					Strict bool   `json:"strict"`
				} `json:"function"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.ToolChoice.Function.Name != "choose_item" || len(body.Tools) != 1 || body.Tools[0].Function.Name != "choose_item" || !body.Tools[0].Function.Strict {
			t.Errorf("unexpected tool configuration: %+v", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"choose_item","arguments":"{\"item\":\"a\"}"}}]}}]}`))
	}))
	defer server.Close()

	client := New("test-key", shared.ChatModelGPT6Luna, option.WithBaseURL(server.URL+"/v1"), option.WithHTTPClient(server.Client()), option.WithMaxRetries(0))
	args, err := client.CallStrictTool(context.Background(), ToolRequest{
		Instructions: "Choose one item.", Input: "a or b", Name: "choose_item",
		Parameters: shared.FunctionParameters{"type": "object", "properties": map[string]any{"item": map[string]any{"type": "string"}}, "required": []string{"item"}, "additionalProperties": false},
	})
	if err != nil || string(args) != `{"item":"a"}` {
		t.Fatalf("unexpected tool result: %s, %v", args, err)
	}
}

func TestCallStrictToolMissingCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"I cannot do that."}}]}`))
	}))
	defer server.Close()
	client := New("test-key", shared.ChatModelGPT6Luna, option.WithBaseURL(server.URL+"/v1"), option.WithHTTPClient(server.Client()), option.WithMaxRetries(0))
	_, err := client.CallStrictTool(context.Background(), ToolRequest{Name: "choose_item"})
	if err == nil || !strings.Contains(err.Error(), "did not make exactly one tool call") {
		t.Fatalf("expected missing tool call error, got %v", err)
	}
}
