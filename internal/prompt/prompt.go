package prompt

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

func System(stateType string) string {
	switch stateType {
	case "monster", "elite", "boss":
		return `Slay the Spire 2 coach. Combat.

Step 1 — Potions: check potions[]. If any has can_use:true and helps, use it (0 energy).
Step 2 — Cards: you have exactly [energy] to spend. Only play cards where can_play:true. Track costs strictly. X-cost cards spend ALL remaining energy, nothing after.
Step 3 — Powers: apply Strength (+N damage/attack), Weak (-25% damage dealt), Vulnerable (+50% damage taken).
Card names ending in + are upgraded — use their upgraded values.

Output ONE line, no explanation:
[Potion: name →] Card(cost) → Card(cost) → ... = Xdmg [enemy dies / enemy at X HP]`

	case "card_reward":
		return `Slay the Spire 2 coach. Card pick only.
Take [Card] — [one reason].
Skip the rest.`

	case "rewards":
		return `Slay the Spire 2 coach. One line: Take [item] or Skip all.`

	case "rest_site":
		return `Slay the Spire 2 coach. One line: Rest or Smith [Card]. Why.`

	case "shop":
		return `Slay the Spire 2 coach. Shop. One line.
Best option: Buy [item], Remove [card], or Save gold. Why.`

	case "event":
		return `Slay the Spire 2 coach. One line: Take [option]. Why.`

	case "map":
		return `Slay the Spire 2 coach. Path advice.

If a Boss node is visible within 1-2 floors, add: BOSS WARNING: [what to prepare].
Otherwise: Go [path]. Why. One or two lines max.`

	default:
		return `Slay the Spire 2 coach. One sentence. Specific.`
	}
}

// compactCombat — stripped combat state sent to Claude (~150 tokens vs ~1400)
type compactCombat struct {
	Act           int             `json:"act"`
	Floor         int             `json:"floor"`
	Energy        int             `json:"energy"`
	MaxEnergy     int             `json:"max_energy"`
	HP            int             `json:"hp"`
	MaxHP         int             `json:"max_hp"`
	Block         int             `json:"block"`
	Hand          []compactCard   `json:"hand"`
	Deck          []string        `json:"deck,omitempty"`
	Powers        []compactPower  `json:"powers,omitempty"`
	Relics        []string        `json:"relics"`
	Potions       []compactPotion `json:"potions,omitempty"`
	Orbs          []compactOrb    `json:"orbs,omitempty"`
	OrbSlots      int             `json:"orb_slots,omitempty"`
	OrbEmptySlots int             `json:"orb_empty_slots,omitempty"`
	Enemies       []compactEnemy  `json:"enemies"`
}

type compactOrb struct {
	Name    string `json:"name"`
	Passive int    `json:"passive"`
	Evoke   int    `json:"evoke"`
}

type compactCard struct {
	Name    string `json:"name"`
	Cost    string `json:"cost"`
	CanPlay bool   `json:"can_play"`
}

type compactPower struct {
	Name   string `json:"name"`
	Amount int    `json:"amount"`
}

type compactPotion struct {
	Name   string `json:"name"`
	CanUse bool   `json:"can_use"`
}

type compactEnemy struct {
	Name   string         `json:"name"`
	HP     int            `json:"hp"`
	MaxHP  int            `json:"max_hp"`
	Block  int            `json:"block"`
	Intent string         `json:"intent"`
	Powers []compactPower `json:"powers,omitempty"`
}

