// Package maths reads the TeX a note writes between dollars and sets it as text.
//
// MyGo's markdown hands maths back to the app to draw (ui/markdown's Math and
// InlineMath take the TeX and expect a painter); Remit has no TeX engine and
// does not want one in the binary, so it transcribes the subset its notes use
// into Unicode: superscripts and subscripts, fractions, roots and the Greek and
// operator names below. What it cannot set is shown as it was written, so a
// formula always reads as something.
package maths

import (
	"strings"
	"unicode"
)

// Segment is a run of a note's text: prose, or a formula.
type Segment struct {
	Text string
	Math bool
}

// Segments splits text into its prose and its formulas. A formula is what sits
// between pairs of dollars, one dollar for inline and two for a display formula;
// an unpaired dollar is prose.
func Segments(text string) []Segment {
	var out []Segment
	for {
		open := strings.IndexByte(text, '$')
		if open < 0 {
			return appendProse(out, text)
		}
		delim := "$"
		if strings.HasPrefix(text[open:], "$$") {
			delim = "$$"
		}
		rest := text[open+len(delim):]
		end := strings.Index(rest, delim)
		if end < 0 {
			return appendProse(out, text)
		}
		out = appendProse(out, text[:open])
		out = append(out, Segment{Text: Transcribe(rest[:end]), Math: true})
		text = rest[end+len(delim):]
	}
}

// appendProse adds prose when there is any.
func appendProse(out []Segment, text string) []Segment {
	if text == "" {
		return out
	}
	return append(out, Segment{Text: text})
}

// Render is a note's text as it reads with its formulas set: the prose as it was
// written, each formula transcribed. It is what the reading and review surfaces
// show; the editor keeps the source.
func Render(text string) string {
	var out strings.Builder
	for _, seg := range Segments(text) {
		out.WriteString(seg.Text)
	}
	return out.String()
}

// Transcribe sets TeX as Unicode text. It knows the letters and operators a
// note about learning uses, superscripts and subscripts, fractions, roots and
// \\text{…}; anything else keeps its own text, with its commands stripped.
func Transcribe(tex string) string {
	var out strings.Builder
	last := rune(0)
	unknown := false
	write := func(s string) {
		out.WriteString(s)
		for _, r := range s {
			last = r
		}
	}
	for i := 0; i < len(tex); {
		switch {
		case tex[i] == '\\':
			name, next := command(tex, i)
			i = next
			switch name {
			case "frac":
				num, after := group(tex, i)
				den, after2 := group(tex, after)
				i = after2
				write(Transcribe(num) + "/" + Transcribe(den))
				unknown = true
			case "sqrt":
				arg, after := group(tex, i)
				i = after
				write("√" + Transcribe(arg))
				unknown = true
			case "text", "mathrm", "mathbf", "operatorname":
				arg, after := group(tex, i)
				i = after
				write(arg)
			case "left", "right", "displaystyle", "limits", "big", "Big":
				// Styling with no meaning in text.
			case ",", ";", ":", "!", " ", "quad", "qquad":
				write(" ")
			default:
				if s, ok := symbols[name]; ok {
					write(s)
				} else if s, ok := greek[name]; ok {
					write(s)
				} else {
					write(name)
					// A name that is not a symbol is a word of the formula, and
					// the space after a word reaches the page.
					unknown = true
				}
			}
		case tex[i] == '^' || tex[i] == '_':
			up := tex[i] == '^'
			arg, next := group(tex, i+1)
			i = next
			if up {
				write(shift(Transcribe(arg), superscripts))
			} else {
				write(shift(Transcribe(arg), subscripts))
			}
		case tex[i] == '{' || tex[i] == '}':
			i++
		case tex[i] == ' ' || tex[i] == '\t' || tex[i] == '\n':
			// TeX's own space is between words, not between letters: it is kept
			// where an operator is on either side, and dropped where two
			// letters stand for a product, as \pi r is.
			for i < len(tex) && (tex[i] == ' ' || tex[i] == '\t' || tex[i] == '\n') {
				i++
			}
			if last != 0 && !strings.HasSuffix(out.String(), " ") && (unknown || operator(last) || nextOperator(tex, i)) {
				write(" ")
			}
			unknown = false
		default:
			write(string(tex[i]))
			i++
		}
	}
	return strings.TrimSpace(out.String())
}

// command reads a command name and returns it with where its text resumes. A
// backslash before a non-letter is that character's own name, as TeX's \\, \\{
// and \\% are.
func command(tex string, at int) (name string, next int) {
	i := at + 1
	if i >= len(tex) {
		return "", i
	}
	if !isLetter(rune(tex[i])) {
		return string(tex[i]), i + 1
	}
	start := i
	for i < len(tex) && isLetter(rune(tex[i])) {
		i++
	}
	return tex[start:i], i
}

