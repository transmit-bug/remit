package contrib

import (
	"fmt"
	"strconv"
	"strings"
)

// Context is the window's state a when expression reads, as the context keys of
// the contribution contract name it (docs/specs/plugins/ui.md §3). A key the
// host adds is a version of its API, never a field here that a caller guesses.
type Context struct {
	// Mode is the screen the window shows: notes, review, browse, stats or
	// settings. A plugin page is none of those, so it is empty there and
	// PageID says which page it is.
	Mode string
	// LibraryOpen is a library read into the window.
	LibraryOpen bool
	// NoteOpen is a note in the editor.
	NoteOpen bool
	// NoteSelection is a selection inside that note's text.
	NoteSelection bool
	// CardPresent is a card on screen, in review or in browse.
	CardPresent bool
	// CardStatus is that card's standing: new, learning, review, suspended or
	// lost.
	CardStatus string
	// BrowseSelection is a chosen row in the card list.
	BrowseSelection bool
	// ScopeActive is a review scope narrower than the whole library. No screen
	// narrows one yet.
	ScopeActive bool
	// PageID is the plugin page on screen, and PluginID the plugin it belongs
	// to; both empty elsewhere.
	PageID   string
	PluginID string
}

// value is what a context key answers and a literal is: bool, string or
// number. Comparisons are total — two values of different kinds are unequal
// rather than an error — so a when expression can never fail at run time and
// hide a contribution by accident.
func (c Context) value(key string) (any, bool) { return contextValue(key, c) }

// contextValue is the frozen key table, one entry per key the contract names.
func contextValue(key string, c Context) (any, bool) {
	switch key {
	case "mode":
		return c.Mode, true
	case "library.open":
		return c.LibraryOpen, true
	case "note.open":
		return c.NoteOpen, true
	case "note.selection":
		return c.NoteSelection, true
	case "card.present":
		return c.CardPresent, true
	case "card.status":
		return c.CardStatus, true
	case "browse.selection":
		return c.BrowseSelection, true
	case "scope.active":
		return c.ScopeActive, true
	case "page.id":
		return c.PageID, true
	case "plugin.id":
		return c.PluginID, true
	}
	return nil, false
}

// Expr is a when expression, compiled once when the contribution was read: what
// a frame does is Eval, and it costs a walk of a handful of nodes.
//
// The zero Expr holds everywhere: a contribution without a when is visible.
type Expr struct {
	root node
	src  string
}

// Compile parses a when expression. The grammar is the contract's and no more:
// unary !, &&, ||, ==, !=, parentheses, and string or number literals. The
// warnings name the context keys this host does not know — they read false, so
// a contribution written for a newer host stays hidden rather than wrong.
func Compile(src string) (Expr, []string, error) {
	if strings.TrimSpace(src) == "" {
		return Expr{}, nil, nil
	}
	tokens, err := lex(src)
	if err != nil {
		return Expr{}, nil, err
	}
	p := exprParser{tokens: tokens}
	root, err := p.expr()
	if err != nil {
		return Expr{}, nil, err
	}
	if p.peek().kind != tokEOF {
		return Expr{}, nil, fmt.Errorf("when: %q: unexpected %q", src, p.peek().text)
	}
	return Expr{root: root, src: src}, p.warnings, nil
}

// Eval answers whether the expression holds in ctx.
func (e Expr) Eval(ctx Context) bool {
	if e.root == nil {
		return true
	}
	return truthy(e.root.eval(ctx))
}

// String is the expression as the plugin wrote it, for a message that names it.
func (e Expr) String() string { return e.src }

// When compiles and evaluates one expression, for a caller that holds no index:
// a test, or a tool checking what a manifest would show.
func When(src string, ctx Context) bool {
	expr, _, err := Compile(src)
	if err != nil {
		return false
	}
	return expr.Eval(ctx)
}

// node is one compiled expression.
type node interface {
	eval(Context) any
}

type literal struct{ v any }

