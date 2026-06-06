package claude

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/prompt"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

const (
	inputPricePer1M  = 1.00 // Haiku 4.5
	outputPricePer1M = 5.00
)

type Client struct {
	api         anthropic.Client
	sessionIn   int64
	sessionOut  int64
	log         *os.File
}

func New() (*Client, error) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY not set")
	}

	logDir := filepath.Join(os.Getenv("HOME"), ".local", "share", "sts2-coach")
	var logFile *os.File
	if err := os.MkdirAll(logDir, 0755); err == nil {
		logPath := filepath.Join(logDir, time.Now().Format("2006-01-02")+".log")
		logFile, _ = os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	}

	return &Client{
		api: anthropic.NewClient(),
		log: logFile,
	}, nil
}

func (c *Client) Advise(ctx context.Context, trigger *state.Trigger) error {
	userMsg := prompt.Build(trigger)

	fmt.Printf("\n[%s]\n", trigger.Reason)

	stream := c.api.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: 200,
		System: []anthropic.TextBlockParam{{
			Text: prompt.System(trigger.State.StateType),
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userMsg)),
		},
	})

	var response strings.Builder
	var accumulated anthropic.Message

	for stream.Next() {
		event := stream.Current()
		accumulated.Accumulate(event)
		switch ev := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch delta := ev.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				fmt.Print(delta.Text)
				response.WriteString(delta.Text)
			}
		}
	}
	fmt.Println()

	if err := stream.Err(); err != nil {
		return fmt.Errorf("stream error: %w", err)
	}

	// Track tokens
	in := accumulated.Usage.InputTokens
	out := accumulated.Usage.OutputTokens
	c.sessionIn += in
	c.sessionOut += out
	cost := c.sessionCostFloat()
	fmt.Printf("[tokens: +%din +%dout | session: $%.4f]\n", in, out, cost)

	// Log to file
	if c.log != nil {
		fmt.Fprintf(c.log, "\n[%s] %s | %s\n%s\n[tokens: %din %dout]\n",
			time.Now().Format("15:04:05"),
			trigger.Reason,
			trigger.State.StateType,
			response.String(),
			in, out,
		)
	}

	return nil
}

func (c *Client) sessionCostFloat() float64 {
	return float64(c.sessionIn)/1_000_000*inputPricePer1M +
		float64(c.sessionOut)/1_000_000*outputPricePer1M
}

func (c *Client) SessionSummary() string {
	return fmt.Sprintf("Session: %d input + %d output tokens = $%.4f",
		c.sessionIn, c.sessionOut, c.sessionCostFloat())
}

func (c *Client) Close() {
	if c.log != nil {
		c.log.Close()
	}
}
