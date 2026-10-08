package quotefinder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

func testClient(server *httptest.Server) openai.Client {
	return openai.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(server.URL+"/v1"),
		option.WithHTTPClient(server.Client()),
		option.WithMaxRetries(0),
	)
}

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
		var request struct {
			Model     string `json:"model"`
			Input     string `json:"input"`
			Questions []struct {
				Type         string `json:"type"`
				Name         string `json:"name"`
				Instructions string `json:"instructions"`
			} `json:"questions"`
		}
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
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"answers":[{"type":"predicate","name":"quote_2","probability":0.8},{"type":"predicate","name":"quote_0","probability":0.8},{"type":"predicate","name":"quote_1","probability":0.3}]}`))
	}))
	defer server.Close()

	ranked, err := rankQuotes(context.Background(), testClient(server), input)
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
		{"HTTP failure", `{}`, "429", 429},
		{"all refused", `{"answers":[{"type":"refusal","name":"quote_0"}]}`, "refused to score every quote", 200},
		{"missing answer", `{"answers":[]}`, "0 answers for 1 quotes", 200},
		{"missing score", `{"answers":[{"type":"predicate","name":"quote_0"}]}`, "invalid score", 200},
		{"bad name", `{"answers":[{"type":"predicate","name":"quote_9","probability":0.5}]}`, "unexpected or duplicate", 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.response))
			}))
			defer server.Close()
			_, err := rankQuotes(context.Background(), testClient(server), input)
			if err == nil || !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("got error %v; want %q", err, tc.wantError)
			}
		})
	}
}

func TestRankQuotesSkipsOneRefusal(t *testing.T) {
	input := sampleInput()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"answers":[{"type":"predicate","name":"quote_0","probability":0.7},{"type":"refusal","name":"quote_1"},{"type":"predicate","name":"quote_2","probability":0.9}]}`))
	}))
	defer server.Close()

	ranked, err := rankQuotes(context.Background(), testClient(server), input)
	if err != nil {
		t.Fatal(err)
	}
	if len(ranked) != 2 || ranked[0].Index != 2 || ranked[1].Index != 0 {
		t.Fatalf("expected scored quotes in rank order, got %+v", ranked)
	}
}

func TestLoadAPIKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env.props")
	if err := os.WriteFile(path, []byte("OPENAI_API_KEY=from-file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	emptyEnv := func(string) string { return "" }
	key, err := loadAPIKey(path, emptyEnv)
	if err != nil || key != "from-file" {
		t.Fatalf("file key: %q, %v", key, err)
	}
	key, err = loadAPIKey(path, func(string) string { return "from-env" })
	if err != nil || key != "from-env" {
		t.Fatalf("environment override: %q, %v", key, err)
	}
	if err := os.WriteFile(path, []byte("INVALID LINE!"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadAPIKey(path, emptyEnv); err == nil || !strings.Contains(err.Error(), "load env.props") {
		t.Fatalf("expected parser error, got %v", err)
	}
	if _, err := loadAPIKey(filepath.Join(dir, "missing"), emptyEnv); err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY") {
		t.Fatalf("expected missing key error, got %v", err)
	}
}

func TestThirdPartyErrorsCarryStacks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env.props")
	if err := os.WriteFile(path, []byte("INVALID LINE!"), 0600); err != nil {
		t.Fatal(err)
	}
	_, configErr := loadAPIKey(path, func(string) string { return "" })
	if configErr == nil || !strings.Contains(fmt.Sprintf("%+v", configErr), "config.go") {
		t.Fatalf("koanf error has no call-site stack: %+v", configErr)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"test failure","type":"invalid_request_error"}}`))
	}))
	defer server.Close()
	input := sampleInput()
	input.Quotes = input.Quotes[:1]
	_, apiErr := rankQuotes(context.Background(), testClient(server), input)
	if apiErr == nil || !strings.Contains(fmt.Sprintf("%+v", apiErr), "ranking.go") {
		t.Fatalf("SDK error has no call-site stack: %+v", apiErr)
	}
}

func TestRunOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "quotes.json")
	if err := os.WriteFile(path, []byte(`{"query":"file query","quotes":[{"text":"Go on.","movie":"Movie","character":"Hero"}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"answers":[{"type":"predicate","name":"quote_0","probability":0.92}]}`))
	}))
	defer server.Close()
	var out bytes.Buffer
	newClient := func(string) openai.Client { return testClient(server) }
	if err := Run([]string{path, "--query", "new query"}, filepath.Join(dir, "missing"), func(string) string { return "test-key" }, newClient, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `Top 1 quotes for: "new query"`) || !strings.Contains(out.String(), `[0.92] "Go on." - Hero (Movie)`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}