// group reads a braced argument, or the single token after the command when
// there are no braces, as TeX's own rules do.
func group(tex string, at int) (arg string, next int) {
	i := at
	for i < len(tex) && (tex[i] == ' ' || tex[i] == '\t' || tex[i] == '\n') {
		i++
	}
	if i < len(tex) && tex[i] == '{' {
		depth := 0
		start := i + 1
		for ; i < len(tex); i++ {
			if tex[i] == '{' {
				depth++
				continue
			}
			if tex[i] == '}' {
				depth--
				if depth == 0 {
					return tex[start:i], i + 1
				}
			}
		}
		return tex[start:], len(tex)
	}
	if i < len(tex) && tex[i] == '\\' {
		name, after := command(tex, i)
		return name, after
	}
	if i < len(tex) {
		return string(tex[i]), i + 1
	}
	return "", i
}

// nextOperator reports whether what follows at is an operator, so that the space
// before it reaches the page.
func nextOperator(tex string, at int) bool {
	if at >= len(tex) {
		return false
	}
	if tex[at] == '\\' {
		name, _ := command(tex, at)
		s, ok := symbols[name]
		return ok && operator([]rune(s)[0])
	}
	return operator(rune(tex[at]))
}

// operator reports whether r is a symbol a formula spaces around.
func operator(r rune) bool {
	return strings.ContainsRune("+-*/=<>()[]≤≥≠≈≡∝·×÷±∓→←⇒∈∉∑∏∫√", r)
}

func isLetter(r rune) bool { return unicode.IsLetter(r) }

// shift sets every character of s as a superscript or a subscript, leaving the
// ones Unicode has no form for as they are.
func shift(s string, table map[rune]rune) string {
	var out strings.Builder
	for _, r := range s {
		if to, ok := table[r]; ok {
			out.WriteRune(to)
			continue
		}
		out.WriteRune(r)
	}
	return out.String()
}

var superscripts = map[rune]rune{
	'0': '⁰', '1': '¹', '2': '²', '3': '³', '4': '⁴', '5': '⁵', '6': '⁶', '7': '⁷', '8': '⁸', '9': '⁹',
	'+': '⁺', '-': '⁻', '=': '⁼', '(': '⁽', ')': '⁾', 'n': 'ⁿ', 'i': 'ⁱ', 'a': 'ᵃ', 'b': 'ᵇ', 'c': 'ᶜ',
	'd': 'ᵈ', 'e': 'ᵉ', 'k': 'ᵏ', 'm': 'ᵐ', 'o': 'ᵒ', 'p': 'ᵖ', 't': 'ᵗ', 'u': 'ᵘ', 'v': 'ᵛ', 'w': 'ʷ',
	'x': 'ˣ', 'y': 'ʸ', 'z': 'ᶻ',
}

var subscripts = map[rune]rune{
	'0': '₀', '1': '₁', '2': '₂', '3': '₃', '4': '₄', '5': '₅', '6': '₆', '7': '₇', '8': '₈', '9': '₉',
	'+': '₊', '-': '₋', '=': '₌', '(': '₍', ')': '₎', 'a': 'ₐ', 'e': 'ₑ', 'i': 'ᵢ', 'j': 'ⱼ', 'n': 'ₙ',
	'o': 'ₒ', 'p': 'ₚ', 'r': 'ᵣ', 's': 'ₛ', 't': 'ₜ', 'u': 'ᵤ', 'v': 'ᵥ', 'x': 'ₓ',
}

var greek = map[string]string{
	"alpha": "α", "beta": "β", "gamma": "γ", "delta": "δ", "epsilon": "ε", "zeta": "ζ",
	"eta": "η", "theta": "θ", "iota": "ι", "kappa": "κ", "lambda": "λ", "mu": "μ", "nu": "ν",
	"xi": "ξ", "pi": "π", "rho": "ρ", "sigma": "σ", "tau": "τ", "upsilon": "υ", "phi": "φ",
	"chi": "χ", "psi": "ψ", "omega": "ω",
	"Gamma": "Γ", "Delta": "Δ", "Theta": "Θ", "Lambda": "Λ", "Xi": "Ξ", "Pi": "Π",
	"Sigma": "Σ", "Upsilon": "Υ", "Phi": "Φ", "Psi": "Ψ", "Omega": "Ω",
}

var symbols = map[string]string{
	"cdot": "·", "times": "×", "div": "÷", "pm": "±", "mp": "∓",
	"le": "≤", "leq": "≤", "ge": "≥", "geq": "≥", "ne": "≠", "neq": "≠",
	"approx": "≈", "equiv": "≡", "propto": "∝", "infty": "∞",
	"to": "→", "rightarrow": "→", "leftarrow": "←", "Rightarrow": "⇒",
	"in": "∈", "notin": "∉", "subset": "⊂", "cup": "∪", "cap": "∩",
	"sum": "∑", "prod": "∏", "int": "∫", "partial": "∂", "nabla": "∇",
	"ldots": "…", "dots": "…", "circ": "∘", "star": "⋆",
}
