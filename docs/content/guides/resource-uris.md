---
title: "Resource URIs"
description: "Use deckofcards as a database/sql-style driver so a host program can address deckofcards as deckofcards:// URIs."
weight: 20
---

`deckofcards` is a command line, but the `deckofcards` Go package is also a
small driver that makes deckofcards addressable as a resource URI. A host
program registers it the way a program registers a database driver with
`database/sql`, then dereferences `deckofcards://` URIs without knowing
anything about how deckofcards is fetched.

The host that does this today is [ant](https://github.com/tamnd/ant), a single
binary that puts one URI namespace over a family of site tools. The examples
below use `ant`; any program that links the package gets the same behaviour.

## Mounting the driver

A host enables the driver with one blank import, exactly like `import _
"github.com/lib/pq"`:

```go
import _ "github.com/tamnd/deckofcards-cli/deckofcards"
```

The package's `init` registers a domain with the scheme `deckofcards` for the
host `deckofcards.com`. The standalone `deckofcards` binary does not change.

## Addressing records

A URI is `scheme://authority/id`. The scaffold ships one type:

| URI                              | What it is                              |
| -------------------------------- | --------------------------------------- |
| `deckofcards://page/<path>`    | a page, keyed by its path on deckofcards.com |

```bash
ant get deckofcards://page/<path>    # the page record
ant cat deckofcards://page/<path>    # just the body text
ant url deckofcards://page/<path>    # the live https URL
ant resolve https://deckofcards.com/<path> # a pasted link, back to its URI
```

As you add resolver operations in `deckofcards/domain.go`, each new `URIType`
becomes another addressable authority here, with no extra wiring. See
[add a command](/guides/adding-a-command/).

## Walking the graph

`ls` lists the members of a collection, and every member is itself an
addressable URI, so a host can follow the graph and write it to disk:

```bash
ant ls     deckofcards://page/<path>             # the pages this one links to
ant export deckofcards://page/<path> --follow 1 --to ./data
```

The example `links` op emits page stubs, so each listed member is a
`deckofcards://page/` URI in its own right. When you model edges between your
real records with `kit:"link"` tags, `ant export --follow` and `ant graph` walk
those edges too, across tools when a link points at another site's scheme.

## Why this is the same code

The driver and the binary share one definition per operation. A resolver op
answers both `deckofcards page` on the command line and `ant get
deckofcards://page/...` through a host, from the same handler and the same
client. There is no second implementation to keep in step.
