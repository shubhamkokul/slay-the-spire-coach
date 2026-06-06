package main

import (
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
	interval := flag.Duration("interval", 2*time.Second, "poll interval")
	cooldown := flag.Duration("cooldown", 10*time.Second, "min time between Claude calls")
	flag.Parse()

	sts2 := client.New(*addr)
	ai := claude.New()

	fmt.Println("Waiting for STS2MCP...")
	for {
		if err := sts2.Ping(); err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	fmt.Println("Connected. Watching game state...\n")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var prev state.GameState
	var lastCall time.Time
	first := true

	ticker := time.NewTicker(*interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			fmt.Println("\nStopping.")
			return
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

			if time.Since(lastCall) < *cooldown {
				continue
			}

			lastCall = time.Now()
			if err := ai.Advise(ctx, trigger); err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("claude error: %v", err)
			}
		}
	}
}
