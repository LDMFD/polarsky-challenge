package main

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
)

func sampleInput() inputFile {
	return inputFile{Query: "I feel stuck", Quotes: []quote{
		{Text: "Keep moving.", Movie: "A", Character: "One"},
		{Text: "Try again.", Movie: "B", Character: "Two"},
		{Text: "Find a way.", Movie: "C", Character: "Three"},
	}}
}

func TestParseArgs(t *testing.T) {
	path, query, overridden, err := parseArgs([]string{"quotes.json", "--query", "  feeling rejected  "})
	if err != nil || path != "quotes.json" || query != "feeling rejected" || !overridden {
		t.Fatalf("unexpected custom query parse: %q %q %v %v", path, query, overridden, err)
	}
	for _, args := range [][]string{{}, {"quotes.json", "--query"}, {"quotes.json", "--query", " "}, {"quotes.json", "--other", "x"}} {
		if _, _, _, err := parseArgs(args); err == nil {
			t.Fatalf("expected argument error for %q", args)
		}
	}
}

func TestLoadInputValidation(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quotes.json")
	for _, tc := range []struct {
		name, contents, wantError string
	}{
		{"malformed", `{`, "parse input JSON"},
		{"empty query", `{"query":"","quotes":[{"text":"Hi","movie":"M","character":"C"}]}`, "query must not be empty"},
		{"empty quotes", `{"query":"Hi","quotes":[]}`, "at least one quote"},
		{"missing field", `{"query":"Hi","quotes":[{"text":"Hi","movie":"M"}]}`, "nonempty text, movie, and character"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.contents), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := loadInput(path, "", false)
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("got error %v; want %q", err, tc.wantError)
			}
		})
	}
	if err := os.WriteFile(path, []byte(`{"query":"","quotes":[{"text":"Hi","movie":"M","character":"C"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	input, err := loadInput(path, "custom", true)
	if err != nil || input.Query != "custom" {
		t.Fatalf("override did not replace empty file query: %+v, %v", input, err)
	}
}

func TestRankQuotesAndRequest(t *testing.T) {
	input := sampleInput()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected method or authorization")
		}
		var request decisionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
		}
		if request.Model != "gpt-6-luna" || request.Input != input.Query || len(request.Questions) != 3 {
			t.Errorf("unexpected decision request: %+v", request)
		}
		for i, q := range request.Questions {
			if q.Name != "quote_"+string(rune('0'+i)) || q.Type != "predicate" || !strings.Contains(q.Instructions, input.Quotes[i].Text) {
				t.Errorf("unexpected question %d: %+v", i, q)
			}
		}
		_, _ = w.Write([]byte(`{"answers":[{"type":"predicate","name":"quote_2","probability":0.8},{"type":"predicate","name":"quote_0","probability":0.8},{"type":"predicate","name":"quote_1","probability":0.3}]}`))
	}))
	defer server.Close()

	ranked, err := rankQuotes(context.Background(), server.Client(), server.URL, "test-key", input)
	if err != nil {
		t.Fatal(err)
	}
	if ranked[0].Index != 0 || ranked[1].Index != 2 || ranked[2].Index != 1 {
		t.Fatalf("wrong rank or tie order: %+v", ranked)
	}
}

func TestRankQuotesErrors(t *testing.T) {
	input := sampleInput()
	input.Quotes = input.Quotes[:1]
	for _, tc := range []struct {
		name, response, wantError string
		status                    int
	}{
		{"HTTP failure", `{}`, "HTTP 429", 429},
		{"refusal", `{"answers":[{"type":"refusal","name":"quote_0"}]}`, "refused", 200},
		{"missing answer", `{"answers":[]}`, "0 answers for 1 quotes", 200},
		{"missing score", `{"answers":[{"type":"predicate","name":"quote_0"}]}`, "invalid score", 200},
		{"bad name", `{"answers":[{"type":"predicate","name":"quote_9","probability":0.5}]}`, "unexpected or duplicate", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()
			_, err := rankQuotes(context.Background(), server.Client(), server.URL, "test-key", input)
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("got error %v; want %q", err, tc.wantError)
			}
		})
	}
	if _, err := rankQuotes(context.Background(), http.DefaultClient, "", "", input); err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("expected missing key error; got %v", err)
	}
}

func TestRunOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quotes.json")
	if err := os.WriteFile(path, []byte(`{"query":"file query","quotes":[{"text":"Go on.","movie":"Movie","character":"Hero"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"answers":[{"type":"predicate","name":"quote_0","probability":0.92}]}`))
	}))
	defer server.Close()
	var out bytes.Buffer
	if err := run([]string{path, "--query", "new query"}, "test-key", server.Client(), server.URL, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `Top 1 quotes for: "new query"`) || !strings.Contains(out.String(), `[0.92] "Go on." - Hero (Movie)`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}
