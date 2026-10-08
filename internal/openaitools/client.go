package openaitools

import (
	"context"
	"encoding/json"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/shared"
	"github.com/pkg/errors"
)

// Client makes forced, strict Chat Completions tool calls. The caller owns the
// tool schema and validates the returned arguments for its own domain.
type Client struct {
	sdk   openai.Client
	model shared.ChatModel
}

func New(apiKey string, model shared.ChatModel, options ...option.RequestOption) Client {
	options = append([]option.RequestOption{option.WithAPIKey(apiKey)}, options...)
	return Client{sdk: openai.NewClient(options...), model: model}
}

type ToolRequest struct {
	Instructions string
	Input        string
	Name         string
	Description  string
	Parameters   shared.FunctionParameters
}

func (c Client) CallStrictTool(ctx context.Context, req ToolRequest) (json.RawMessage, error) {
	completion, err := c.sdk.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model:           c.model,
		ReasoningEffort: shared.ReasoningEffortNone,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.DeveloperMessage(req.Instructions),
			openai.UserMessage(req.Input),
		},
		Tools: []openai.ChatCompletionToolUnionParam{
			openai.ChatCompletionFunctionTool(shared.FunctionDefinitionParam{
				Name:        req.Name,
				Description: openai.String(req.Description),
				Strict:      openai.Bool(true),
				Parameters:  req.Parameters,
			}),
		},
		ToolChoice:        openai.ToolChoiceOptionFunctionToolChoice(openai.ChatCompletionNamedToolChoiceFunctionParam{Name: req.Name}),
		ParallelToolCalls: openai.Bool(false),
	})
	if err != nil {
		return nil, errors.Wrap(err, "Chat Completions request")
	}
	if completion == nil || len(completion.Choices) != 1 || len(completion.Choices[0].Message.ToolCalls) != 1 {
		return nil, errors.New("Chat Completions did not make exactly one tool call")
	}
	call := completion.Choices[0].Message.ToolCalls[0]
	if call.Type != "function" || call.AsFunction().Function.Name != req.Name {
		return nil, errors.New("Chat Completions returned an unexpected tool call")
	}
	arguments := json.RawMessage(call.AsFunction().Function.Arguments)
	if !json.Valid(arguments) {
		return nil, errors.New("Chat Completions returned invalid tool arguments")
	}
	return arguments, nil
}
