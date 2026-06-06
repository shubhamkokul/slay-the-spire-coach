package claude

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/shubhamkokul/slay-the-spire-coach/internal/prompt"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

const defaultModel = "deepseek-r1:14b"
const ollamaAddr = "http://localhost:11434"

type Client struct {
	http  *http.Client
	model string
}

func New(model string) *Client {
	if model == "" {
		model = defaultModel
	}
	return &Client{
		http:  &http.Client{Timeout: 60 * time.Second},
		model: model,
	}
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
	Stream   bool      `json:"stream"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatChunk struct {
	Message message `json:"message"`
	Done    bool    `json:"done"`
}

func (c *Client) Advise(ctx context.Context, trigger *state.Trigger) error {
	body, _ := json.Marshal(chatRequest{
		Model: c.model,
		Messages: []message{
			{Role: "system", Content: prompt.System(trigger.State.StateType)},
			{Role: "user", Content: prompt.Build(trigger)},
		},
		Stream: true,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", ollamaAddr+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	fmt.Printf("\n[%s]\n", trigger.Reason)

	scanner := bufio.NewScanner(resp.Body)
	inThink := false
	for scanner.Scan() {
		var chunk chatChunk
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		tok := chunk.Message.Content
		if tok != "" {
			// Strip deepseek-r1 <think>...</think> reasoning blocks
			for {
				if inThink {
					if end := indexOf(tok, "</think>"); end >= 0 {
						tok = tok[end+len("</think>"):]
						inThink = false
					} else {
						tok = ""
						break
					}
				} else {
					if start := indexOf(tok, "<think>"); start >= 0 {
						fmt.Print(tok[:start])
						tok = tok[start+len("<think>"):]
						inThink = true
					} else {
						fmt.Print(tok)
						break
					}
				}
			}
		}
		if chunk.Done {
			break
		}
	}
	fmt.Println()

	return scanner.Err()
}

func indexOf(s, sub string) int {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
