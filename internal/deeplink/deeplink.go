// Package deeplink parses the remit:// URLs the app registers, so that an
// external caller can point the app at one place and only one place
// (decision #25). Anything else is refused rather than guessed.
package deeplink

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/transmit-bug/remit/internal/core"
)

// Scheme is the URL scheme the app registers.
const Scheme = "remit"

// Kind is what a link points at.
type Kind uint8

const (
	KindNote Kind = iota
	KindCard
	KindQueue
	KindSearch
)

// Route is a parsed link.
type Route struct {
	Kind   Kind
	ID     core.ID // KindNote, KindCard
	Query  string  // KindSearch
	Search bool    // KindSearch only
}

// Parse turns a remit:// URL into a route. It accepts no file paths and no
// unknown hosts: a link from another app is untrusted input.
func Parse(raw string) (Route, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return Route{}, fmt.Errorf("deeplink: %q: %w", raw, err)
	}
	if u.Scheme != Scheme {
		return Route{}, fmt.Errorf("deeplink: scheme %q is not %s", u.Scheme, Scheme)
	}
	host := strings.ToLower(u.Host)
	rest := strings.TrimPrefix(u.EscapedPath(), "/")
	id := strings.TrimSpace(rest)

	switch host {
	case "queue":
		if id != "" || u.RawQuery != "" {
			return Route{}, fmt.Errorf("deeplink: queue takes no argument")
		}
		return Route{Kind: KindQueue}, nil
	case "search":
		q := u.Query().Get("q")
		if q == "" {
			return Route{}, fmt.Errorf("deeplink: search needs q")
		}
		return Route{Kind: KindSearch, Query: q, Search: true}, nil
	case "note", "card":
		decoded, err := url.PathUnescape(id)
		if err != nil {
			return Route{}, fmt.Errorf("deeplink: %q: %w", raw, err)
		}
		entity := core.ID(decoded)
		if !entity.Valid() {
			return Route{}, fmt.Errorf("deeplink: %q is not an id", decoded)
		}
		if strings.ContainsAny(decoded, `/\.`) {
			return Route{}, fmt.Errorf("deeplink: %q is not an id", decoded)
		}
		kind := KindNote
		if host == "card" {
			kind = KindCard
		}
		return Route{Kind: kind, ID: entity}, nil
	}
	return Route{}, fmt.Errorf("deeplink: unknown target %q", u.Host)
}

// String renders a route back into a URL, which is what the app registers and
// what a test compares against.
func (r Route) String() string {
	switch r.Kind {
	case KindQueue:
		return Scheme + "://queue"
	case KindSearch:
		return Scheme + "://search?q=" + url.QueryEscape(r.Query)
	case KindCard:
		return Scheme + "://card/" + url.PathEscape(string(r.ID))
	default:
		return Scheme + "://note/" + url.PathEscape(string(r.ID))
	}
}
