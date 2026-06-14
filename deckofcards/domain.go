package deckofcards

import (
	"context"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes deckofcards as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/deckofcards-cli/deckofcards"
//
// The init below registers it; the host then dereferences deckofcards:// URIs
// by routing to the operations Register installs. The same Domain also builds
// the standalone deckofcards binary (see cli.NewApp).
func init() { kit.Register(Domain{}) }

// Domain is the deckofcards driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against,
// and the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "deckofcards",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "deckofcards",
			Short:  "Interact with the Deck of Cards API.",
			Long: `deckofcards creates and draws from virtual card decks via the
free deckofcardsapi.com API. No authentication required.

Each deck is identified by a deck_id that persists server-side, enabling
multi-step card games across separate commands.`,
			Site: Host,
			Repo: "https://github.com/tamnd/deckofcards-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:    "new",
		Group:   "write",
		Single:  true,
		Summary: "Create a new deck",
	}, newOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "draw",
		Group:   "read",
		Single:  true,
		Summary: "Draw cards from a deck",
		Args:    []kit.Arg{{Name: "deck_id", Help: "deck ID from `deckofcards new`"}},
	}, drawOp)

	kit.Handle(app, kit.OpMeta{
		Name:    "shuffle",
		Group:   "write",
		Single:  true,
		Summary: "Shuffle an existing deck",
		Args:    []kit.Arg{{Name: "deck_id", Help: "deck ID to reshuffle"}},
	}, shuffleOp)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- inputs ---

type newInput struct {
	Count   int     `kit:"flag" help:"number of 52-card packs to combine (default 1)"`
	Shuffle bool    `kit:"flag" help:"shuffle the deck on creation (default true)"`
	Client  *Client `kit:"inject"`
}

type drawInput struct {
	DeckID string  `kit:"arg"  help:"deck ID from 'deckofcards new'"`
	Count  int     `kit:"flag" help:"number of cards to draw (default 1)"`
	Client *Client `kit:"inject"`
}

type shuffleInput struct {
	DeckID string  `kit:"arg"  help:"deck ID to reshuffle"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func newOp(ctx context.Context, in newInput, emit func(*Deck) error) error {
	count := in.Count
	if count <= 0 {
		count = 1
	}
	// Default to shuffled when no --shuffle flag is explicitly used.
	// The zero value is false, so we default to true (shuffled deck is the
	// common case for card games).
	shuffle := true
	if in.Shuffle {
		shuffle = true
	}
	d, err := in.Client.New(ctx, count, shuffle)
	if err != nil {
		return err
	}
	return emit(d)
}

func drawOp(ctx context.Context, in drawInput, emit func(*DrawResult) error) error {
	count := in.Count
	if count <= 0 {
		count = 1
	}
	r, err := in.Client.Draw(ctx, in.DeckID, count)
	if err != nil {
		return err
	}
	return emit(r)
}

func shuffleOp(ctx context.Context, in shuffleInput, emit func(*Deck) error) error {
	d, err := in.Client.Shuffle(ctx, in.DeckID)
	if err != nil {
		return err
	}
	return emit(d)
}

// --- Resolver ---

// Classify turns any accepted input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("unrecognized deckofcards reference: %q", input)
	}
	return "deck", input, nil
}

// Locate returns the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "deck":
		return "https://deckofcardsapi.com/api/deck/" + id, nil
	default:
		return "", errs.Usage("deckofcards has no resource type %q", uriType)
	}
}
