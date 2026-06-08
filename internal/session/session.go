package session

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

// ── Entry types ──────────────────────────────────────────────────────────────

type DeckEntry struct {
	Name     string
	Upgraded bool
	Source   string // "start", "card_reward", "shop_buy", "shop_remove", "upgrade", "event", "boon", "reconcile"
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
	Source string
	Floor  int
}

type PotionEntry struct {
	Name string
	Slot int
}

type StatusEntry struct {
	Name   string
	Amount int
}

type EnemySnapshot struct {
	Name   string
	HP     int
	MaxHP  int
	Block  int
	Intent string
	Status []StatusEntry
}

type Event struct {
	Floor  int
	Screen string
	Type   string
	Detail string
}

// ── Session ───────────────────────────────────────────────────────────────────

type Session struct {
	mu        sync.RWMutex
	Character string
	Act       int
	Floor     int
	StateType string

	// Player snapshot
	HP    int
	MaxHP int
	Gold  int

	// Collections — updated on every poll
	Deck    []DeckEntry
	Relics  []RelicEntry
	Potions []PotionEntry
	Status  []StatusEntry
	Enemies []EnemySnapshot // non-nil only in combat states

	// Run log
	Events    []Event
	StartedAt time.Time

	prev state.GameState
}

func New(character string) *Session {
	return &Session{
		Character: character,
		Deck:      startingDeck(character),
		StartedAt: time.Now(),
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

// Update syncs every aspect of the session from the latest GameState.
func (s *Session) Update(curr state.GameState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev := s.prev
	defer func() { s.prev = curr }()

	s.Act = curr.Run.Act
	s.Floor = curr.Run.Floor
	s.StateType = curr.StateType
	s.HP = curr.Player.HP
	s.MaxHP = curr.Player.MaxHP
	s.Gold = curr.Player.Gold

	s.syncPotions(curr)
	s.syncStatus(curr)
	s.syncEnemies(curr)
	s.detectRelicChanges(prev, curr)

	if state.IsCombat(curr.StateType) || curr.StateType == "card_select" || curr.StateType == "hand_select" {
		s.reconcileDeck(curr)
	}
}

func (s *Session) syncPotions(curr state.GameState) {
	s.Potions = s.Potions[:0]
	for _, p := range curr.Player.Potions {
		s.Potions = append(s.Potions, PotionEntry{Name: p.Name, Slot: p.Slot})
	}
}

func (s *Session) syncStatus(curr state.GameState) {
	s.Status = s.Status[:0]
	for _, st := range curr.Player.Status {
		s.Status = append(s.Status, StatusEntry{Name: st.Name, Amount: st.Amount})
	}
}

func (s *Session) syncEnemies(curr state.GameState) {
	if !state.IsCombat(curr.StateType) || curr.Battle == nil {
		s.Enemies = nil
		return
	}
	s.Enemies = s.Enemies[:0]
	for _, e := range curr.Battle.Enemies {
		var intents []string
		for _, i := range e.Intents {
			part := i.Title
			if i.Label != "" {
				part += " " + i.Label
			}
			intents = append(intents, part)
		}
		epow := make([]StatusEntry, len(e.Status))
		for i, p := range e.Status {
			epow[i] = StatusEntry{Name: p.Name, Amount: p.Amount}
		}
		s.Enemies = append(s.Enemies, EnemySnapshot{
			Name:   e.Name,
			HP:     e.HP,
			MaxHP:  e.MaxHP,
			Block:  e.Block,
			Intent: strings.Join(intents, ", "),
			Status: epow,
		})
	}
}

func (s *Session) detectRelicChanges(prev, curr state.GameState) {
	if len(curr.Player.Relics) == 0 {
		return
	}
	prevRelics := make(map[string]bool, len(prev.Player.Relics))
	for _, r := range prev.Player.Relics {
		prevRelics[r.Name] = true
	}
	// Save existing sources before clearing.
	existingSources := make(map[string]string, len(s.Relics))
	for _, r := range s.Relics {
		existingSources[r.Name] = r.Source
	}
	s.Relics = s.Relics[:0]
	for _, r := range curr.Player.Relics {
		source := existingSources[r.Name]
		if !prevRelics[r.Name] {
			source = relicSource(curr.StateType)
			s.logEvent(curr.Run.Floor, curr.StateType, "relic_added", r.Name)
		}
		if source == "" {
			source = "unknown"
		}
		s.Relics = append(s.Relics, RelicEntry{Name: r.Name, Source: source, Floor: curr.Run.Floor})
	}
}

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

	for name, apiCount := range apiCards {
		diff := apiCount - sessionCards[name]
		for range diff {
			upgraded := strings.HasSuffix(name, "+")
			s.Deck = append(s.Deck, DeckEntry{
				Name:     strings.TrimSuffix(name, "+"),
				Upgraded: upgraded,
				Source:   "reconcile",
				Floor:    curr.Run.Floor,
			})
			s.logEvent(curr.Run.Floor, curr.StateType, "card_added", name+" (reconciled)")
		}
	}

	for i, entry := range s.Deck {
		if !entry.Upgraded && apiCards[entry.Name+"+"] > 0 && sessionCards[entry.Name+"+"] < apiCards[entry.Name+"+"] {
			s.Deck[i].Upgraded = true
			s.logEvent(curr.Run.Floor, curr.StateType, "card_upgraded", entry.Name)
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

// ── Display ───────────────────────────────────────────────────────────────────

func (s *Session) PrintStatus() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var sb strings.Builder

	fmt.Fprintf(&sb, "\n%s\n", strings.Repeat("═", 48))
	fmt.Fprintf(&sb, "  %s  |  Act %d Floor %d  |  %s\n", s.Character, s.Act, s.Floor, s.StateType)
	fmt.Fprintf(&sb, "%s\n", strings.Repeat("─", 48))

	// Player
	fmt.Fprintf(&sb, "  HP: %d/%d    Gold: %d\n", s.HP, s.MaxHP, s.Gold)

	if len(s.Status) > 0 {
		parts := make([]string, len(s.Status))
		for i, st := range s.Status {
			parts[i] = fmt.Sprintf("%s %d", st.Name, st.Amount)
		}
		fmt.Fprintf(&sb, "  Status: %s\n", strings.Join(parts, ", "))
	}

	// Potions
	if len(s.Potions) > 0 {
		names := make([]string, len(s.Potions))
		for i, p := range s.Potions {
			names[i] = p.Name
		}
		fmt.Fprintf(&sb, "  Potions: %s\n", strings.Join(names, ", "))
	}

	// Relics
	if len(s.Relics) > 0 {
		names := make([]string, len(s.Relics))
		for i, r := range s.Relics {
			names[i] = r.Name
		}
		fmt.Fprintf(&sb, "  Relics: %s\n", strings.Join(names, ", "))
	}

	// Deck
	fmt.Fprintf(&sb, "%s\n", strings.Repeat("─", 48))
	counts := make(map[string]int)
	order := []string{}
	for _, d := range s.Deck {
		key := d.Display()
		if counts[key] == 0 {
			order = append(order, key)
		}
		counts[key]++
	}
	fmt.Fprintf(&sb, "  Deck (%d cards)\n", len(s.Deck))
	for _, name := range order {
		fmt.Fprintf(&sb, "    %dx %s\n", counts[name], name)
	}

	// Enemies (combat only)
	if len(s.Enemies) > 0 {
		fmt.Fprintf(&sb, "%s\n", strings.Repeat("─", 48))
		fmt.Fprintf(&sb, "  Enemies\n")
		for _, e := range s.Enemies {
			fmt.Fprintf(&sb, "    %s  HP %d/%d  Block %d\n", e.Name, e.HP, e.MaxHP, e.Block)
			if e.Intent != "" {
				fmt.Fprintf(&sb, "    → %s\n", e.Intent)
			}
			for _, st := range e.Status {
				fmt.Fprintf(&sb, "    [%s %d]\n", st.Name, st.Amount)
			}
		}
	}

	fmt.Fprintf(&sb, "%s\n", strings.Repeat("═", 48))
	return sb.String()
}

func (s *Session) PrintDeck() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var sb strings.Builder
	counts := make(map[string]int)
	order := []string{}
	for _, d := range s.Deck {
		key := d.Display()
		if counts[key] == 0 {
			order = append(order, key)
		}
		counts[key]++
	}
	fmt.Fprintf(&sb, "Deck (%d cards) — %s Act %d Floor %d\n", len(s.Deck), s.Character, s.Act, s.Floor)
	sb.WriteString(strings.Repeat("─", 40))
	sb.WriteByte('\n')
	for _, name := range order {
		fmt.Fprintf(&sb, "  %dx %s\n", counts[name], name)
	}
	return sb.String()
}

func (s *Session) DeckNames() []string {
	names := make([]string, len(s.Deck))
	for i, d := range s.Deck {
		names[i] = d.Display()
	}
	return names
}
