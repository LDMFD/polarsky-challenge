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

func parseArgs(args []string) (path, override string, hasOverride bool, err error) {
	if len(args) != 1 && len(args) != 3 {
		return "", "", false, errors.New("usage: quote-finder quotes.json [--query \"your situation\"]")
	}
	if strings.TrimSpace(args[0]) == "" {
		return "", "", false, errors.New("input file path is empty")
	}
	if len(args) == 3 {
		if args[1] != "--query" || strings.TrimSpace(args[2]) == "" {
			return "", "", false, errors.New("usage: quote-finder quotes.json [--query \"your situation\"]")
		}
		return args[0], strings.TrimSpace(args[2]), true, nil
	}
	return args[0], "", false, nil
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
