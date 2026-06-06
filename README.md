# slay-the-spire-coach

A real-time AI coach for Slay the Spire 2. It reads live game state from a mod running inside the game and sends it to Claude (Haiku 4.5) for concise, single-line advice — combat lines, card picks, rest choices, shop decisions, and map pathing.

Tracks token usage and cost per session.

---

## How it works

```
STS2 game → STS2MCP mod (HTTP on :15526) → coach (Go) → Claude API → advice printed to terminal
```

The coach polls the mod's REST API to detect game state transitions, compacts the payload to ~150 tokens, and streams a response from Claude. After each call it prints token usage and running session cost. Everything is also written to a daily log at `~/.local/share/sts2-coach/YYYY-MM-DD.log`.

---

## Prerequisites

- Slay the Spire 2 (Steam)
- [STS2MCP mod](https://github.com/Lautha1/STS2MCP) installed and enabled in the game
- Go 1.21+
- An Anthropic API key

---

## The mod

The coach depends on **STS2MCP** — a Slay the Spire 2 mod that exposes game state over a local HTTP server on port `15526`.

**Install:**

1. Subscribe to STS2MCP on the Steam Workshop (or download and place the mod folder into the STS2 mods directory).
2. Launch STS the Spire 2 and enable the mod from the Mods menu before starting a run.
3. Once in-game, the mod starts an HTTP server automatically. You can verify it's running:

```bash
curl http://localhost:15526/api/v1/singleplayer
```

This returns a JSON blob of the current game state. The coach reads this endpoint on every advice request.

---

## Setup

**1. Clone and build:**

```bash
git clone https://github.com/shubhamkokul/slay-the-spire-coach
cd slay-the-spire-coach
go build -o coach ./cmd/coach
```

**2. Add your API key:**

```bash
cp .env.example .env   # or create .env manually
```

Edit `.env`:

```
ANTHROPIC_API_KEY=sk-ant-...
```

The key is loaded automatically at startup via `godotenv`. It never touches your shell environment and is gitignored.

---

## Usage

Start the game, enable the STS2MCP mod, begin a run, then:

```bash
./coach
```

The coach waits until the mod is reachable, then prints `Ready.`

| Input | Action |
|---|---|
| `Enter` (empty line) | Get advice for the current game state |
| `Stored: <text>` + Enter | Save a note as player context (e.g. deck strategy, boss upcoming) |
| `clear` + Enter | Clear saved context |
| `Ctrl+C` | Quit — prints session token + cost summary |

**Example session:**

```
Ready.
  Enter                  → advice
  Stored: <text> + Enter → save context
  clear + Enter          → clear context

[cards dealt]
Strike(1) → Bash(2) = 14dmg [Jaw Worm at 26 HP]
[tokens: +312in +28out | session: $0.0005]

[entered card_reward]
Take Inflame — scales every Strike permanently, better than both alternatives.
[tokens: +198in +19out | session: $0.0007]
```

---

## Token costs

Uses Claude Haiku 4.5 — the cheapest tier with fast latency.

| | Price |
|---|---|
| Input | $1.00 / 1M tokens |
| Output | $5.00 / 1M tokens |

A typical full run (A20, ~50 floors) costs under $0.05.

---

## Custom mod address

If the mod runs on a different port:

```bash
./coach -addr http://localhost:9999
```

---

## Character-specific state

Each STS2 character has unique mechanics that change what information the model needs to give useful advice. The coach handles this per character:

| Character | Extra state sent |
|---|---|
| The Defect | Active orbs (name, passive value, evoke value), orb slots, empty slots |
| Others | Deck composition for card/relic decisions; standard combat state otherwise |

Orb management is central to The Defect — knowing what's slotted, what evokes for how much, and how many empty slots remain changes every combat decision. For characters without orbs the fields are omitted from the payload entirely.

As STS2 adds characters with their own unique mechanics (stances, summons, etc.), add the relevant fields to `internal/state/types.go` and wire them into the appropriate compact builder in `internal/prompt/prompt.go`.

---

## Project structure

```
cmd/coach/          main entrypoint — input loop, context management
internal/claude/    Anthropic client, token tracking, session logging
internal/client/    HTTP client for the STS2MCP mod API
internal/prompt/    Prompt builders — compact state serialization, system prompts per state type
internal/state/     Game state types, trigger detection, state hashing
```
