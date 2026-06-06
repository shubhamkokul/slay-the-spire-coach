package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

const defaultAddr = "http://localhost:15526"

type STS2Client struct {
	http *http.Client
	addr string
}

func New(addr string) *STS2Client {
	if addr == "" {
		addr = defaultAddr
	}
	return &STS2Client{
		http: &http.Client{Timeout: 5 * time.Second},
		addr: addr,
	}
}

func (c *STS2Client) Ping() error {
	resp, err := c.http.Get(c.addr + "/")
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

func (c *STS2Client) GetState() (state.GameState, json.RawMessage, error) {
	resp, err := c.http.Get(c.addr + "/api/v1/singleplayer")
	if err != nil {
		return state.GameState{}, nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return state.GameState{}, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return state.GameState{}, nil, fmt.Errorf("api returned %d: %s", resp.StatusCode, string(body))
	}

	var gs state.GameState
	if err := json.Unmarshal(body, &gs); err != nil {
		return state.GameState{}, nil, fmt.Errorf("parse error: %w", err)
	}

	return gs, json.RawMessage(body), nil
}
