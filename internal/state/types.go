package state

import (
	"encoding/json"
	"fmt"
)

type GameState struct {
	StateType string       `json:"state_type"`
	Run       RunState     `json:"run"`
	Player    PlayerState  `json:"player"`
	Battle    *BattleState `json:"battle,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

type RunState struct {
	Act       int `json:"act"`
	Floor     int `json:"floor"`
	Ascension int `json:"ascension"`
}

type PlayerState struct {
	Character        string   `json:"character"`
	HP               int      `json:"hp"`
	MaxHP            int      `json:"max_hp"`
	Block            int      `json:"block"`
	Gold             int      `json:"gold"`
	Energy           int      `json:"energy"`
	MaxEnergy        int      `json:"max_energy"`
	Hand             []Card   `json:"hand,omitempty"`
	DrawPileCount    int      `json:"draw_pile_count"`
	DiscardPileCount int      `json:"discard_pile_count"`
	ExhaustPileCount int      `json:"exhaust_pile_count"`
	Status           []Power  `json:"status,omitempty"`
	Relics           []Relic  `json:"relics,omitempty"`
	Potions          []Potion `json:"potions,omitempty"`
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
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	CanUse      bool   `json:"can_use"`
}

type Trigger struct {
	Reason string
	State  GameState
	Raw    json.RawMessage
}

// Hash returns a short string identifying the key combat state.
// Used to skip Claude calls when nothing meaningful has changed.
func Hash(gs GameState) string {
	round := 0
	var enemyStates string
	if gs.Battle != nil {
		round = gs.Battle.Round
		for _, e := range gs.Battle.Enemies {
			enemyStates += fmt.Sprintf("%s:%d:%d|", e.EntityID, e.HP, e.Block)
		}
	}
	var handIDs string
	for _, c := range gs.Player.Hand {
		handIDs += c.ID + ","
	}
	return fmt.Sprintf("%s|f%d|r%d|hp%d|blk%d|e%d|hand:%s|enemies:%s",
		gs.StateType,
		gs.Run.Floor,
		round,
		gs.Player.HP,
		gs.Player.Block,
		gs.Player.Energy,
		handIDs,
		enemyStates,
	)
}

func Detect(prev, curr GameState, currRaw json.RawMessage) *Trigger {
	if prev.StateType != curr.StateType {
		return &Trigger{Reason: "entered " + curr.StateType, State: curr, Raw: currRaw}
	}

	if curr.Run.Floor != prev.Run.Floor {
		return &Trigger{Reason: "new floor", State: curr, Raw: currRaw}
	}

	// Heavy damage
	if prev.Player.MaxHP > 0 {
		if drop := prev.Player.HP - curr.Player.HP; drop > prev.Player.MaxHP/5 {
			return &Trigger{Reason: "took heavy damage", State: curr, Raw: currRaw}
		}
	}

	if curr.Battle == nil || prev.Battle == nil {
		return nil
	}

	// Hand just arrived — cards went from empty to populated (covers both first round and new rounds)
	if len(prev.Player.Hand) == 0 && len(curr.Player.Hand) > 0 {
		return &Trigger{Reason: "cards dealt", State: curr, Raw: currRaw}
	}

	// Enemy intent changed mid-round
	if intentKey(curr) != intentKey(prev) {
		return &Trigger{Reason: "enemy intent changed", State: curr, Raw: currRaw}
	}

	return nil
}

func intentKey(gs GameState) string {
	if gs.Battle == nil {
		return ""
	}
	key := ""
	for _, e := range gs.Battle.Enemies {
		for _, i := range e.Intents {
			key += e.EntityID + ":" + i.Type + ":" + i.Label + "|"
		}
	}
	return key
}
