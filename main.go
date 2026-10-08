package main

import (
	"fmt"
	"os"

	"github.com/openai/openai-go/v3/shared"

	"quote-finder/internal/openaitools"
	"quote-finder/internal/quotefinder"
)

func main() {
	newClient := func(key string) openaitools.Client {
		return openaitools.New(key, shared.ChatModelGPT6Luna)
	}
	if err := quotefinder.Run(os.Args[1:], "env.props", os.Getenv, newClient, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "quote-finder: %+v\n", err)
		os.Exit(1)
	}
}
