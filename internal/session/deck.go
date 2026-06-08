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

// isTemporary returns true for Status-type cards added during combat.
// These are purged when combat ends and must not trigger upgrade detection.
//
// Primary signal: card type from API ("Status"). This covers all Status cards
// regardless of name and correctly excludes permanent cards added by powers
// or card effects (Shiv, Bite, Miracle, etc.) which have Attack/Skill type.
//
// Fallback name list: draw/discard pile entries lack type in the API response.
// Only pure Status cards go here — NOT cards that can be permanent additions.
func isTemporary(name, cardType string) bool {
	if cardType == "Status" {
		return true
	}
	// Fallback for pile cards where type is absent.
	// Only add names that are ALWAYS temporary — never permanent acquisitions.
	switch name {
	case "Slimed", "Wound", "Dazed", "Burn", "Void":
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
