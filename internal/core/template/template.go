// Package template implements the host template dialect described in
// docs/specs/plugin-system.md §3. Templates are user-editable library data,
// interpreted by the host on the review critical path: pure data replacement,
// no scripting, no HTML/CSS, no third-party functions. Output is markdown.
package template

import (
	"errors"
	"fmt"
	"sort"
	"strings"
)

// Side selects which card face is being rendered.
type Side uint8

const (
	SideFront Side = iota
	SideBack
)

// Input is the render request: the card's field values, the ordinal that
// distinguishes derived cards from one Block (cloze blanks, bidirectional
// directions), and the face being rendered.
type Input struct {
	Fields  map[string]string
	Ordinal int // 0-based: matches core.Card.Ordinal, the n-th cloze blank counted from 0
	Side    Side
}

// Output is the rendered face. Hint and TypeAnswer are captured from
// {{hint:…}} / {{type:…}} tags, which render as empty inline.
type Output struct {
	Markdown   string
	Hint       string
	TypeAnswer string
}

// Sentinel errors. Rendering failures degrade to the field list via Fallback;
// callers match kinds with errors.Is and read offsets from *Error.
var (
	ErrSyntax        = errors.New("template: syntax error")
	ErrUnknownField  = errors.New("template: unknown field")
	ErrUnknownFilter = errors.New("template: unknown filter")
)

// Error is a dialect error carrying the byte offset of the offending
// construct in the template source, so the template editor can point at it.
type Error struct {
	Kind   error
	Offset int
	Msg    string
}

func (e *Error) Error() string {
	return fmt.Sprintf("%v (at offset %d): %s", e.Kind, e.Offset, e.Msg)
}

func (e *Error) Unwrap() error { return e.Kind }

func errAt(kind error, offset int, format string, args ...any) error {
	return &Error{Kind: kind, Offset: offset, Msg: fmt.Sprintf(format, args...)}
}

// Template is a compiled template. It is immutable: compile once with
// Compile and render any number of times with Render.
type Template struct {
	src   string
	nodes []node
}

// node is a parsed construct.
type node interface {
	render(ctx *renderContext)
}

type renderContext struct {
	in   Input
	out  *Output
	buf  strings.Builder
	fail error
}

func (ctx *renderContext) err(e error) {
	if ctx.fail == nil {
		ctx.fail = e
	}
}

type textNode struct{ text string }

func (n textNode) render(ctx *renderContext) { ctx.buf.WriteString(n.text) }

type fieldNode struct {
	name string
	off  int
}

func (n fieldNode) render(ctx *renderContext) {
	v, ok := ctx.in.Fields[n.name]
	if !ok {
		ctx.err(errAt(ErrUnknownField, n.off, "unknown field %q", n.name))
		return
	}
	ctx.buf.WriteString(v)
}

type sectionNode struct {
	name     string
	off      int
	nonEmpty bool // true for {{#…}} (render when non-empty), false for {{^…}}
	body     []node
}

func (n sectionNode) render(ctx *renderContext) {
	v, ok := ctx.in.Fields[n.name]
	if !ok {
		ctx.err(errAt(ErrUnknownField, n.off, "unknown field %q", n.name))
		return
	}
	empty := strings.TrimSpace(v) == ""
	if empty == n.nonEmpty {
		return
	}
	for _, child := range n.body {
		if ctx.fail != nil {
			return
		}
		child.render(ctx)
	}
}

type clozeNode struct {
	field string
	off   int
}

func (n clozeNode) render(ctx *renderContext) {
	v, ok := ctx.in.Fields[n.field]
	if !ok {
		ctx.err(errAt(ErrUnknownField, n.off, "unknown field %q", n.field))
		return
	}
	segs, blanks := splitCloze(v)
	// An ordinal outside the value's blanks asks none of them: every blank is
	// revealed on both sides, which is what the card plugin did for a blank a
	// paragraph no longer has.
	asked := ctx.in.Ordinal >= 0 && ctx.in.Ordinal < len(blanks)
	for i, seg := range segs {
		ctx.buf.WriteString(seg)
		if i == len(blanks) {
			break
		}
		if ctx.in.Side == SideFront && asked && i == ctx.in.Ordinal {
			ctx.buf.WriteString("[…]")
		} else {
			ctx.buf.WriteString(blanks[i])
		}
	}
}

// splitCloze splits v around {{…}} markers: len(segs) == len(blanks)+1.
// A "{{" with no closing "}}" is literal text.
func splitCloze(v string) (segs, blanks []string) {
	for {
		open := strings.Index(v, "{{")
		if open < 0 {
			break
		}
		close := strings.Index(v[open+2:], "}}")
		if close < 0 {
			break
		}
		segs = append(segs, v[:open])
		blanks = append(blanks, v[open+2:open+2+close])
		v = v[open+2+close+2:]
	}
	segs = append(segs, v)
	return segs, blanks
}

type hintNode struct {
	field string
	off   int
}

func (n hintNode) render(ctx *renderContext) {
	v, ok := ctx.in.Fields[n.field]
	if !ok {
		ctx.err(errAt(ErrUnknownField, n.off, "unknown field %q", n.field))
		return
	}
	ctx.out.Hint = v // captured; renders as empty inline
}

type typeNode struct {
	field string
	off   int
}

