package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shubhamkokul/slay-the-spire-coach/internal/claude"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/client"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

func main() {
	addr := flag.String("addr", "", "STS2MCP address (default http://localhost:15526)")
	model := flag.String("model", "", "Ollama model (default llama3.1:8b)")
	interval := flag.Duration("interval", 2*time.Second, "poll interval")
	cooldown := flag.Duration("cooldown", 6*time.Second, "min time between auto calls")
	flag.Parse()

	sts2 := client.New(*addr)
	ai := claude.New(*model)

	for {
		if err := sts2.Ping(); err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	fmt.Println("Ready. Press Enter anytime for advice.")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Manual trigger: press Enter
	manual := make(chan struct{}, 1)
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			select {
			case manual <- struct{}{}:
			default:
			}
		}
	}()

	var prev state.GameState
	var lastAdvisedHash string
	var lastCall time.Time
	var lastWaiting time.Time
	first := true

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	advise := func(trigger *state.Trigger, force bool) {
		currHash := state.Hash(trigger.State)

		if !force && currHash == lastAdvisedHash {
			if time.Since(lastWaiting) > 15*time.Second {
				fmt.Println("waiting for player action...")
				lastWaiting = time.Now()
			}
			return
		}

		if !force && time.Since(lastCall) < *cooldown {
			return
		}

		lastCall = time.Now()
		lastAdvisedHash = currHash

		if err := ai.Advise(ctx, trigger); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("claude error: %v", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			return

		case <-manual:
			// On-demand: use current game state, bypass cooldown and hash check
			curr, raw, err := sts2.GetState()
			if err != nil {
				log.Printf("poll error: %v", err)
				continue
			}
			trigger := &state.Trigger{Reason: "manual", State: curr, Raw: raw}
			advise(trigger, true)

		case <-ticker.C:
			curr, raw, err := sts2.GetState()
			if err != nil {
				log.Printf("poll error: %v", err)
				continue
			}

			if first {
				prev = curr
				first = false
				continue
			}

			trigger := state.Detect(prev, curr, raw)
			prev = curr

			if trigger == nil {
				continue
			}

			advise(trigger, false)
		}
	}
}