func (l literal) eval(Context) any { return l.v }

type key struct{ name string }

// key.eval answers the context's value, or nil for a key this host does not
// know: nil is false, and compares equal to nothing.
func (k key) eval(ctx Context) any {
	v, _ := ctx.value(k.name)
	return v
}

type notNode struct{ inner node }

func (n notNode) eval(ctx Context) any { return !truthy(n.inner.eval(ctx)) }

type andNode struct{ left, right node }

func (n andNode) eval(ctx Context) any {
	return truthy(n.left.eval(ctx)) && truthy(n.right.eval(ctx))
}

type orNode struct{ left, right node }

func (n orNode) eval(ctx Context) any {
	return truthy(n.left.eval(ctx)) || truthy(n.right.eval(ctx))
}

type compareNode struct {
	left, right node
	equal       bool
}

func (n compareNode) eval(ctx Context) any {
	same := sameValue(n.left.eval(ctx), n.right.eval(ctx))
	if n.equal {
		return same
	}
	return !same
}

// sameValue is equality across the three kinds: a bool, a string or a number,
// and nothing else equals anything. A float64 comparison is exact because the
// only numbers a when expression holds are literals and whole-valued keys.
func sameValue(a, b any) bool {
	switch left := a.(type) {
	case bool:
		right, ok := b.(bool)
		return ok && left == right
	case string:
		right, ok := b.(string)
		return ok && left == right
	case float64:
		right, ok := b.(float64)
		return ok && left == right
	}
	return false
}

// truthy is how an expression's value reads as a condition: a bool as itself, a
// string or number as whether it is there.
func truthy(v any) bool {
	switch value := v.(type) {
	case bool:
		return value
	case string:
		return value != ""
	case float64:
		return value != 0
	}
	return false
}

type tokKind int

const (
	tokEOF tokKind = iota
	tokIdent
	tokString
	tokNumber
	tokNot
	tokAnd
	tokOr
	tokEq
	tokNe
	tokLParen
	tokRParen
)

type token struct {
	kind tokKind
	text string
}

func lex(src string) ([]token, error) {
	var out []token
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '(':
			out = append(out, token{tokLParen, "("})
			i++
		case c == ')':
			out = append(out, token{tokRParen, ")"})
			i++
		case c == '!':
			if i+1 < len(src) && src[i+1] == '=' {
				out = append(out, token{tokNe, "!="})
				i += 2
				continue
			}
			out = append(out, token{tokNot, "!"})
			i++
		case c == '=':
			if i+1 >= len(src) || src[i+1] != '=' {
				return nil, fmt.Errorf("when: %q: a single = is not a comparison; use ==", src)
			}
			out = append(out, token{tokEq, "=="})
			i += 2
		case c == '&':
			if i+1 >= len(src) || src[i+1] != '&' {
				return nil, fmt.Errorf("when: %q: a single & is not a conjunction; use &&", src)
			}
			out = append(out, token{tokAnd, "&&"})
			i += 2
		case c == '|':
			if i+1 >= len(src) || src[i+1] != '|' {
				return nil, fmt.Errorf("when: %q: a single | is not a disjunction; use ||", src)
			}
			out = append(out, token{tokOr, "||"})
			i += 2
		case c == '"' || c == '\'':
			end := strings.IndexByte(src[i+1:], c)
			if end < 0 {
				return nil, fmt.Errorf("when: %q: no closing %c", src, c)
			}
			out = append(out, token{tokString, src[i+1 : i+1+end]})
			i += end + 2
		case c >= '0' && c <= '9':
			start := i
			for i < len(src) && (src[i] >= '0' && src[i] <= '9' || src[i] == '.') {
				i++
			}
			out = append(out, token{tokNumber, src[start:i]})
		case isIdentStart(c):
			start := i
			for i < len(src) && isIdentPart(src[i]) {
				i++
			}
			out = append(out, token{tokIdent, src[start:i]})
		default:
			return nil, fmt.Errorf("when: %q: %q is not part of an expression", src, string(c))
		}
	}
	return append(out, token{tokEOF, ""}), nil
}

