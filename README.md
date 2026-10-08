# quote-finder

A Go CLI that ranks movie quotes for a user's situation or feeling. It uses OpenAI's Decisions API to estimate each quote's emotional relevance, then prints the top three with scores from 0 to 1.

## Run

Requires Go 1.25+, network access, and an OpenAI API key. Put `OPENAI_API_KEY=...` in `env.props` beside `main.go`, or set the environment variable; the environment variable takes precedence. No shell sourcing is needed. `env.props` is ignored by Git.

```sh
go run main.go quotes.json
go run main.go quotes.json --query "I just got rejected and feel like giving up"
```

The JSON file needs a `query` string and a `quotes` array. Each quote needs `text`, `movie`, and `character` strings. `--query` overrides the file's query. Scores are model estimates of relevance, not calibrated mental wellness measures.

Run tests with `go test ./...`.

## Next feature

Let users save a quote and optionally record whether it helped, then use that feedback to improve future recommendations and invite a later check-in.
