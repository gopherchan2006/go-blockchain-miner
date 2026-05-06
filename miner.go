package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
)

func computeHash(t Template, nonce int) string {
	raw := fmt.Sprintf("%d%d%s%s%d", t.Index, t.Timestamp, t.TxData, t.PrevHash, nonce)
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}

func isValid(hash string, difficulty int) bool {
	return strings.HasPrefix(hash, strings.Repeat("0", difficulty))
}

func Mine(ctx context.Context, template Template, workers int) (nonce int, hash string, ok bool) {
	type result struct {
		nonce int
		hash  string
	}

	found := make(chan result, 1)
	mineCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i := 0; i < workers; i++ {
		start := i * (math.MaxInt / workers)
		go func(startNonce int) {
			for n := startNonce; ; n++ {
				select {
				case <-mineCtx.Done():
					return
				default:
				}
				h := computeHash(template, n)
				if isValid(h, template.Difficulty) {
					select {
					case found <- result{n, h}:
						cancel()
					default:
					}
					return
				}
			}
		}(start)
	}

	select {
	case r := <-found:
		return r.nonce, r.hash, true
	case <-ctx.Done():
		return 0, "", false
	}
}
