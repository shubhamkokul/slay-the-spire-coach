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
- player.energy is your current energy this turn — you cannot spend more than this
- Only use cards where can_play is true
- Track energy: start at player.energy, subtract each card's cost. Stop when energy hits 0
- X-cost cards (Whirlwind, Blade Dance, etc.) spend ALL remaining energy — nothing playable after them
- Never suggest a card that costs more than your remaining energy
- Factor in player.status powers (Strength, Weak, Vulnerable, Frail) in damage calculations
- Factor in available potions if using one changes the outcome

One line response only:
Play: [Card(cost)] → [Card(cost)] → ... energy used: X/Y = [damage] dmg, [enemy dies / X HP left]. Watch: [intent + number].
If a potion changes the optimal line, prepend: Potion: [name].`

	case "card_reward":
		return `Slay the Spire 2 coach. Card pick advice only.

Consider the player's current relics, potions, and existing deck when evaluating synergy.

Take [Card] — [one reason referencing deck/relic synergy].
Skip the rest.`

	case "rewards":
		return `Slay the Spire 2 coach. Reward advice only.

Take [item]. Skip [item]. One sentence max.`

	case "rest_site":
		return `Slay the Spire 2 coach. Rest site advice only.

Rest / Smith [Card]. One sentence reason.`

	case "shop":
		return `Slay the Spire 2 coach. Shop advice only.

Buy [item]. Skip [item]. Save gold if nothing fits.`

	case "event":
		return `Slay the Spire 2 coach. Event advice only.

Take [option]. One sentence reason.`

	case "map":
		return `Slay the Spire 2 coach. Path advice only.

Go [node types in order]. One sentence reason.`

	default:
		return `Slay the Spire 2 coach. One to two sentences. Specific and direct.`
	}
}

func Build(trigger *state.Trigger) string {
	raw, _ := json.MarshalIndent(json.RawMessage(trigger.Raw), "", "  ")

	header := fmt.Sprintf("Trigger: %s", trigger.Reason)

	if state.IsCombat(trigger.State.StateType) {
		header += fmt.Sprintf("\nEnergy: %d/%d",
			trigger.State.Player.Energy,
			trigger.State.Player.MaxEnergy,
		)
	}

	if len(trigger.Context) > 0 {
		ctx := strings.Join(trigger.Context, "\n- ")
		return fmt.Sprintf("%s\n\nPlayer context:\n- %s\n\nGame state:\n%s", header, ctx, string(raw))
	}
	return fmt.Sprintf("%s\n\nGame state:\n%s", header, string(raw))
}
