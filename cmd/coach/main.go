package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
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

const maxContext = 5

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
	fmt.Println("  Enter                  → advice")
	fmt.Println("  Stored: <text> + Enter → save context")
	fmt.Println("  clear + Enter          → clear context")
	fmt.Println("  dump + Enter           → write raw state to /tmp/sts2-state.json")

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	var userContext []string

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

		switch {
		case line == "dump":
			_, raw, err := sts2.GetState()
			if err != nil {
				log.Printf("error: %v", err)
				continue
			}
			var pretty bytes.Buffer
			if err := json.Indent(&pretty, raw, "", "  "); err != nil {
				log.Printf("indent error: %v", err)
				continue
			}
			const dumpPath = "/tmp/sts2-state.json"
			if err := os.WriteFile(dumpPath, pretty.Bytes(), 0644); err != nil {
				log.Printf("write error: %v", err)
				continue
			}
			fmt.Printf("dumped → %s\n", dumpPath)

		case line == "clear":
			userContext = nil
			fmt.Println("context cleared")

		case strings.HasPrefix(line, "Stored:"):
			text := strings.TrimSpace(strings.TrimPrefix(line, "Stored:"))
			if text != "" {
				if len(userContext) >= maxContext {
					userContext = userContext[1:]
				}
				userContext = append(userContext, text)
				fmt.Printf("stored (%d/%d)\n", len(userContext), maxContext)
			}

		case line == "":
			curr, raw, err := sts2.GetState()
			if err != nil {
				log.Printf("error: %v", err)
				continue
			}
			trigger := &state.Trigger{
				Reason:  curr.StateType,
				State:   curr,
				Raw:     raw,
				Context: append([]string(nil), userContext...),
			}
			if err := ai.Advise(ctx, trigger); err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("claude error: %v", err)
			}
		}
	}
}
