package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	cfg := loadConfig("config.yaml")
	client := NewNodeClient(cfg.Node.URL)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	events := client.SubscribeEvents(ctx)

	fmt.Printf("miner started  address=%s workers=%d node=%s\n",
		cfg.Miner.Address, cfg.Miner.Workers, cfg.Node.URL)

	for {
		if ctx.Err() != nil {
			fmt.Println("shutting down")
			return
		}

		template, err := client.FetchTemplate(cfg.Miner.Address)
		if err != nil {
			fmt.Fprintf(os.Stderr, "fetch template: %v\n", err)
			select {
			case <-ctx.Done():
				fmt.Println("shutting down")
				return
			case <-time.After(2 * time.Second):
				continue
			}
		}

		fmt.Printf("mining block index=%d difficulty=%d\n", template.Index, template.Difficulty)
		start := time.Now()

		mineCtx, cancelMine := context.WithCancel(ctx)

		doneMine := make(chan struct{})
		var foundNonce int
		var foundHash string
		var foundOk bool

		go func() {
			defer close(doneMine)
			foundNonce, foundHash, foundOk = Mine(mineCtx, template, cfg.Miner.Workers)
		}()

		select {
		case event := <-events:
			cancelMine()
			<-doneMine
			fmt.Printf("interrupted by event=%s, fetching new template\n", event)
			continue
		case <-doneMine:
			cancelMine()
		case <-ctx.Done():
			cancelMine()
			<-doneMine
			fmt.Println("shutting down")
			return
		}

		if !foundOk {
			fmt.Println("shutting down")
			return
		}

		elapsed := time.Since(start)
		fmt.Printf("block mined  index=%d nonce=%d hash=%s time=%s\n",
			template.Index, foundNonce, foundHash, elapsed)

		if err := client.SubmitBlock(foundNonce, foundHash); err != nil {
			fmt.Fprintf(os.Stderr, "submit block: %v\n", err)
		}

		select {
		case event := <-events:
			fmt.Printf("received event=%s, fetching new template\n", event)
		case <-ctx.Done():
			fmt.Println("shutting down")
			return
		}
	}
}
