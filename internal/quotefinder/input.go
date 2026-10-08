package quotefinder

import (
	"encoding/json"
	"github.com/pkg/errors"
	"io"
	"os"
	"strings"
)

type quote struct {
	Text      string `json:"text"`
	Movie     string `json:"movie"`
	Character string `json:"character"`
}

type inputFile struct {
	Query  string  `json:"query"`
	Quotes []quote `json:"quotes"`
}

type rankedQuote struct {
	Quote quote
	Score float64
	Index int
}

type cliOptions struct {
	Path     string
	Query    string
	HasQuery bool
	Engine   string
}

const usage = "usage: quote-finder quotes.json [--query \"your situation\"] [--engine=decisions|completions]"

func parseArgs(args []string) (cliOptions, error) {
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" || strings.HasPrefix(args[0], "--") {
		return cliOptions{}, errors.New(usage)
	}
	opts := cliOptions{Path: args[0], Engine: "decisions"}
	engineSet := false
	for i := 1; i < len(args); i++ {
		switch {
		case args[i] == "--query" && !opts.HasQuery && i+1 < len(args) && !strings.HasPrefix(args[i+1], "--"):
			i++
			opts.Query = strings.TrimSpace(args[i])
			if opts.Query == "" {
				return cliOptions{}, errors.New(usage)
			}
			opts.HasQuery = true
		case strings.HasPrefix(args[i], "--engine=") && !engineSet:
			opts.Engine = strings.TrimPrefix(args[i], "--engine=")
			if opts.Engine != "decisions" && opts.Engine != "completions" {
				return cliOptions{}, errors.New(usage)
			}
			engineSet = true
		default:
			return cliOptions{}, errors.New(usage)
		}
	}
	return opts, nil
}

func loadInput(path, override string, hasOverride bool) (inputFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return inputFile{}, errors.Wrap(err, "open input")
	}
	defer f.Close()

	var input inputFile
	decoder := json.NewDecoder(f)
	if err := decoder.Decode(&input); err != nil {
		return inputFile{}, errors.Wrap(err, "parse input JSON")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return inputFile{}, errors.New("input JSON must contain exactly one object")
	}
	if hasOverride {
		input.Query = override
	}
	input.Query = strings.TrimSpace(input.Query)
	if input.Query == "" {
		return inputFile{}, errors.New("query must not be empty")
	}
	if len(input.Quotes) == 0 {
		return inputFile{}, errors.New("quotes must contain at least one quote")
	}
	for i, q := range input.Quotes {
		if strings.TrimSpace(q.Text) == "" || strings.TrimSpace(q.Movie) == "" || strings.TrimSpace(q.Character) == "" {
			return inputFile{}, errors.Errorf("quote %d needs nonempty text, movie, and character", i+1)
		}
	}
	return input, nil
}
