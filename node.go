package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Template struct {
	Index      int    `json:"index"`
	Timestamp  int64  `json:"timestamp"`
	TxData     string `json:"txData"`
	TxIDs      string `json:"txIDs"`
	PrevHash   string `json:"prevHash"`
	Difficulty int    `json:"difficulty"`
}

type NodeClient struct {
	baseURL string
	http    *http.Client
}

func NewNodeClient(baseURL string) *NodeClient {
	return &NodeClient{baseURL: baseURL, http: &http.Client{}}
}

func (c *NodeClient) FetchTemplate(address string) (Template, error) {
	miner := url.QueryEscape(address)
	resp, err := c.http.Get(fmt.Sprintf("%s/api/blocktemplate?miner=%s", c.baseURL, miner))
	if err != nil {
		return Template{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Template{}, fmt.Errorf("blocktemplate: unexpected status %d", resp.StatusCode)
	}

	var t Template
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil {
		return Template{}, err
	}
	return t, nil
}

func (c *NodeClient) SubmitBlock(nonce int, hash string) error {
	body, _ := json.Marshal(map[string]interface{}{"nonce": nonce, "hash": hash})
	resp, err := c.http.Post(fmt.Sprintf("%s/api/block", c.baseURL), "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("submit block: unexpected status %d", resp.StatusCode)
	}
	return nil
}

func (c *NodeClient) SubscribeEvents(ctx context.Context) <-chan string {
	ch := make(chan string, 4)
	go func() {
		defer close(ch)
		for {
			if err := c.streamEvents(ctx, ch); err != nil {
				if ctx.Err() != nil {
					return
				}
			}
			if ctx.Err() != nil {
				return
			}
		}
	}()
	return ch
}

func (c *NodeClient) streamEvents(ctx context.Context, ch chan<- string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/api/events", c.baseURL), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "event:") {
			event := strings.TrimSpace(strings.TrimPrefix(line, "event:"))
			if !shouldForwardEvent(event) {
				continue
			}
			select {
			case ch <- event:
			case <-ctx.Done():
				return nil
			}
		}
	}
	return scanner.Err()
}

func shouldForwardEvent(event string) bool {
	switch event {
	case "new_block", "block_mined", "new_tx":
		return true
	default:
		return false
	}
}
