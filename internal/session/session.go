package session

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

// ── Entry types ───────────────────────────────────────────────────────────────

type DeckEntry struct {
	Name      string
	Upgraded  bool
	Temporary bool   // true for Status cards added during combat — purged on combat exit
	Source    string // "start", "card_reward", "shop_buy", "shop_remove", "upgrade", "event", "boon", "reconcile"
	Floor     int
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

// ── Versioning ────────────────────────────────────────────────────────────────

// Change records one atomic change at a specific version.
type Change struct {
	Field  string // "hp", "gold", "floor", "deck", "relic", "potion", "status"
	Type   string // "added", "removed", "upgraded", "changed"
	Detail string // human-readable: "Shatter", "75→62", "Floor 3→4"
}

// VersionEntry is a snapshot of what changed at a given version.
type VersionEntry struct {
	Version   int64
	Timestamp time.Time
	Floor     int
	Act       int
	StateType string
	Changes   []Change
}

// ── Session ───────────────────────────────────────────────────────────────────

type Session struct {
	mu        sync.RWMutex
	Character string
	Act       int
	Floor     int
	StateType string
	Version   int64
	VersionLog []VersionEntry

	// Player snapshot
	HP    int
	MaxHP int
	Gold  int

	// Collections — updated on every poll
	Deck    []DeckEntry
	Relics  []RelicEntry
	Potions []PotionEntry
	Status  []StatusEntry
	Enemies []EnemySnapshot

	// Legacy event log — kept for compatibility
	Events    []Event
	StartedAt time.Time

	pendingCardReward []string // card names offered at last card_reward screen
	prev              state.GameState
}

func New(character string) *Session {
	return &Session{
		Character: character,
		Deck:      startingDeck(character),
		StartedAt: time.Now(),
	}
}

// ── Update ────────────────────────────────────────────────────────────────────

func (s *Session) Update(curr state.GameState) {
	s.mu.Lock()
	defer s.mu.Unlock()

	prev := s.prev
	defer func() { s.prev = curr }()

	var changes []Change

	// Floor / act
	if prev.Run.Floor != curr.Run.Floor && curr.Run.Floor > 0 {
		changes = append(changes, Change{
			Field:  "floor",
			Type:   "changed",
			Detail: fmt.Sprintf("Floor %d→%d", prev.Run.Floor, curr.Run.Floor),
		})
	}

	// HP
	if prev.Player.HP != curr.Player.HP && curr.Player.HP > 0 {
		changes = append(changes, Change{
			Field:  "hp",
			Type:   "changed",
			Detail: fmt.Sprintf("%d→%d", prev.Player.HP, curr.Player.HP),
		})
	}

	// Gold
	if prev.Player.Gold != curr.Player.Gold && curr.Player.Gold > 0 {
		changes = append(changes, Change{
			Field:  "gold",
			Type:   "changed",
			Detail: fmt.Sprintf("%d→%d", prev.Player.Gold, curr.Player.Gold),
		})
	}

	// Relics
	relicChanges := s.syncRelics(prev, curr)
	changes = append(changes, relicChanges...)

	// Potions
	potionChanges := s.syncPotions(prev, curr)
	changes = append(changes, potionChanges...)

	// Status effects
	s.syncStatus(curr)

	// Enemies
	s.syncEnemies(curr)

	// Purge temporary cards when leaving combat.
	wasInCombat := state.IsCombat(prev.StateType) || prev.StateType == "hand_select"
	nowInCombat := state.IsCombat(curr.StateType) || curr.StateType == "hand_select"
	if wasInCombat && !nowInCombat {
		s.purgeTempCards()
	}

	// Snapshot offered cards on entering card_reward so reconcile can
	// attribute the pick correctly instead of logging it as "reconcile".
	if curr.StateType == "card_reward" && curr.CardReward != nil {
		s.pendingCardReward = s.pendingCardReward[:0]
		for _, c := range curr.CardReward.Cards {
			s.pendingCardReward = append(s.pendingCardReward, c.Name)
		}
	}

	// card_select: API gives us the full deck in card_select.cards.
	// Sync it directly and detect what operation just completed on transition out.
	var deckChanges []Change
	if curr.StateType == "card_select" && curr.CardSelect != nil {
		deckChanges = s.syncFromCardSelect(curr)
	} else if prev.StateType == "card_select" && prev.CardSelect != nil {
		deckChanges = s.applyCardSelectResult(prev, curr)
	} else if nowInCombat {
		deckChanges = s.reconcileDeck(curr)
	}
	changes = append(changes, deckChanges...)

	// Bump version if anything changed.
	if len(changes) > 0 {
		s.Version++
		s.VersionLog = append(s.VersionLog, VersionEntry{
			Version:   s.Version,
			Timestamp: time.Now(),
			Floor:     curr.Run.Floor,
			Act:       curr.Run.Act,
			StateType: curr.StateType,
			Changes:   changes,
		})
	}

	s.Act = curr.Run.Act
	s.Floor = curr.Run.Floor
	s.StateType = curr.StateType
	s.HP = curr.Player.HP
	s.MaxHP = curr.Player.MaxHP
	s.Gold = curr.Player.Gold
}

func (s *Session) syncRelics(prev, curr state.GameState) []Change {
	if len(curr.Player.Relics) == 0 {
		return nil
	}
	prevRelics := make(map[string]bool, len(prev.Player.Relics))
	for _, r := range prev.Player.Relics {
		prevRelics[r.Name] = true
	}
	existingSources := make(map[string]string, len(s.Relics))
	for _, r := range s.Relics {
		existingSources[r.Name] = r.Source
	}

	var changes []Change
	s.Relics = s.Relics[:0]
	for _, r := range curr.Player.Relics {
		source := existingSources[r.Name]
		if !prevRelics[r.Name] {
			source = relicSource(curr.StateType)
			changes = append(changes, Change{Field: "relic", Type: "added", Detail: r.Name})
			s.logEvent(curr.Run.Floor, curr.StateType, "relic_added", r.Name)
		}
		if source == "" {
			source = "unknown"
		}
		s.Relics = append(s.Relics, RelicEntry{Name: r.Name, Source: source, Floor: curr.Run.Floor})
	}
	return changes
}

func (s *Session) syncPotions(prev, curr state.GameState) []Change {
	prevPotions := make(map[string]bool, len(prev.Player.Potions))
	for _, p := range prev.Player.Potions {
		prevPotions[p.Name] = true
	}
	currPotions := make(map[string]bool, len(curr.Player.Potions))
	for _, p := range curr.Player.Potions {
		currPotions[p.Name] = true
	}

	var changes []Change
	for name := range currPotions {
		if !prevPotions[name] {
			changes = append(changes, Change{Field: "potion", Type: "added", Detail: name})
		}
	}
	for name := range prevPotions {
		if !currPotions[name] {
			changes = append(changes, Change{Field: "potion", Type: "removed", Detail: name + " (used/discarded)"})
		}
	}

	s.Potions = s.Potions[:0]
	for _, p := range curr.Player.Potions {
		s.Potions = append(s.Potions, PotionEntry{Name: p.Name, Slot: p.Slot})
	}
	return changes
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

// syncFromCardSelect rebuilds the permanent deck from card_select.cards,
// which is the authoritative full deck the API exposes during this state.
func (s *Session) syncFromCardSelect(curr state.GameState) []Change {
	if curr.CardSelect == nil {
		return nil
	}
	apiCards := make(map[string]int)
	for _, c := range curr.CardSelect.Cards {
		name := c.Name
		if c.IsUpgraded {
			name += "+"
		}
		apiCards[name]++
	}

	sessionCards := make(map[string]int)
	for _, d := range s.Deck {
		if !d.Temporary {
			sessionCards[d.Display()]++
		}
	}

	var changes []Change
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
			changes = append(changes, Change{Field: "deck", Type: "added", Detail: name + " (card_select sync)"})
		}
	}
	return changes
}

