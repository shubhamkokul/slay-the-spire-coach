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

func repeat(name string, n int, source string, floor int, rest []DeckEntry) []DeckEntry {
	entries := make([]DeckEntry, n, n+len(rest))
	for i := range entries {
		entries[i] = DeckEntry{Name: name, Source: source, Floor: floor}
	}
	return append(entries, rest...)
}
