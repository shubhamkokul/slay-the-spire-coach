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

	"github.com/joho/godotenv"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/claude"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/client"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/session"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

func main() {
	_ = godotenv.Load()

	addr := flag.String("addr", "", "STS2MCP address (default http://localhost:15526)")
	flag.Parse()

	store := session.NewMemoryStore()
	sts2 := client.New(*addr, store)

	ai, err := claude.New()
	if err != nil {
		log.Fatal(err)
	}

	defer func() {
		fmt.Println("\n" + ai.SessionSummary())
		ai.Close()
	}()

	for {
		if err := sts2.Ping(); err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	sts2.Poll(ctx)

	fmt.Println("Ready. Session updating every 2s.")
	fmt.Println("  Enter       → advice")
	fmt.Println("  status      → full session snapshot")
	fmt.Println("  deck        → deck only")
	fmt.Println("  history     → version log")
	fmt.Println("  new         → reset session")
	fmt.Println("  Type text   → ask a question")
	fmt.Println("  Ctrl+C      → quit")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				log.Printf("input error: %v", err)
			}
			return
		}

		line := strings.TrimSpace(scanner.Text())

		if line == "new" {
			sts2.ResetSession()
			fmt.Println("Session reset.")
			continue
		}

		curr, raw, err := sts2.GetState()
		if err != nil {
			log.Printf("error: %v", err)
			continue
		}

		sess := sts2.Session
		if sess == nil {
			fmt.Println("No active session — start a run first.")
			continue
		}

		switch line {
		case "":
			trigger := &state.Trigger{
				Reason: curr.StateType,
				State:  curr,
				Raw:    raw,
			}
			if err := ai.Advise(ctx, trigger, sess.RecentEvents(5)); err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("claude error: %v", err)
			}

		case "status":
			fmt.Print(sess.PrintStatus())

		case "deck":
			fmt.Print(sess.PrintDeck())

		case "history":
			fmt.Print(sess.PrintHistory())

		case "debug":
			fmt.Printf("version: %d\n", sess.Version)
			fmt.Printf("relics(%d): %+v\n", len(sess.Relics), sess.Relics)
			fmt.Printf("potions(%d): %+v\n", len(sess.Potions), sess.Potions)
			fmt.Printf("deck(%d)\n", len(sess.Deck))

		default:
			if err := ai.Ask(ctx, line, curr, raw); err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("claude error: %v", err)
			}
		}
	}
}