// applyCardSelectResult detects what changed when leaving a card_select screen.
func (s *Session) applyCardSelectResult(prev, curr state.GameState) []Change {
	if prev.CardSelect == nil {
		return nil
	}
	screenType := prev.CardSelect.ScreenType
	var changes []Change

	switch screenType {
	case "upgrade":
		if c := s.applyUpgrade(prev.CardSelect.Cards, curr.Run.Floor); c != nil {
			changes = append(changes, *c)
		}
	case "transform":
		changes = append(changes, Change{Field: "deck", Type: "changed", Detail: "transform pending — replacement via card_reward"})
		s.logEvent(curr.Run.Floor, "card_select", "card_transform_pending", prev.CardSelect.Prompt)
	case "remove":
		changes = append(changes, Change{Field: "deck", Type: "changed", Detail: "remove pending"})
		s.logEvent(curr.Run.Floor, "card_select", "card_remove_pending", prev.CardSelect.Prompt)
	}

	return changes
}

func (s *Session) applyUpgrade(offered []state.Card, floor int) *Change {
	for _, c := range offered {
		for i, d := range s.Deck {
			if d.Name == c.Name && !d.Upgraded && !d.Temporary {
				s.Deck[i].Upgraded = true
				s.logEvent(floor, "card_select", "card_upgraded", c.Name)
				ch := Change{Field: "deck", Type: "upgraded", Detail: c.Name}
				return &ch
			}
		}
	}
	return nil
}

