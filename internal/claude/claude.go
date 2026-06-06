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

const defaultModel = "llama3.1:8b"
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
	for scanner.Scan() {
		var chunk chatChunk
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			continue
		}
		if chunk.Message.Content != "" {
			fmt.Print(chunk.Message.Content)
		}
		if chunk.Done {
			break
		}
	}
	fmt.Println()

	return scanner.Err()
}
