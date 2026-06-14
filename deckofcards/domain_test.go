package deckofcards_test

import (
	"testing"

	"github.com/tamnd/deckofcards-cli/deckofcards"
)

// These tests are offline: they exercise the URI driver's pure string functions.
// HTTP behaviour is covered in deckofcards_test.go.

func TestDomainInfo(t *testing.T) {
	info := deckofcards.Domain{}.Info()
	if info.Scheme != "deckofcards" {
		t.Errorf("Scheme = %q, want deckofcards", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != deckofcards.Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, deckofcards.Host)
	}
	if info.Identity.Binary != "deckofcards" {
		t.Errorf("Identity.Binary = %q, want deckofcards", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ, id string }{
		{"abc123", "deck", "abc123"},
		{"sezfekm5xy24", "deck", "sezfekm5xy24"},
	}
	for _, tc := range cases {
		typ, id, err := deckofcards.Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestClassifyEmpty(t *testing.T) {
	_, _, err := deckofcards.Domain{}.Classify("")
	if err == nil {
		t.Error("expected error for empty input, got nil")
	}
}

func TestLocate(t *testing.T) {
	got, err := deckofcards.Domain{}.Locate("deck", "abc123")
	if err != nil {
		t.Fatalf("Locate: %v", err)
	}
	if got == "" {
		t.Error("Locate returned empty URL")
	}
}

func TestLocateUnknownType(t *testing.T) {
	_, err := deckofcards.Domain{}.Locate("unknown", "foo")
	if err == nil {
		t.Error("expected error for unknown type, got nil")
	}
}
