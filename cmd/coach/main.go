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
	"github.com/shubhamkokul/slay-the-spire-coach/internal/client"
	"github.com/shubhamkokul/slay-the-spire-coach/internal/session"
)

func main() {
	_ = godotenv.Load()

	addr := flag.String("addr", "", "STS2MCP address (default http://localhost:15526)")
	flag.Parse()

	store := session.NewMemoryStore()
	sts2 := client.New(*addr, store)

	// Wait for mod to be reachable.
	for {
		if err := sts2.Ping(); err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Background poller — keeps session and store current without any user input.
	sts2.Poll(ctx)

	fmt.Println("Ready. Session updating in background every 2s.")
	fmt.Println("  Enter     → full status snapshot")
	fmt.Println("  deck      → deck only")
	fmt.Println("  history   → version log of every change this run")
	fmt.Println("  new       → reset session")
	fmt.Println("  Ctrl+C    → quit")

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
			fmt.Println("Session reset — will reinitialize on next poll.")
			continue
		}

		if sts2.Session == nil {
			fmt.Println("No active session yet — waiting for game state...")
			continue
		}

		switch line {
		case "":
			fmt.Print(sts2.Session.PrintStatus())
		case "deck":
			fmt.Print(sts2.Session.PrintDeck())
		case "history":
			fmt.Print(sts2.Session.PrintHistory())
		case "debug":
			fmt.Printf("version: %d\n", sts2.Session.Version)
			fmt.Printf("relics(%d): %+v\n", len(sts2.Session.Relics), sts2.Session.Relics)
			fmt.Printf("potions(%d): %+v\n", len(sts2.Session.Potions), sts2.Session.Potions)
			fmt.Printf("deck(%d)\n", len(sts2.Session.Deck))
		default:
			fmt.Println("Commands: Enter (status), deck, history, debug, new")
		}
	}
}
