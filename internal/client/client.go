package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

const (
	defaultAddr   = "http://localhost:15526"
	stateFilePath = "/tmp/sts2-state.json"
)

type STS2Client struct {
	http      *http.Client
	addr      string
	lastDeck  []state.DeckCard // last seen non-empty deck, carried across states
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

	// Update last known deck whenever the API gives us one.
	allCards := append(gs.Player.DrawPile, gs.Player.DiscardPile...)
	if len(allCards) > 0 {
		c.lastDeck = allCards
	} else if len(c.lastDeck) > 0 {
		// API returned empty piles (e.g. post-combat card reward) — restore from cache.
		gs.Player.DrawPile = c.lastDeck
	}

	// Always write latest state to tmp for debugging and future DB migration.
	os.WriteFile(stateFilePath, body, 0644)

	return gs, json.RawMessage(body), nil
}
