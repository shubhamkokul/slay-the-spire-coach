package claude

import (
	"context"
	"fmt"
	"os"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/prompt"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

type Client struct {
	api anthropic.Client
}

func New() (*Client, error) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}
	return &Client{api: anthropic.NewClient()}, nil
}

func (c *Client) Advise(ctx context.Context, trigger *state.Trigger) error {
	stream := c.api.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 200,
		System: []anthropic.TextBlockParam{{
			Text: prompt.System(trigger.State.StateType),
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(prompt.Build(trigger))),
		},
	})

	fmt.Printf("\n[%s]\n", trigger.Reason)

	for stream.Next() {
		event := stream.Current()
		switch ev := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch delta := ev.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				fmt.Print(delta.Text)
			}
		}
	}
	fmt.Println()

	return stream.Err()
}
