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
	// The three areas a link may name: today, the reading queue and the recall
	// queue. They take no argument, as queue does.
	KindToday
	KindRead
	KindRecall
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
	case "queue", "today", "read", "recall":
		if id != "" || u.RawQuery != "" {
			return Route{}, fmt.Errorf("deeplink: %s takes no argument", host)
		}
		return Route{Kind: areaKind(host)}, nil
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

// areaKind is the route a host names, for the links that take no argument.
func areaKind(host string) Kind {
	switch host {
	case "today":
		return KindToday
	case "read":
		return KindRead
	case "recall":
		return KindRecall
	}
	return KindQueue
}

// String renders a route back into a URL, which is what the app registers and
// what a test compares against.
func (r Route) String() string {
	switch r.Kind {
	case KindToday:
		return Scheme + "://today"
	case KindRead:
		return Scheme + "://read"
	case KindRecall:
		return Scheme + "://recall"
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