func (s *Session) reconcileDeck(curr state.GameState) []Change {
	// Build API card map with type info from hand (has type), name-only from piles.
	type apiCard struct {
		count    int
		cardType string
	}
	apiCards := make(map[string]*apiCard)

	for _, c := range curr.Player.Hand {
		name := c.Name
		if c.IsUpgraded {
			name += "+"
		}
		if apiCards[name] == nil {
			apiCards[name] = &apiCard{cardType: c.Type}
		}
		apiCards[name].count++
	}
	for _, c := range curr.Player.DrawPile {
		if apiCards[c.Name] == nil {
			apiCards[c.Name] = &apiCard{cardType: c.Type}
		}
		apiCards[c.Name].count++
	}
	for _, c := range curr.Player.DiscardPile {
		if apiCards[c.Name] == nil {
			apiCards[c.Name] = &apiCard{cardType: c.Type}
		}
		apiCards[c.Name].count++
	}

	sessionCards := make(map[string]int)
	for _, d := range s.Deck {
		sessionCards[d.Display()]++
	}

	// Build lookup of cards offered at last card_reward for source attribution.
	offeredAtReward := make(map[string]bool, len(s.pendingCardReward))
	for _, name := range s.pendingCardReward {
		offeredAtReward[name] = true
	}

	var changes []Change
	for name, info := range apiCards {
		diff := info.count - sessionCards[name]
		for range diff {
			upgraded := strings.HasSuffix(name, "+")
			baseName := strings.TrimSuffix(name, "+")
			temp := isTemporary(baseName, info.cardType)

			source := "reconcile"
			if offeredAtReward[baseName] {
				source = "card_reward"
			}

			s.Deck = append(s.Deck, DeckEntry{
				Name:      baseName,
				Upgraded:  upgraded,
				Temporary: temp,
				Source:    source,
				Floor:     curr.Run.Floor,
			})
			label := name
			if temp {
				label += " (temp)"
			}
			changes = append(changes, Change{Field: "deck", Type: "added", Detail: fmt.Sprintf("%s (%s)", label, source)})
			s.logEvent(curr.Run.Floor, curr.StateType, "card_added", label)
		}
	}

	// Clear pending reward once reconciled.
	s.pendingCardReward = s.pendingCardReward[:0]

	// Only check upgrades on permanent cards — skip temporaries.
	for i, entry := range s.Deck {
		if entry.Temporary || entry.Upgraded {
			continue
		}
		if ac, ok := apiCards[entry.Name+"+"]; ok && ac.count > sessionCards[entry.Name+"+"] {
			s.Deck[i].Upgraded = true
			changes = append(changes, Change{Field: "deck", Type: "upgraded", Detail: entry.Name})
			s.logEvent(curr.Run.Floor, curr.StateType, "card_upgraded", entry.Name)
		}
	}
	return changes
}

