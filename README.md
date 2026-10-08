# quote-finder

A Go CLI that ranks movie quotes for a user's situation or feeling. It prints the top three with relevance scores from 0 to 1.

## Run

Requires Go 1.25+, network access, and an OpenAI API key. Put `OPENAI_API_KEY=...` in `env.props` beside `main.go`, or set the environment variable; the environment variable takes precedence. No shell sourcing is needed. `env.props` is ignored by Git.

```sh
go run main.go quotes.json
go run main.go quotes.json --query "I just got rejected and feel like giving up"
go run main.go quotes.json --query "I'm into space travel right now"
```

The CLI uses Chat Completions with a forced function call whose quote IDs are restricted to the supplied quotes. Its scores are model relevance estimates, not calibrated mental wellness measures. The JSON file needs a `query` string and a `quotes` array; each quote needs `text`, `movie`, and `character` strings. `--query` overrides the file's query.

Run tests with `go test ./...`.

## Next feature

Let users save a quote and optionally record whether it helped, then use that feedback to improve future recommendations and invite a later check-in.
