package deckofcards_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/deckofcards-cli/deckofcards"
)

func newTestClient(t *testing.T, handler http.HandlerFunc) *deckofcards.Client {
	t.Helper()
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	cfg := deckofcards.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return deckofcards.NewClient(cfg)
}

// --- new deck tests ---

func TestNew_basic(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"deck_id":"abc123","shuffled":true,"remaining":52}`))
	})
	d, err := c.New(context.Background(), 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if d.ID != "abc123" {
		t.Errorf("ID = %q, want abc123", d.ID)
	}
	if !d.Shuffled {
		t.Error("Shuffled = false, want true")
	}
	if d.Remaining != 52 {
		t.Errorf("Remaining = %d, want 52", d.Remaining)
	}
	if !d.Success {
		t.Error("Success = false, want true")
	}
}

func TestNew_multiDeck(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "deck_count=2") {
			t.Errorf("expected deck_count=2 in query, got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"success":true,"deck_id":"multi1","shuffled":true,"remaining":104}`))
	})
	d, err := c.New(context.Background(), 2, true)
	if err != nil {
		t.Fatal(err)
	}
	if d.Remaining != 104 {
		t.Errorf("Remaining = %d, want 104", d.Remaining)
	}
}

func TestNew_sendsUserAgent(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent header")
		}
		_, _ = w.Write([]byte(`{"success":true,"deck_id":"ua1","shuffled":true,"remaining":52}`))
	})
	_, err := c.New(context.Background(), 1, true)
	if err != nil {
		t.Fatal(err)
	}
}

// --- draw tests ---

func TestDraw_basic(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "abc123") {
			t.Errorf("deck_id not in URL path: %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"success":true,"deck_id":"abc123","cards":[{"code":"AS","value":"ACE","suit":"SPADES","image":"https://example.com/AS.png"},{"code":"KH","value":"KING","suit":"HEARTS","image":"https://example.com/KH.png"}],"remaining":50}`))
	})
	r, err := c.Draw(context.Background(), "abc123", 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Cards) != 2 {
		t.Fatalf("Cards len = %d, want 2", len(r.Cards))
	}
	if r.Remaining != 50 {
		t.Errorf("Remaining = %d, want 50", r.Remaining)
	}
}

func TestDraw_countParam(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.RawQuery, "count=5") {
			t.Errorf("expected count=5 in query, got %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"success":true,"deck_id":"x","cards":[{"code":"2D","value":"2","suit":"DIAMONDS","image":""}],"remaining":47}`))
	})
	_, err := c.Draw(context.Background(), "x", 5)
	if err != nil {
		t.Fatal(err)
	}
}

func TestDraw_parsesCards(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"deck_id":"z","cards":[{"code":"QC","value":"QUEEN","suit":"CLUBS","image":"https://example.com/QC.png"}],"remaining":51}`))
	})
	r, err := c.Draw(context.Background(), "z", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Cards) != 1 {
		t.Fatalf("Cards len = %d, want 1", len(r.Cards))
	}
	card := r.Cards[0]
	if card.Code != "QC" {
		t.Errorf("Code = %q, want QC", card.Code)
	}
	if card.Value != "QUEEN" {
		t.Errorf("Value = %q, want QUEEN", card.Value)
	}
	if card.Suit != "CLUBS" {
		t.Errorf("Suit = %q, want CLUBS", card.Suit)
	}
}

func TestDraw_remaining(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"success":true,"deck_id":"r1","cards":[{"code":"3S","value":"3","suit":"SPADES","image":""}],"remaining":47}`))
	})
	r, err := c.Draw(context.Background(), "r1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if r.Remaining != 47 {
		t.Errorf("Remaining = %d, want 47", r.Remaining)
	}
}

// --- shuffle tests ---

func TestShuffle_basic(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "abc123") {
			t.Errorf("deck_id not in URL path: %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"success":true,"deck_id":"abc123","shuffled":true,"remaining":52}`))
	})
	d, err := c.Shuffle(context.Background(), "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if !d.Shuffled {
		t.Error("Shuffled = false, want true")
	}
	if d.Remaining != 52 {
		t.Errorf("Remaining = %d, want 52", d.Remaining)
	}
}

func TestShuffle_retry503(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"deck_id":"s1","shuffled":true,"remaining":52}`))
	}))
	defer ts.Close()

	cfg := deckofcards.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := deckofcards.NewClient(cfg)

	start := time.Now()
	_, err := c.Shuffle(context.Background(), "s1")
	if err != nil {
		t.Fatal(err)
	}
	if hits != 3 {
		t.Errorf("server hits = %d, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

// --- error tests ---

func TestDraw_http404(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	_, err := c.Draw(context.Background(), "bad_id", 1)
	if err == nil {
		t.Fatal("expected error on 404, got nil")
	}
}

func TestNew_retry500(t *testing.T) {
	var hits int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(`{"success":true,"deck_id":"ok1","shuffled":true,"remaining":52}`))
	}))
	defer ts.Close()

	cfg := deckofcards.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	cfg.Retries = 3
	c := deckofcards.NewClient(cfg)

	_, err := c.New(context.Background(), 1, true)
	if err != nil {
		t.Fatal(err)
	}
	if hits != 2 {
		t.Errorf("server hits = %d, want 2", hits)
	}
}
