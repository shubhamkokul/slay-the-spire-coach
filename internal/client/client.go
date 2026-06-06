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

	// During combat, capture the full deck: hand + draw + discard.
	// Outside combat the mod omits deck data entirely, so we restore from cache.
	if state.IsCombat(gs.StateType) || gs.StateType == "card_select" || gs.StateType == "hand_select" {
		full := make([]state.DeckCard, 0, len(gs.Player.Hand)+len(gs.Player.DrawPile)+len(gs.Player.DiscardPile))
		for _, card := range gs.Player.Hand {
			name := card.Name
			if card.IsUpgraded {
				name += "+"
			}
			full = append(full, state.DeckCard{Name: name, Cost: card.Cost})
		}
		full = append(full, gs.Player.DrawPile...)
		full = append(full, gs.Player.DiscardPile...)
		if len(full) > 0 {
			c.lastDeck = full
		}
	} else if len(gs.Player.DrawPile) == 0 && len(c.lastDeck) > 0 {
		gs.Player.DrawPile = c.lastDeck
	}

	// Always write latest state to tmp for debugging and future DB migration.
	os.WriteFile(stateFilePath, body, 0644)

	return gs, json.RawMessage(body), nil
}
