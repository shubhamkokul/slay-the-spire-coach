package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/shubhamkokul/slay-the-spire-coach/internal/claude"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/client"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

const maxContext = 5

func main() {
	addr := flag.String("addr", "", "STS2MCP address (default http://localhost:15526)")
	interval := flag.Duration("interval", 2*time.Second, "poll interval")
	cooldown := flag.Duration("cooldown", 8*time.Second, "min time between auto calls")
	flag.Parse()

	sts2 := client.New(*addr)
	ai, err := claude.New()
	if err != nil {
		log.Fatal(err)
	}

	for {
		if err := sts2.Ping(); err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	fmt.Println("Ready.")
	fmt.Println("  Enter          → advice now")
	fmt.Println("  type + Enter   → add context")
	fmt.Println("  clear + Enter  → clear context")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	manual := make(chan struct{}, 1)
	contextMsg := make(chan string, 4)

	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				select {
				case manual <- struct{}{}:
				default:
				}
			} else {
				select {
				case contextMsg <- line:
				default:
				}
			}
		}
	}()

	var prev state.GameState
	var lastAdvisedHash string
	var lastCall time.Time
	var lastWaiting time.Time
	var userContext []string
	first := true

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	withContext := func(t *state.Trigger) *state.Trigger {
		t.Context = append([]string(nil), userContext...)
		return t
	}

	advise := func(trigger *state.Trigger, force bool) {
		currHash := state.Hash(trigger.State)

		if !force {
			if currHash == lastAdvisedHash {
				if state.IsCombat(trigger.State.StateType) && time.Since(lastWaiting) > 20*time.Second {
					fmt.Println("thinking...")
					lastWaiting = time.Now()
				}
				return
			}
			if trigger.Reason != "cards dealt" && time.Since(lastCall) < *cooldown {
				return
			}
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

		case line := <-contextMsg:
			if strings.ToLower(line) == "clear" {
				userContext = nil
				fmt.Println("context cleared")
			} else {
				if len(userContext) >= maxContext {
					userContext = userContext[1:]
				}
				userContext = append(userContext, line)
				fmt.Printf("context saved (%d/%d)\n", len(userContext), maxContext)
			}

		case <-manual:
			curr, raw, err := sts2.GetState()
			if err != nil {
				log.Printf("poll error: %v", err)
				continue
			}
			trigger := withContext(&state.Trigger{Reason: "manual", State: curr, Raw: raw})
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

			advise(withContext(trigger), false)
		}
	}
}
