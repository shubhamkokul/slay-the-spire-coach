package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/shubhamkokul/slay-the-spire-coach/internal/session"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

const (
	defaultAddr   = "http://localhost:15526"
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

	// New run or first call — initialize session for this character.
	if gs.Player.Character != "" {
		if c.Session == nil || c.Session.Character != gs.Player.Character {
			c.Session = session.New(gs.Player.Character)
			c.store.Save(c.Session)
		}
		c.Session.Update(gs)
		c.store.Save(c.Session)

		// Restore deck into GameState so prompt builders still work.
		if len(gs.Player.DrawPile) == 0 && c.Session != nil {
			for _, d := range c.Session.Deck {
				gs.Player.DrawPile = append(gs.Player.DrawPile, state.DeckCard{
					Name: d.Display(),
					Cost: "",
				})
			}
		}
	}

	os.WriteFile(stateFilePath, body, 0644)

	return gs, json.RawMessage(body), nil
}
