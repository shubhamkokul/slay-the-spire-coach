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

	for {
		if err := sts2.Ping(); err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	fmt.Println("Ready.")
	fmt.Println("  deck  → show current deck")
	fmt.Println("  Ctrl+C → quit")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

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

		// Refresh state on every command.
		if _, _, err := sts2.GetState(); err != nil {
			log.Printf("error: %v", err)
			continue
		}

		switch line {
		case "deck":
			if sts2.Session == nil {
				fmt.Println("No active session — start a run first.")
				continue
			}
			fmt.Println(sts2.Session.PrintDeck())
		default:
			fmt.Println("Commands: deck")
		}
	}
}
