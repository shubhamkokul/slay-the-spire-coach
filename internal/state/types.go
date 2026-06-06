package state

import "encoding/json"

type GameState struct {
	StateType string      `json:"state_type"`
	Run       RunState    `json:"run"`
	Player    PlayerState `json:"player"`
	Battle    *BattleState `json:"battle,omitempty"`
	// Raw JSON preserved for prompt building
	Raw json.RawMessage `json:"-"`
}

type RunState struct {
	Act       int `json:"act"`
	Floor     int `json:"floor"`
	Ascension int `json:"ascension"`
}

type PlayerState struct {
	Character       string   `json:"character"`
	HP              int      `json:"hp"`
	MaxHP           int      `json:"max_hp"`
	Block           int      `json:"block"`
	Gold            int      `json:"gold"`
	Energy          int      `json:"energy"`
	MaxEnergy       int      `json:"max_energy"`
	Hand            []Card   `json:"hand,omitempty"`
	DrawPileCount   int      `json:"draw_pile_count"`
	DiscardPileCount int     `json:"discard_pile_count"`
	ExhaustPileCount int     `json:"exhaust_pile_count"`
	Status          []Power  `json:"status,omitempty"`
	Relics          []Relic  `json:"relics,omitempty"`
	Potions         []Potion `json:"potions,omitempty"`
}

type BattleState struct {
	Round   int     `json:"round"`
	Turn    string  `json:"turn"`
	Enemies []Enemy `json:"enemies,omitempty"`
}

type Card struct {
	Index           int    `json:"index"`
	ID              string `json:"id"`
	Name            string `json:"name"`
	Type            string `json:"type"`
	Cost            string `json:"cost"`
	Description     string `json:"description"`
	TargetType      string `json:"target_type,omitempty"`
	CanPlay         bool   `json:"can_play"`
	UnplayableReason string `json:"unplayable_reason,omitempty"`
	IsUpgraded      bool   `json:"is_upgraded"`
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

func Detect(prev, curr GameState, currRaw json.RawMessage) *Trigger {
	if prev.StateType != curr.StateType {
		return &Trigger{
			Reason: "entered " + curr.StateType,
			State:  curr,
			Raw:    currRaw,
		}
	}

	if curr.Battle != nil && prev.Battle != nil && curr.Battle.Round != prev.Battle.Round {
		return &Trigger{
			Reason: "new combat round",
			State:  curr,
			Raw:    currRaw,
		}
	}

	if prev.Player.MaxHP > 0 {
		hpDrop := prev.Player.HP - curr.Player.HP
		if hpDrop > prev.Player.MaxHP/5 {
			return &Trigger{
				Reason: "took heavy damage",
				State:  curr,
				Raw:    currRaw,
			}
		}
	}

	if prev.Run.Floor != curr.Run.Floor {
		return &Trigger{
			Reason: "new floor",
			State:  curr,
			Raw:    currRaw,
		}
	}

	return nil
}
