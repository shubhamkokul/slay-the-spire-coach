package claude

import (
	"context"
	"encoding/json"
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
	api       anthropic.Client
	sessionIn int64
	sessionOut int64
	log       *os.File
	history   []anthropic.MessageParam
	sysPrompt string
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

// Advise gives state-specific advice and starts a fresh conversation thread.
// Follow-up questions via Ask will have this advice in context.
func (c *Client) Advise(ctx context.Context, trigger *state.Trigger) error {
	userMsg := prompt.Build(trigger)
	sys := prompt.System(trigger.State.StateType)

	c.sysPrompt = sys
	c.history = []anthropic.MessageParam{
		anthropic.NewUserMessage(anthropic.NewTextBlock(userMsg)),
	}

	fmt.Printf("\n[%s]\n", trigger.Reason)

	reply, in, out, err := c.stream(ctx, sys, c.history, 200)
	if err != nil {
		return err
	}

	c.history = append(c.history, anthropic.NewAssistantMessage(anthropic.NewTextBlock(reply)))
	c.track(in, out)

	if c.log != nil {
		fmt.Fprintf(c.log, "\n[%s] %s | %s\n%s\n[tokens: %din %dout]\n",
			time.Now().Format("15:04:05"), trigger.Reason, trigger.State.StateType, reply, in, out)
	}
	return nil
}

// Ask answers a follow-up question using the current conversation thread.
// If no advice has been given yet, it includes the full game state in the question.
func (c *Client) Ask(ctx context.Context, question string, gs state.GameState, raw json.RawMessage) error {
	var userMsg string
	if len(c.history) == 0 {
		// No prior advice — include full game state so Claude has context
		userMsg = prompt.BuildQuestion(question, gs, raw)
	} else {
		userMsg = question
	}

	c.history = append(c.history, anthropic.NewUserMessage(anthropic.NewTextBlock(userMsg)))

	sys := c.sysPrompt
	if sys == "" {
		sys = prompt.SystemQuestion()
	}

	fmt.Printf("\n[question]\n")

	reply, in, out, err := c.stream(ctx, sys, c.history, 400)
	if err != nil {
		return err
	}

	c.history = append(c.history, anthropic.NewAssistantMessage(anthropic.NewTextBlock(reply)))
	c.track(in, out)

	if c.log != nil {
		fmt.Fprintf(c.log, "\n[%s] question\n%s\n→ %s\n[tokens: %din %dout]\n",
			time.Now().Format("15:04:05"), question, reply, in, out)
	}
	return nil
}

func (c *Client) stream(ctx context.Context, sys string, messages []anthropic.MessageParam, maxTokens int64) (string, int64, int64, error) {
	s := c.api.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeHaiku4_5,
		MaxTokens: maxTokens,
		System:    []anthropic.TextBlockParam{{Text: sys}},
		Messages:  messages,
	})

	var sb strings.Builder
	var accumulated anthropic.Message

	for s.Next() {
		event := s.Current()
		accumulated.Accumulate(event)
		switch ev := event.AsAny().(type) {
		case anthropic.ContentBlockDeltaEvent:
			switch delta := ev.Delta.AsAny().(type) {
			case anthropic.TextDelta:
				fmt.Print(delta.Text)
				sb.WriteString(delta.Text)
			}
		}
	}
	fmt.Println()

	if err := s.Err(); err != nil {
		return "", 0, 0, fmt.Errorf("stream error: %w", err)
	}

	return sb.String(), accumulated.Usage.InputTokens, accumulated.Usage.OutputTokens, nil
}

func (c *Client) track(in, out int64) {
	c.sessionIn += in
	c.sessionOut += out
	fmt.Printf("[tokens: +%din +%dout | session: $%.4f]\n", in, out, c.sessionCostFloat())
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
