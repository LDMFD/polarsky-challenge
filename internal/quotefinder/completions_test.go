package quotefinder

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3"
)

func writeToolCall(t *testing.T, w http.ResponseWriter, arguments string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	response := map[string]any{"choices": []any{map[string]any{
		"index": 0, "finish_reason": "tool_calls",
		"message": map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{
			"id": "call_1", "type": "function", "function": map[string]any{"name": rankQuotesTool, "arguments": arguments},
		}}},
	}}}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		t.Error(err)
	}
}

func TestCompletionsRanksFromEnumToolCall(t *testing.T) {
	input := sampleInput()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" || r.Method != http.MethodPost {
			t.Errorf("unexpected API route: %s %s", r.Method, r.URL.Path)
		}
		var request struct {
			Model           string `json:"model"`
			ReasoningEffort string `json:"reasoning_effort"`
			Messages        []struct {
				Content string `json:"content"`
			} `json:"messages"`
			Tools []struct {
				Function struct {
					Strict     bool `json:"strict"`
					Parameters struct {
						Properties map[string]struct {
							Properties map[string]struct {
								Enum []string `json:"enum"`
							} `json:"properties"`
						} `json:"properties"`
					} `json:"parameters"`
				} `json:"function"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Model != "gpt-6-luna" || request.ReasoningEffort != "none" || len(request.Messages) != 2 || len(request.Tools) != 1 || !request.Tools[0].Function.Strict {
			t.Errorf("unexpected completion request: %+v", request)
		}
		for _, q := range input.Quotes {
			if !strings.Contains(request.Messages[1].Content, q.Text) {
				t.Errorf("candidate quote missing: %q", q.Text)
			}
		}
		if !strings.Contains(request.Messages[1].Content, input.Query) {
			t.Error("query missing from completion request")
		}
		enum := request.Tools[0].Function.Parameters.Properties["rank_1"].Properties["quote_id"].Enum
		if len(enum) != 3 || enum[0] != "quote_0" || enum[2] != "quote_2" {
			t.Errorf("incorrect quote ID enum: %v", enum)
		}
		writeToolCall(t, w, `{"rank_1":{"quote_id":"quote_2","relevance_score":0.9},"rank_2":{"quote_id":"quote_0","relevance_score":0.8},"rank_3":{"quote_id":"quote_1","relevance_score":0.7}}`)
	}))
	defer server.Close()

	ranked, err := rankQuotesWithCompletions(context.Background(), testClient(server), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranked) != 3 || ranked[0].Index != 2 || ranked[1].Index != 0 || ranked[2].Index != 1 {
		t.Fatalf("wrong ranking: %+v", ranked)
	}
}

func TestCompletionsRejectsDuplicateQuote(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeToolCall(t, w, `{"rank_1":{"quote_id":"quote_0","relevance_score":0.9},"rank_2":{"quote_id":"quote_0","relevance_score":0.8},"rank_3":{"quote_id":"quote_1","relevance_score":0.7}}`)
	}))
	defer server.Close()
	_, err := rankQuotesWithCompletions(context.Background(), testClient(server), sampleInput())
	if err == nil || !strings.Contains(err.Error(), "duplicate quote ID") {
		t.Fatalf("expected duplicate quote error, got %v", err)
	}
}

func TestRunCompletionsEngine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quotes.json")
	if err := os.WriteFile(path, []byte(`{"query":"file query","quotes":[{"text":"Go on.","movie":"Movie","character":"Hero"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		writeToolCall(t, w, `{"rank_1":{"quote_id":"quote_0","relevance_score":0.92}}`)
	}))
	defer server.Close()
	var out bytes.Buffer
	if err := Run([]string{path, "--engine=completions", "--query", "new query"}, filepath.Join(dir, "missing"), func(string) string { return "test-key" }, func(string) openai.Client { return testClient(server) }, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `Top 1 quotes for: "new query"`) || !strings.Contains(out.String(), `[0.92] "Go on." - Hero (Movie)`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}
