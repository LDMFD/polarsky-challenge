package quotefinder

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/pkg/errors"

	"quote-finder/internal/openaitools"
)

func Run(args []string, keyPath string, getenv func(string) string, newClient func(string) openaitools.Client, out io.Writer) error {
	opts, err := parseArgs(args)
	if err != nil {
		return err
	}
	input, err := loadInput(opts.Path, opts.Query, opts.HasQuery)
	if err != nil {
		return err
	}
	apiKey, err := loadAPIKey(keyPath, getenv)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	client := newClient(apiKey)
	ranked, err := rankQuotes(ctx, client, input)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Top %d quotes for: %q\n\n", min(3, len(ranked)), input.Query); err != nil {
		return errors.Wrap(err, "write heading")
	}
	for i := 0; i < min(3, len(ranked)); i++ {
		q := ranked[i]
		if _, err := fmt.Fprintf(out, "%d. [%.2f] %q - %s (%s)\n", i+1, q.Score, q.Quote.Text, q.Quote.Character, q.Quote.Movie); err != nil {
			return errors.Wrap(err, "write quote")
		}
	}
	return nil
}