func buildCombatCompact(gs state.GameState) string {
	hand := make([]compactCard, len(gs.Player.Hand))
	for i, c := range gs.Player.Hand {
		name := c.Name
		if c.IsUpgraded {
			name += "+"
		}
		hand[i] = compactCard{Name: name, Cost: c.Cost, CanPlay: c.CanPlay}
	}
	powers := make([]compactPower, len(gs.Player.Status))
	for i, p := range gs.Player.Status {
		powers[i] = compactPower{Name: p.Name, Amount: p.Amount}
	}
	relics := make([]string, len(gs.Player.Relics))
	for i, r := range gs.Player.Relics {
		relics[i] = r.Name
	}
	var potions []compactPotion
	for _, p := range gs.Player.Potions {
		if p.CanUseInCombat {
			potions = append(potions, compactPotion{Name: p.Name, CanUse: true})
		}
	}
	var enemies []compactEnemy
	if gs.Battle != nil {
		for _, e := range gs.Battle.Enemies {
			var parts []string
			for _, i := range e.Intents {
				s := i.Title
				if i.Label != "" {
					s += " " + i.Label
				}
				parts = append(parts, s)
			}
			epow := make([]compactPower, len(e.Status))
			for i, p := range e.Status {
				epow[i] = compactPower{Name: p.Name, Amount: p.Amount}
			}
			enemies = append(enemies, compactEnemy{
				Name:   e.Name,
				HP:     e.HP,
				MaxHP:  e.MaxHP,
				Block:  e.Block,
				Intent: strings.Join(parts, ", "),
				Powers: epow,
			})
		}
	}
	orbs := make([]compactOrb, len(gs.Player.Orbs))
	for i, o := range gs.Player.Orbs {
		orbs[i] = compactOrb{Name: o.Name, Passive: o.PassiveVal, Evoke: o.EvokeVal}
	}

	deck := make([]string, 0, len(gs.Player.DrawPile)+len(gs.Player.DiscardPile))
	for _, c := range gs.Player.DrawPile {
		deck = append(deck, c.Name)
	}
	for _, c := range gs.Player.DiscardPile {
		deck = append(deck, c.Name)
	}

	b, _ := json.Marshal(compactCombat{
		Act: gs.Run.Act, Floor: gs.Run.Floor,
		Energy: gs.Player.Energy, MaxEnergy: gs.Player.MaxEnergy,
		HP: gs.Player.HP, MaxHP: gs.Player.MaxHP, Block: gs.Player.Block,
		Hand: hand, Deck: deck, Powers: powers, Relics: relics, Potions: potions,
		Orbs: orbs, OrbSlots: gs.Player.OrbSlots, OrbEmptySlots: gs.Player.OrbEmptySlots,
		Enemies: enemies,
	})
	return string(b)
}

// buildNonCombatCompact strips verbose player fields (relic/card descriptions)
// while keeping all state-specific data (card choices, shop inventory, event options).
func buildNonCombatCompact(gs state.GameState, raw json.RawMessage) string {
	var data map[string]json.RawMessage
	if err := json.Unmarshal(raw, &data); err != nil {
		return string(raw)
	}

	relics := make([]string, len(gs.Player.Relics))
	for i, r := range gs.Player.Relics {
		relics[i] = r.Name
	}
	var potions []string
	for _, p := range gs.Player.Potions {
		potions = append(potions, p.Name)
	}

	p := map[string]any{
		"character": gs.Player.Character,
		"hp":        gs.Player.HP,
		"max_hp":    gs.Player.MaxHP,
		"gold":      gs.Player.Gold,
		"relics":    relics,
		"potions":   potions,
	}

	all := make([]string, 0, len(gs.Player.DrawPile)+len(gs.Player.DiscardPile))
	for _, c := range gs.Player.DrawPile {
		all = append(all, c.Name)
	}
	for _, c := range gs.Player.DiscardPile {
		all = append(all, c.Name)
	}
	if len(all) > 0 {
		p["deck"] = all
	}

	compactP, _ := json.Marshal(p)
	data["player"] = compactP

	out, _ := json.Marshal(data)
	return string(out)
}

func SystemQuestion() string {
	return `Slay the Spire 2 coach. Answer the player's question using their current game state. Be specific and direct. Two sentences max unless the question needs more.`
}

func BuildQuestion(question string, gs state.GameState, raw json.RawMessage) string {
	var gameData string
	if state.IsCombat(gs.StateType) {
		gameData = buildCombatCompact(gs)
	} else {
		gameData = buildNonCombatCompact(gs, raw)
	}
	return fmt.Sprintf("Question: %s\n\nCurrent game state:\n%s", question, gameData)
}

func Build(trigger *state.Trigger, recentEvents string) string {
	var gameData string
	if state.IsCombat(trigger.State.StateType) {
		gameData = buildCombatCompact(trigger.State)
	} else {
		gameData = buildNonCombatCompact(trigger.State, trigger.Raw)
	}
	if recentEvents != "" {
		return fmt.Sprintf("Trigger: %s\n\nRecent run events:\n%s\n\nState:\n%s", trigger.Reason, recentEvents, gameData)
	}
	return fmt.Sprintf("Trigger: %s\n\nState:\n%s", trigger.Reason, gameData)
}
