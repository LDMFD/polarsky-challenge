package main

import (
	"fmt"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"os"
	"quote-finder/internal/quotefinder"
)

func main() {
	newClient := func(key string) openai.Client {
		return openai.NewClient(option.WithAPIKey(key))
	}
	if err := quotefinder.Run(os.Args[1:], "env.props", os.Getenv, newClient, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "quote-finder: %+v\n", err)
		os.Exit(1)
	}
}
