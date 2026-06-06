package prompt

import (
	"encoding/json"
	"fmt"

	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

func System(stateType string) string {
	switch stateType {
	case "monster", "elite", "boss":
		return `Slay the Spire 2 coach. Combat only.

Rules:
- Only use cards where can_play is true
- Track energy: subtract each card's cost as you go, stop when energy hits 0
- X-cost cards (Whirlwind, Blade Dance, etc.) spend ALL remaining energy — nothing can be played after them
- Never suggest a card you cannot afford
- Factor in active powers/status on player and enemies (Strength, Weak, Vulnerable, Frail, etc.)
- Factor in available potions if using one changes the outcome (mention it explicitly)

One line response only:
Play: [Card] → [Card] → ... = [damage] dmg, [enemy dies / X HP left]. Watch: [intent + number].
If a potion changes the optimal line, add: Potion: [name] before the play line.`

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
	return fmt.Sprintf("Trigger: %s\n\nGame state:\n%s", trigger.Reason, string(raw))
}
