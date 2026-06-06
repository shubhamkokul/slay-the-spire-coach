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

	fmt.Println("Ready. Press Enter for advice.")

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

		curr, raw, err := sts2.GetState()
		if err != nil {
			log.Printf("error: %v", err)
			continue
		}
		trigger := &state.Trigger{
			Reason: curr.StateType,
			State:  curr,
			Raw:    raw,
		}
		if err := ai.Advise(ctx, trigger); err != nil {
			if ctx.Err() != nil {
				return
			}
			log.Printf("claude error: %v", err)
		}
	}
}
