# quote-finder

A Go CLI that ranks movie quotes for a user's situation or feeling. It prints the top three with relevance scores from 0 to 1.

## Run

Requires Go 1.25+, and an OpenAI API key. Put `OPENAI_API_KEY=...` in `env.props` beside `main.go`, or set the environment variable.

Usage:

    go run main.go quotes.json [--query ...]

--query overrides the JSON file's query.

Example runs:

```sh
$ go run main.go quotes.json
Top 3 quotes for: "I need motivation to keep going when things are tough"

1. [0.98] "Just keep swimming." - Dory (Finding Nemo)
2. [0.94] "Get busy living, or get busy dying." - Andy Dufresne (The Shawshank Redemption)
3. [0.89] "The only way out is through." - John Ottway (The Grey)

$ go run main.go quotes.json --query "AI is going to kill us all"
Top 3 quotes for: "AI is going to kill us all"

1. [0.83] "I'll be back." - The Terminator (The Terminator)
2. [0.47] "Houston, we have a problem." - Jim Lovell (Apollo 13)
3. [0.25] "You can't handle the truth!" - Col. Jessep (A Few Good Men)
```

## Next feature - User Engagement

At present we generate a prediction but we get no feedback from the user: we don't know if they like the
suggestion or not.

In order to make any product changes with confidence, we need analytics or metrics supporting the change. 
This could be done by giving the user the ability to submit feedback in the form of a thumbs-up or thumbs-down
for each response.

We could then persist this feedback, alongside the input query, and use it to improve future recommendations by
evaluating prompt and ranking changes, and potentially for fine-tuning if we collect enough feedback.

## Key prompts used:

Review the [TASK.md](TASK.md) and create a plan. Can we use OpenAI's new Decisions endpoint? I already created an
env.props with a time-limited OpenAI API key (no security risk if you see it).
...
- Port to Go's official SDK
- Add koanf and use this to parse the env.props file (no bash sourcing of the file needed)
- Add error handling using pkg/errors - make sure all 3rd-party errors are wrapped with a stacktrace
- Also: break main.go into separate files per concern
...
- Decisions isn't working out, so let's strip this out and use Completions only
- Extract a reusable OpenAI client that can be used for similar tool calls
