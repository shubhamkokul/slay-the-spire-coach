package state

import (
	"encoding/json"
	"fmt"
	"strings"
)

type GameState struct {
	StateType  string           `json:"state_type"`
	Run        RunState         `json:"run"`
	Player     PlayerState      `json:"player"`
	Battle     *BattleState     `json:"battle,omitempty"`
	CardSelect *CardSelectState `json:"card_select,omitempty"`
	CardReward *CardRewardState `json:"card_reward,omitempty"`
	Raw        json.RawMessage  `json:"-"`
}

type CardSelectState struct {
	ScreenType string `json:"screen_type"` // "upgrade", "transform", "remove", "exhaust"
	Prompt     string `json:"prompt"`
	Cards      []Card `json:"cards"`
}

type CardRewardState struct {
	Cards []Card `json:"cards"`
}

type RunState struct {
	Act       int `json:"act"`
	Floor     int `json:"floor"`
	Ascension int `json:"ascension"`
}

type PlayerState struct {
	Character        string     `json:"character"`
	HP               int        `json:"hp"`
	MaxHP            int        `json:"max_hp"`
	Block            int        `json:"block"`
	Gold             int        `json:"gold"`
	Energy           int        `json:"energy"`
	MaxEnergy        int        `json:"max_energy"`
	Hand             []Card     `json:"hand,omitempty"`
	DrawPileCount    int        `json:"draw_pile_count"`
	DiscardPileCount int        `json:"discard_pile_count"`
	ExhaustPileCount int        `json:"exhaust_pile_count"`
	DrawPile         []DeckCard `json:"draw_pile,omitempty"`
	DiscardPile      []DeckCard `json:"discard_pile,omitempty"`
	Orbs             []Orb      `json:"orbs,omitempty"`
	OrbSlots         int        `json:"orb_slots"`
	OrbEmptySlots    int        `json:"orb_empty_slots"`
	Status           []Power    `json:"status,omitempty"`
	Relics           []Relic    `json:"relics,omitempty"`
	Potions          []Potion   `json:"potions,omitempty"`
}

type DeckCard struct {
	Name string `json:"name"`
	Cost string `json:"cost"`
	Type string `json:"type,omitempty"` // "Attack", "Skill", "Power", "Status", "Curse"
}

type Orb struct {
	Name       string `json:"name"`
	PassiveVal int    `json:"passive_val"`
	EvokeVal   int    `json:"evoke_val"`
}

type BattleState struct {
	Round   int     `json:"round"`
	Turn    string  `json:"turn"`
	Enemies []Enemy `json:"enemies,omitempty"`
}

type Card struct {
	Index            int    `json:"index"`
	ID               string `json:"id"`
	Name             string `json:"name"`
	Type             string `json:"type"`
	Cost             string `json:"cost"`
	Description      string `json:"description"`
	TargetType       string `json:"target_type,omitempty"`
	CanPlay          bool   `json:"can_play"`
	UnplayableReason string `json:"unplayable_reason,omitempty"`
	IsUpgraded       bool   `json:"is_upgraded"`
}

type Enemy struct {
	EntityID string   `json:"entity_id"`
	Name     string   `json:"name"`
	HP       int      `json:"hp"`
	MaxHP    int      `json:"max_hp"`
	Block    int      `json:"block"`
	Status   []Power  `json:"status,omitempty"`
	Intents  []Intent `json:"intents,omitempty"`
}

type Intent struct {
	Type        string `json:"type"`
	Label       string `json:"label"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description"`
}

type Power struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Amount      int    `json:"amount"`
	Description string `json:"description,omitempty"`
}

type Relic struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type Potion struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	Slot           int    `json:"slot"`
	CanUseInCombat bool   `json:"can_use_in_combat"`
	TargetType     string `json:"target_type,omitempty"`
}

type Trigger struct {
	Reason string
	State  GameState
	Raw    json.RawMessage
}

var advisable = map[string]struct{}{
	"monster":     {},
	"elite":       {},
	"boss":        {},
	"card_reward": {},
	"rewards":     {},
	"rest_site":   {},
	"shop":        {},
	"event":       {},
	"map":         {},
}

func IsCombat(s string) bool {
	return s == "monster" || s == "elite" || s == "boss"
}

func Hash(gs GameState) string {
	round := 0
	var enemies strings.Builder
	if gs.Battle != nil {
		round = gs.Battle.Round
		for _, e := range gs.Battle.Enemies {
			fmt.Fprintf(&enemies, "%s:%d:%d|", e.EntityID, e.HP, e.Block)
		}
	}
	var hand strings.Builder
	for _, c := range gs.Player.Hand {
		hand.WriteString(c.ID)
		hand.WriteByte(',')
	}
	return fmt.Sprintf("%s|f%d|r%d|hp%d|blk%d|e%d|hand:%s|enemies:%s",
		gs.StateType,
		gs.Run.Floor,
		round,
		gs.Player.HP,
		gs.Player.Block,
		gs.Player.Energy,
		hand.String(),
		enemies.String(),
	)
}

func Detect(prev, curr GameState, currRaw json.RawMessage) *Trigger {
	// State type changed
	if prev.StateType != curr.StateType {
		// Skip states we have no advice for
		if _, ok := advisable[curr.StateType]; !ok {
			return nil
		}
		// For combat: if cards already in hand on first poll, fire immediately.
		// Otherwise return nil and let the cards dealt trigger below handle it.
		if IsCombat(curr.StateType) {
			if len(curr.Player.Hand) > 0 {
				return &Trigger{Reason: "cards dealt", State: curr, Raw: currRaw}
			}
			return nil
		}
		return &Trigger{Reason: "entered " + curr.StateType, State: curr, Raw: currRaw}
	}

	// Skip everything below for non-advisable states
	if _, ok := advisable[curr.StateType]; !ok {
		return nil
	}

	if curr.Run.Floor != prev.Run.Floor {
		return &Trigger{Reason: "new floor", State: curr, Raw: currRaw}
	}

	// Heavy damage — combat only, 20% max HP threshold
	if IsCombat(curr.StateType) && prev.Player.MaxHP > 0 {
		if drop := prev.Player.HP - curr.Player.HP; drop > prev.Player.MaxHP/5 {
			return &Trigger{Reason: "took heavy damage", State: curr, Raw: currRaw}
		}
	}

	if curr.Battle == nil || prev.Battle == nil {
		return nil
	}

	// Cards just dealt — hand went from empty to populated (start of every player turn)
	if len(prev.Player.Hand) == 0 && len(curr.Player.Hand) > 0 {
		return &Trigger{Reason: "cards dealt", State: curr, Raw: currRaw}
	}

	return nil
}
