package quotefinder

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"

	"quote-finder/internal/openaitools"
)

func testClient(server *httptest.Server) openaitools.Client {
	return openaitools.New("test-key", shared.ChatModelGPT6Luna,
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
	opts, err := parseArgs([]string{"quotes.json", "--query", "  feeling rejected  "})
	if err != nil || opts.Path != "quotes.json" || opts.Query != "feeling rejected" || !opts.HasQuery {
		t.Fatalf("unexpected custom query parse: %+v %v", opts, err)
	}
	opts, err = parseArgs([]string{"quotes.json"})
	if err != nil || opts.HasQuery {
		t.Fatalf("unexpected default options: %+v %v", opts, err)
	}
	for _, args := range [][]string{{}, {"quotes.json", "--query"}, {"quotes.json", "--query", " "}, {"quotes.json", "--query", "--engine=completions"}, {"quotes.json", "--other", "x"}, {"quotes.json", "--engine=other"}} {
		if _, err := parseArgs(args); err == nil {
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
	if apiErr == nil || !strings.Contains(fmt.Sprintf("%+v", apiErr), "client.go") {
		t.Fatalf("SDK error has no call-site stack: %+v", apiErr)
	}
}
