package quotefinder

import (
	"context"
	"fmt"
	"github.com/openai/openai-go/v3"
	"github.com/pkg/errors"
	"io"
	"time"
)

func Run(args []string, keyPath string, getenv func(string) string, newClient func(string) openai.Client, out io.Writer) error {
	path, override, hasOverride, err := parseArgs(args)
	if err != nil {
		return err
	}
	input, err := loadInput(path, override, hasOverride)
	if err != nil {
		return err
	}
	apiKey, err := loadAPIKey(keyPath, getenv)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ranked, err := rankQuotes(ctx, newClient(apiKey), input)
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
