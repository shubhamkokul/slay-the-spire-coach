package prompt

import (
	"encoding/json"
	"fmt"

	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

const System = `You are a real-time Slay the Spire 2 coach. You give sharp, actionable advice.

Always respond in this exact format:
NOW: What to do right now (1-2 sentences)
NEXT: What to plan or prepare for (1-2 sentences)
WATCH: Key threat or risk to be aware of (1 sentence)

Be specific. Reference card names, relic names, enemy names. No filler.`

func Build(trigger *state.Trigger) string {
	raw, _ := json.MarshalIndent(json.RawMessage(trigger.Raw), "", "  ")
	return fmt.Sprintf("Trigger: %s\n\nGame state:\n%s", trigger.Reason, string(raw))
}
