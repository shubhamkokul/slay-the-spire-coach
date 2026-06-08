package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/shubhamkokul/slay-the-spire-coach/internal/session"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

const (
	defaultAddr   = "http://localhost:15526"
	pollInterval  = 2 * time.Second
	stateFilePath = "/tmp/sts2-state.json"
)

type STS2Client struct {
	http    *http.Client
	addr    string
	Session *session.Session
	store   session.Store
}

func New(addr string, store session.Store) *STS2Client {
	if addr == "" {
		addr = defaultAddr
	}
	return &STS2Client{
		http:  &http.Client{Timeout: 5 * time.Second},
		addr:  addr,
		store: store,
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

// ResetSession clears the current session so the next poll initializes
// a fresh one for whatever character is currently active.
func (c *STS2Client) ResetSession() {
	c.Session = nil
}

// Poll starts a background goroutine that fetches game state every 2 seconds
// and keeps the session and store up to date automatically.
func (c *STS2Client) Poll(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, _, err := c.GetState(); err != nil {
					log.Printf("[poll] %v", err)
				}
			}
		}
	}()
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

	if gs.Player.Character != "" {
		// New run or character changed — initialize fresh session.
		if c.Session == nil || c.Session.Character != gs.Player.Character {
			c.Session = session.New(gs.Player.Character)
		}
		c.Session.Update(gs)
		c.store.Save(c.Session)

		// Restore deck into GameState for prompt builders.
		if len(gs.Player.DrawPile) == 0 && len(c.Session.Deck) > 0 {
			for _, d := range c.Session.Deck {
				gs.Player.DrawPile = append(gs.Player.DrawPile, state.DeckCard{
					Name: d.Display(),
				})
			}
		}
	}

	os.WriteFile(stateFilePath, body, 0644)

	return gs, json.RawMessage(body), nil
}
