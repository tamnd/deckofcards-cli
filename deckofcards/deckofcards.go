// Package deckofcards is the library behind the deckofcards command line:
// the HTTP client, request shaping, and the typed data models for
// deckofcardsapi.com (free, no API key required).
//
// The Client is the spine every command shares. It sets a real User-Agent,
// paces requests so a busy session stays polite, and retries the transient
// failures (5xx) that any public API throws under load.
package deckofcards

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Host is the site this client talks to.
const Host = "deckofcardsapi.com"

// Config holds all tuneable parameters for a Client.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Timeout   time.Duration
	Retries   int
}

// DefaultConfig returns the production configuration for deckofcardsapi.com.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://deckofcardsapi.com/api/deck",
		UserAgent: "deckofcards-cli/0.1.0 (github.com/tamnd/deckofcards-cli)",
		Rate:      200 * time.Millisecond,
		Timeout:   30 * time.Second,
		Retries:   3,
	}
}

// Deck represents a deck created or reshuffled via the API.
type Deck struct {
	ID        string `json:"deck_id"`
	Shuffled  bool   `json:"shuffled"`
	Remaining int    `json:"remaining"`
	Success   bool   `json:"success"`
}

// Card is one card drawn from a deck.
type Card struct {
	Code  string `json:"code"`
	Value string `json:"value"`
	Suit  string `json:"suit"`
	Image string `json:"image"`
}

// DrawResult is the response from a draw operation.
type DrawResult struct {
	DeckID    string `json:"deck_id"`
	Cards     []Card `json:"cards"`
	Remaining int    `json:"remaining"`
	Success   bool   `json:"success"`
}

// Client talks to deckofcardsapi.com over HTTP.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client configured from cfg.
func NewClient(cfg Config) *Client {
	return &Client{cfg: cfg, http: &http.Client{Timeout: cfg.Timeout}}
}

// New creates a new deck. count is the number of 52-card packs to combine
// (defaults to 1 if <= 0). If shuffle is true, uses the /new/shuffle/ endpoint;
// otherwise uses /new/ which creates an unshuffled ordered deck.
func (c *Client) New(ctx context.Context, count int, shuffle bool) (*Deck, error) {
	if count <= 0 {
		count = 1
	}
	var u string
	if shuffle {
		u = fmt.Sprintf("%s/new/shuffle/?deck_count=%d", c.cfg.BaseURL, count)
	} else {
		u = fmt.Sprintf("%s/new/?deck_count=%d", c.cfg.BaseURL, count)
	}
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var d Deck
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, fmt.Errorf("deckofcards: decode deck response: %w", err)
	}
	if !d.Success {
		return nil, fmt.Errorf("deckofcards: API returned success=false")
	}
	return &d, nil
}

// Draw draws n cards from the deck identified by deckID.
// n defaults to 1 if <= 0.
func (c *Client) Draw(ctx context.Context, deckID string, n int) (*DrawResult, error) {
	if n <= 0 {
		n = 1
	}
	u := fmt.Sprintf("%s/%s/draw/?count=%d", c.cfg.BaseURL, deckID, n)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var r DrawResult
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("deckofcards: decode draw response: %w", err)
	}
	if !r.Success {
		return nil, fmt.Errorf("deckofcards: draw API returned success=false")
	}
	return &r, nil
}

// Shuffle reshuffles the existing deck identified by deckID.
// All drawn cards are returned to the deck before reshuffling.
func (c *Client) Shuffle(ctx context.Context, deckID string) (*Deck, error) {
	u := fmt.Sprintf("%s/%s/shuffle/", c.cfg.BaseURL, deckID)
	body, err := c.get(ctx, u)
	if err != nil {
		return nil, err
	}
	var d Deck
	if err := json.Unmarshal(body, &d); err != nil {
		return nil, fmt.Errorf("deckofcards: decode shuffle response: %w", err)
	}
	if !d.Success {
		return nil, fmt.Errorf("deckofcards: shuffle API returned success=false")
	}
	return &d, nil
}

func (c *Client) get(ctx context.Context, u string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, u)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("deckofcards: get %s: %w", u, lastErr)
}

func (c *Client) do(ctx context.Context, u string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.cfg.Rate <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}
