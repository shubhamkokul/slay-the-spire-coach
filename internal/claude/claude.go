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
	return &Client{
		api: anthropic.NewClient(),
	}, nil
}

func (c *Client) Advise(ctx context.Context, trigger *state.Trigger) error {
	userMsg := prompt.Build(trigger)

	fmt.Printf("\n[%s]\n", trigger.Reason)

	stream := c.api.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_6,
		MaxTokens: 150,
		System: []anthropic.TextBlockParam{{
			Text: prompt.System(trigger.State.StateType),
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userMsg)),
		},
	})

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

	if err := stream.Err(); err != nil {
		return fmt.Errorf("stream error: %w", err)
	}
	return nil
}