func isIdentStart(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isIdentPart(c byte) bool {
	return isIdentStart(c) || c >= '0' && c <= '9' || c == '.' || c == '-'
}

// exprParser is a recursive descent over the tokens: or, then and, then
// comparison, then unary not, then a primary. The precedence is the one every
// language a plugin author has written in has, so a when expression reads the
// same here.
type exprParser struct {
	tokens   []token
	i        int
	warnings []string
	unknown  map[string]bool
}

func (p *exprParser) peek() token { return p.tokens[p.i] }

func (p *exprParser) next() token {
	t := p.tokens[p.i]
	if t.kind != tokEOF {
		p.i++
	}
	return t
}

func (p *exprParser) expr() (node, error) { return p.or() }

func (p *exprParser) or() (node, error) {
	left, err := p.and()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokOr {
		p.next()
		right, err := p.and()
		if err != nil {
			return nil, err
		}
		left = orNode{left, right}
	}
	return left, nil
}

func (p *exprParser) and() (node, error) {
	left, err := p.comparison()
	if err != nil {
		return nil, err
	}
	for p.peek().kind == tokAnd {
		p.next()
		right, err := p.comparison()
		if err != nil {
			return nil, err
		}
		left = andNode{left, right}
	}
	return left, nil
}

func (p *exprParser) comparison() (node, error) {
	left, err := p.unary(false)
	if err != nil {
		return nil, err
	}
	kind := p.peek().kind
	if kind != tokEq && kind != tokNe {
		return left, nil
	}
	p.next()
	right, err := p.unary(true)
	if err != nil {
		return nil, err
	}
	return compareNode{left: left, right: right, equal: kind == tokEq}, nil
}

// unary parses one operand. value says whether it stands beside a comparison,
// where an identifier the contract does not name is a string to compare with
// (`mode == review`) rather than a key to read and hide on.
func (p *exprParser) unary(value bool) (node, error) {
	if p.peek().kind == tokNot {
		p.next()
		inner, err := p.unary(value)
		if err != nil {
			return nil, err
		}
		return notNode{inner}, nil
	}
	return p.primary(value)
}

func (p *exprParser) primary(value bool) (node, error) {
	t := p.next()
	switch t.kind {
	case tokLParen:
		inner, err := p.expr()
		if err != nil {
			return nil, err
		}
		if p.next().kind != tokRParen {
			return nil, fmt.Errorf("when: no closing )")
		}
		return inner, nil
	case tokString:
		return literal{t.text}, nil
	case tokNumber:
		n, err := strconv.ParseFloat(t.text, 64)
		if err != nil {
			return nil, fmt.Errorf("when: %q is not a number", t.text)
		}
		return literal{n}, nil
	case tokIdent:
		switch t.text {
		case "true":
			return literal{true}, nil
		case "false":
			return literal{false}, nil
		}
		// A word the contract names is a context key. A bare word that is not
		// one is a value, so that `mode == review` reads as the contract's
		// example does; a dotted word that is not a key is a newer host's, and
		// reads false with a warning rather than being taken for a string.
		if _, ok := contextValue(t.text, Context{}); ok {
			return key{t.text}, nil
		}
		if value || !strings.Contains(t.text, ".") {
			return literal{t.text}, nil
		}
		p.warn(t.text)
		return key{t.text}, nil
	}
	return nil, fmt.Errorf("when: unexpected %q", t.text)
}

// warn records an unknown key once per expression: a plugin asking about three
// keys this host lacks reads one line, not three.
func (p *exprParser) warn(name string) {
	if p.unknown == nil {
		p.unknown = map[string]bool{}
	}
	if p.unknown[name] {
		return
	}
	p.unknown[name] = true
	p.warnings = append(p.warnings, fmt.Sprintf("when: %q is not a context key of this host", name))
}
