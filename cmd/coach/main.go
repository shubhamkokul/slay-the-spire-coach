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
	"github.com/shubhamkokul/slay-the-spire-coach/internal/state"
)

func main() {
	_ = godotenv.Load()

	addr := flag.String("addr", "", "STS2MCP address (default http://localhost:15526)")
	flag.Parse()

	sts2 := client.New(*addr)
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

	fmt.Println("Ready.")
	fmt.Println("  Enter          → advice for current state")
	fmt.Println("  Type + Enter   → ask a question")

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

		curr, raw, err := sts2.GetState()
		if err != nil {
			log.Printf("error: %v", err)
			continue
		}

		var callErr error
		if line == "" {
			trigger := &state.Trigger{
				Reason: curr.StateType,
				State:  curr,
				Raw:    raw,
			}
			callErr = ai.Advise(ctx, trigger)
		} else {
			callErr = ai.Ask(ctx, line, curr, raw)
		}

		if callErr != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("claude error: %v", callErr)
		}
	}
}