// purgeTempCards removes all cards marked temporary from the deck.
// Called when transitioning out of combat so the permanent deck is clean.
func (s *Session) purgeTempCards() {
	kept := s.Deck[:0]
	for _, d := range s.Deck {
		if !d.Temporary {
			kept = append(kept, d)
		}
	}
	s.Deck = kept
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
	fmt.Fprintf(&sb, "\n%s\n", strings.Repeat("═", 50))
	fmt.Fprintf(&sb, "  %s  |  Act %d Floor %d  |  v%d\n", s.Character, s.Act, s.Floor, s.Version)
	fmt.Fprintf(&sb, "  Screen: %s\n", s.StateType)
	fmt.Fprintf(&sb, "%s\n", strings.Repeat("─", 50))

	fmt.Fprintf(&sb, "  HP: %d/%d    Gold: %d\n", s.HP, s.MaxHP, s.Gold)

	if len(s.Status) > 0 {
		parts := make([]string, len(s.Status))
		for i, st := range s.Status {
			parts[i] = fmt.Sprintf("%s %d", st.Name, st.Amount)
		}
		fmt.Fprintf(&sb, "  Status: %s\n", strings.Join(parts, ", "))
	}

	if len(s.Potions) > 0 {
		names := make([]string, len(s.Potions))
		for i, p := range s.Potions {
			names[i] = p.Name
		}
		fmt.Fprintf(&sb, "  Potions: %s\n", strings.Join(names, ", "))
	}

	if len(s.Relics) > 0 {
		names := make([]string, len(s.Relics))
		for i, r := range s.Relics {
			names[i] = r.Name
		}
		fmt.Fprintf(&sb, "  Relics: %s\n", strings.Join(names, ", "))
	}

	fmt.Fprintf(&sb, "%s\n", strings.Repeat("─", 50))
	counts := make(map[string]int)
	order := []string{}
	for _, d := range s.Deck {
		key := d.Display()
		if counts[key] == 0 {
			order = append(order, key)
		}
		counts[key]++
	}
	permCount := 0
	for _, d := range s.Deck {
		if !d.Temporary {
			permCount++
		}
	}
	fmt.Fprintf(&sb, "  Deck (%d cards)\n", permCount)
	for _, name := range order {
		suffix := ""
		// Check if this card is temporary in the deck.
		for _, d := range s.Deck {
			if d.Display() == name && d.Temporary {
				suffix = " (temp)"
				break
			}
		}
		fmt.Fprintf(&sb, "    %dx %s%s\n", counts[name], name, suffix)
	}

	if len(s.Enemies) > 0 {
		fmt.Fprintf(&sb, "%s\n", strings.Repeat("─", 50))
		fmt.Fprintf(&sb, "  Enemies\n")
		for _, e := range s.Enemies {
			fmt.Fprintf(&sb, "    %-20s HP %d/%d  Block %d\n", e.Name, e.HP, e.MaxHP, e.Block)
			if e.Intent != "" {
				fmt.Fprintf(&sb, "    → %s\n", e.Intent)
			}
			for _, st := range e.Status {
				fmt.Fprintf(&sb, "    [%s %d]\n", st.Name, st.Amount)
			}
		}
	}

	fmt.Fprintf(&sb, "%s\n", strings.Repeat("═", 50))
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
	fmt.Fprintf(&sb, "Deck (%d cards) — %s Act %d Floor %d  v%d\n", len(s.Deck), s.Character, s.Act, s.Floor, s.Version)
	sb.WriteString(strings.Repeat("─", 40))
	sb.WriteByte('\n')
	for _, name := range order {
		fmt.Fprintf(&sb, "  %dx %s\n", counts[name], name)
	}
	return sb.String()
}

// PrintHistory shows the version log — every change across the run.
func (s *Session) PrintHistory() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.VersionLog) == 0 {
		return "No changes recorded yet.\n"
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "Run history — %s  (%d versions)\n", s.Character, s.Version)
	sb.WriteString(strings.Repeat("─", 50))
	sb.WriteByte('\n')
	for _, v := range s.VersionLog {
		fmt.Fprintf(&sb, "  v%-4d  Act %d Floor %-3d  [%s]\n", v.Version, v.Act, v.Floor, v.StateType)
		for _, c := range v.Changes {
			fmt.Fprintf(&sb, "         %-8s %-10s %s\n", c.Field, c.Type, c.Detail)
		}
	}
	return sb.String()
}

// RecentEvents returns the last N notable changes as a compact string
// for inclusion in Claude prompts as run context.
func (s *Session) RecentEvents(n int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.VersionLog) == 0 {
		return ""
	}
	start := max(0, len(s.VersionLog)-n)
	var parts []string
	for _, v := range s.VersionLog[start:] {
		for _, c := range v.Changes {
			// Only surface meaningful events — skip hp/gold noise.
			if c.Field == "deck" || c.Field == "relic" || c.Field == "potion" {
				parts = append(parts, fmt.Sprintf("Floor %d: %s %s %s", v.Floor, c.Field, c.Type, c.Detail))
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

func (s *Session) DeckNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, len(s.Deck))
	for i, d := range s.Deck {
		names[i] = d.Display()
	}
	return names
}
