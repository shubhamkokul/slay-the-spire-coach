package session

import (
	"fmt"
	"strings"
	"time"

	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

type DeckEntry struct {
	Name     string
	Upgraded bool
	Source   string // "start", "card_reward", "shop_buy", "shop_remove", "upgrade", "event", "boon"
	Floor    int
}

func (d DeckEntry) Display() string {
	if d.Upgraded {
		return d.Name + "+"
	}
	return d.Name
}

type RelicEntry struct {
	Name   string
	Source string // "combat_reward", "shop", "boss_relic", "treasure", "event", "boon"
	Floor  int
}

type Event struct {
	Floor  int
	Screen string
	Type   string // "card_added", "card_upgraded", "card_removed", "relic_added"
	Detail string
}

type Session struct {
	Character string
	Act       int
	Floor     int
	Deck      []DeckEntry
	Relics    []RelicEntry
	Events    []Event
	StartedAt time.Time
	prev      state.GameState
}

func New(character string) *Session {
	return &Session{
		Character: character,
		Deck:      startingDeck(character),
		StartedAt: time.Now(),
	}
}

// Update processes the latest game state, detects events, and keeps session in sync.
func (s *Session) Update(curr state.GameState) {
	prev := s.prev
	defer func() { s.prev = curr }()

	s.Act = curr.Run.Act
	s.Floor = curr.Run.Floor

	// Relic change detection — API always returns relics, all states.
	s.detectRelicChanges(prev, curr)

	// Combat reconciliation — API gives ground truth here.
	if state.IsCombat(curr.StateType) || curr.StateType == "card_select" || curr.StateType == "hand_select" {
		s.reconcileDeck(curr)
	}
}

// reconcileDeck syncs the session deck against the full card list from the API.
// Adds any cards the event detection missed (events, mod interactions, etc.).
func (s *Session) reconcileDeck(curr state.GameState) {
	apiCards := make(map[string]int)
	for _, c := range curr.Player.Hand {
		name := c.Name
		if c.IsUpgraded {
			name += "+"
		}
		apiCards[name]++
	}
	for _, c := range curr.Player.DrawPile {
		apiCards[c.Name]++
	}
	for _, c := range curr.Player.DiscardPile {
		apiCards[c.Name]++
	}

	sessionCards := make(map[string]int)
	for _, d := range s.Deck {
		sessionCards[d.Display()]++
	}

	// Add any cards the API has that the session doesn't.
	for name, apiCount := range apiCards {
		diff := apiCount - sessionCards[name]
		for range diff {
			upgraded := strings.HasSuffix(name, "+")
			baseName := strings.TrimSuffix(name, "+")
			s.Deck = append(s.Deck, DeckEntry{
				Name:     baseName,
				Upgraded: upgraded,
				Source:   "reconcile",
				Floor:    curr.Run.Floor,
			})
			s.logEvent(curr.Run.Floor, curr.StateType, "card_added", name+" (reconciled)")
		}
	}

	// Mark upgrades: if API has Card+ but session has Card, upgrade it.
	for i, entry := range s.Deck {
		if !entry.Upgraded && apiCards[entry.Name+"+"] > 0 && sessionCards[entry.Name+"+"] < apiCards[entry.Name+"+"] {
			s.Deck[i].Upgraded = true
			s.logEvent(curr.Run.Floor, curr.StateType, "card_upgraded", entry.Name)
		}
	}
}

func (s *Session) detectRelicChanges(prev, curr state.GameState) {
	if len(curr.Player.Relics) == 0 {
		return
	}
	prevRelics := make(map[string]bool)
	for _, r := range prev.Player.Relics {
		prevRelics[r.Name] = true
	}
	for _, r := range curr.Player.Relics {
		if !prevRelics[r.Name] {
			source := relicSource(curr.StateType)
			s.Relics = append(s.Relics, RelicEntry{
				Name:   r.Name,
				Source: source,
				Floor:  curr.Run.Floor,
			})
			s.logEvent(curr.Run.Floor, curr.StateType, "relic_added", r.Name)
		}
	}
}

func relicSource(stateType string) string {
	switch stateType {
	case "rewards":
		return "combat_reward"
	case "shop":
		return "shop"
	case "relic_select":
		return "boss_relic"
	case "treasure":
		return "treasure"
	case "event":
		return "event"
	default:
		return "unknown"
	}
}

func (s *Session) logEvent(floor int, screen, eventType, detail string) {
	s.Events = append(s.Events, Event{
		Floor:  floor,
		Screen: screen,
		Type:   eventType,
		Detail: detail,
	})
}

// PrintDeck returns a formatted deck list for display.
func (s *Session) PrintDeck() string {
	if len(s.Deck) == 0 {
		return "Deck is empty — not yet seen in combat."
	}

	counts := make(map[string]int)
	sources := make(map[string]string)
	order := []string{}

	for _, d := range s.Deck {
		key := d.Display()
		if counts[key] == 0 {
			order = append(order, key)
			sources[key] = d.Source
		}
		counts[key]++
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Deck (%d cards) — %s, Act %d Floor %d\n", len(s.Deck), s.Character, s.Act, s.Floor)
	sb.WriteString(strings.Repeat("─", 40))
	sb.WriteByte('\n')
	for _, name := range order {
		fmt.Fprintf(&sb, "  %dx %s\n", counts[name], name)
	}
	return sb.String()
}

// Deck returns all card names for use in prompts.
func (s *Session) DeckNames() []string {
	names := make([]string, len(s.Deck))
	for i, d := range s.Deck {
		names[i] = d.Display()
	}
	return names
}
