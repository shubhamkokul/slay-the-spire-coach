package session

// startingDecks holds the known starting deck for each character.
// Defect verified live: 4x Strike, 4x Defend, 1x Zap, 1x Dualcast.
// Regent and Necrobinder starting decks still need live verification.
var startingDecks = map[string][]DeckEntry{
	"The Ironclad": repeat("Strike", 5, "start", 0,
		repeat("Defend", 4, "start", 0,
			[]DeckEntry{{Name: "Bash", Source: "start"}})),

	"The Silent": repeat("Strike", 5, "start", 0,
		repeat("Defend", 5, "start", 0,
			[]DeckEntry{
				{Name: "Neutralize", Source: "start"},
				{Name: "Survivor", Source: "start"},
			})),

	"The Defect": repeat("Strike", 4, "start", 0,
		repeat("Defend", 4, "start", 0,
			[]DeckEntry{
				{Name: "Zap", Source: "start"},
				{Name: "Dualcast", Source: "start"},
			})),

	"The Regent": repeat("Strike", 4, "start", 0,
		repeat("Defend", 4, "start", 0,
			[]DeckEntry{
				{Name: "Falling Star", Source: "start"},
				{Name: "Venerate", Source: "start"},
			})),

	"The Necrobinder": repeat("Strike", 4, "start", 0,
		repeat("Defend", 4, "start", 0,
			[]DeckEntry{
				{Name: "Bodyguard", Source: "start"},
				{Name: "Unleash", Source: "start"},
			})),
}

func startingDeck(character string) []DeckEntry {
	template, ok := startingDecks[character]
	if !ok {
		return nil
	}
	deck := make([]DeckEntry, len(template))
	copy(deck, template)
	return deck
}

// isTemporary returns true for cards that are added during combat and should
// be purged when combat ends. Uses card type from API when available,
// falls back to a known name list for draw/discard pile cards which lack type.
func isTemporary(name, cardType string) bool {
	if cardType == "Status" {
		return true
	}
	// Known status card names — expanded as new ones are discovered in STS2.
	switch name {
	case "Slimed", "Wound", "Dazed", "Burn", "Void", "Parasite",
		"Pride", "Normality", "Decay", "Regret", "Shame", "Doubt",
		"Injury", "Pain", "Clumsy", "Depression", "Curse of the Bell",
		"Writhe", "Necronomicurse", "Bite", "Shiv":
		// Note: Bite and Shiv can be permanent (via relics/cards) — reconcile
		// will only mark them temporary if they appear unexpectedly mid-combat.
		// For now include them; revisit if false positives appear.
		return true
	}
	return false
}

func repeat(name string, n int, source string, floor int, rest []DeckEntry) []DeckEntry {
	entries := make([]DeckEntry, n, n+len(rest))
	for i := range entries {
		entries[i] = DeckEntry{Name: name, Source: source, Floor: floor}
	}
	return append(entries, rest...)
}
