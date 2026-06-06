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
		return `Slay the Spire 2 coach. Combat only.

Rules:
- energy is your current energy — cannot spend more than this
- Only play cards where can_play is true
- Track energy: subtract each card cost, stop at 0
- X-cost cards (Whirlwind, etc.) spend ALL remaining energy — nothing after them
- Factor in player powers (Strength adds damage, Weak reduces by 25%, Vulnerable takes 50% more)
- If a potion (can_use: true) changes the outcome, recommend it — potions cost 0 energy

One line only:
Play: [Card] → [Card] → ... = [damage] dmg, [dies / X HP left]. Watch: [intent + number].
Prepend Potion: [name] if needed.`

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
	Act       int              `json:"act"`
	Floor     int              `json:"floor"`
	Energy    int              `json:"energy"`
	MaxEnergy int              `json:"max_energy"`
	HP        int              `json:"hp"`
	MaxHP     int              `json:"max_hp"`
	Block     int              `json:"block"`
	Hand      []compactCard    `json:"hand"`
	Powers    []compactPower   `json:"powers,omitempty"`
	Relics    []string         `json:"relics"`
	Potions   []compactPotion  `json:"potions,omitempty"`
	Enemies   []compactEnemy   `json:"enemies"`
}

type compactCard struct {
	Name     string `json:"name"`
	Cost     string `json:"cost"`
	CanPlay  bool   `json:"can_play"`
	Upgraded bool   `json:"upgraded,omitempty"`
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
		hand[i] = compactCard{Name: c.Name, Cost: c.Cost, CanPlay: c.CanPlay, Upgraded: c.IsUpgraded}
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
	b, _ := json.Marshal(compactCombat{
		Act: gs.Run.Act, Floor: gs.Run.Floor,
		Energy: gs.Player.Energy, MaxEnergy: gs.Player.MaxEnergy,
		HP: gs.Player.HP, MaxHP: gs.Player.MaxHP, Block: gs.Player.Block,
		Hand: hand, Powers: powers, Relics: relics, Potions: potions, Enemies: enemies,
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

	compactP, _ := json.Marshal(map[string]interface{}{
		"character": gs.Player.Character,
		"hp":        gs.Player.HP,
		"max_hp":    gs.Player.MaxHP,
		"gold":      gs.Player.Gold,
		"relics":    relics,
		"potions":   potions,
	})
	data["player"] = compactP

	out, _ := json.Marshal(data)
	return string(out)
}

func Build(trigger *state.Trigger) string {
	header := fmt.Sprintf("Trigger: %s", trigger.Reason)

	var gameData string
	if state.IsCombat(trigger.State.StateType) {
		gameData = buildCombatCompact(trigger.State)
	} else {
		gameData = buildNonCombatCompact(trigger.State, trigger.Raw)
	}

	if len(trigger.Context) > 0 {
		ctx := strings.Join(trigger.Context, "\n- ")
		return fmt.Sprintf("%s\n\nPlayer context:\n- %s\n\nState:\n%s", header, ctx, gameData)
	}
	return fmt.Sprintf("%s\n\nState:\n%s", header, gameData)
}