func (n typeNode) render(ctx *renderContext) {
	v, ok := ctx.in.Fields[n.field]
	if !ok {
		ctx.err(errAt(ErrUnknownField, n.off, "unknown field %q", n.field))
		return
	}
	ctx.out.TypeAnswer = v // captured; renders as empty inline
}

// Compile parses src into a Template. All syntax errors, unknown constructs,
// filter references (the v1 filter registry is empty; the pipe syntax is
// reserved) and duplicate hint/type tags surface here with their offsets.
func Compile(src string) (*Template, error) {
	p := &parser{src: src}
	nodes, err := p.parseNodes("", 0)
	if err != nil {
		return nil, err
	}
	return &Template{src: src, nodes: nodes}, nil
}

type parser struct {
	src   string
	pos   int
	hint  bool
	typed bool
}

// parseNodes parses constructs until EOF (closing == "") or until the close
// tag {{/closing}}; closingOffset is where the section opened, for the
// unclosed-section error.
func (p *parser) parseNodes(closing string, closingOffset int) ([]node, error) {
	var nodes []node
	for p.pos < len(p.src) {
		open := strings.Index(p.src[p.pos:], "{{")
		if open < 0 {
			nodes = append(nodes, textNode{p.src[p.pos:]})
			p.pos = len(p.src)
			break
		}
		open += p.pos
		if open > p.pos {
			nodes = append(nodes, textNode{p.src[p.pos:open]})
		}
		end := strings.Index(p.src[open+2:], "}}")
		if end < 0 {
			return nil, errAt(ErrSyntax, open, "unclosed tag")
		}
		tag := p.src[open+2 : open+2+end]
		next := open + 2 + end + 2
		switch {
		case tag == "":
			return nil, errAt(ErrSyntax, open, "empty tag")
		case tag[0] == '#' || tag[0] == '^':
			name := tag[1:]
			if name == "" {
				return nil, errAt(ErrSyntax, open, "section tag without field name")
			}
			p.pos = next
			body, err := p.parseNodes(name, open)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, &sectionNode{name: name, off: open, nonEmpty: tag[0] == '#', body: body})
		case tag[0] == '/':
			name := tag[1:]
			if name == "" {
				return nil, errAt(ErrSyntax, open, "close tag without field name")
			}
			if closing == "" {
				return nil, errAt(ErrSyntax, open, "unexpected close tag {{/%s}}", name)
			}
			if name != closing {
				return nil, errAt(ErrSyntax, open, "mismatched close tag {{/%s}}, expected {{/%s}}", name, closing)
			}
			p.pos = next
			return nodes, nil
		case strings.Contains(tag, "|"):
			parts := strings.SplitN(tag, "|", 2)
			return nil, errAt(ErrUnknownFilter, open, "filter %q on field %q is not registered", parts[1], parts[0])
		case strings.HasPrefix(tag, "cloze:"):
			if err := p.claimSpecial(open, tag, "cloze"); err != nil {
				return nil, err
			}
			nodes = append(nodes, clozeNode{field: tag[len("cloze:"):], off: open})
			p.pos = next
		case strings.HasPrefix(tag, "hint:"):
			if err := p.claimSpecial(open, tag, "hint"); err != nil {
				return nil, err
			}
			if p.hint {
				return nil, errAt(ErrSyntax, open, "duplicate {{hint:…}} tag: at most one per template")
			}
			p.hint = true
			nodes = append(nodes, hintNode{field: tag[len("hint:"):], off: open})
			p.pos = next
		case strings.HasPrefix(tag, "type:"):
			if err := p.claimSpecial(open, tag, "type"); err != nil {
				return nil, err
			}
			if p.typed {
				return nil, errAt(ErrSyntax, open, "duplicate {{type:…}} tag: at most one per template")
			}
			p.typed = true
			nodes = append(nodes, typeNode{field: tag[len("type:"):], off: open})
			p.pos = next
		case strings.Contains(tag, ":"):
			return nil, errAt(ErrSyntax, open, "unknown construct %q", tag)
		default:
			nodes = append(nodes, fieldNode{name: tag, off: open})
			p.pos = next
		}
	}
	if closing != "" {
		return nil, errAt(ErrSyntax, closingOffset, "unclosed section for field %q", closing)
	}
	return nodes, nil
}

func (p *parser) claimSpecial(offset int, tag, keyword string) error {
	if len(tag) <= len(keyword)+1 {
		return errAt(ErrSyntax, offset, "{{%s:…}} without field name", keyword)
	}
	return nil
}

// Render renders one face. The returned error is nil or a *Error carrying a
// sentinel kind and the offset of the offending construct.
func (t *Template) Render(in Input) (Output, error) {
	ctx := &renderContext{in: in, out: &Output{}}
	for _, n := range t.nodes {
		if ctx.fail != nil {
			break
		}
		n.render(ctx)
	}
	if ctx.fail != nil {
		return Output{}, ctx.fail
	}
	ctx.out.Markdown = strings.TrimRight(ctx.buf.String(), " \t\r\n")
	return *ctx.out, nil
}

// Fallback renders fields as a "name: value" markdown list. It is the
// degradation path when a template is missing or fails to render (spec §2.1:
// never a blank card) and itself never returns a blank string.
func Fallback(fields map[string]string) string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	if len(names) == 0 {
		return "- (no fields)"
	}
	var b strings.Builder
	for _, name := range names {
		b.WriteString("- ")
		b.WriteString(name)
		b.WriteString(": ")
		b.WriteString(fields[name])
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n")
}
