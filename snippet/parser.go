package snippet

import (
	"bytes"
	"regexp"
	"strconv"
)

type TokenType int

const (
	Dollar TokenType = iota
	Colon
	CurlyOpen
	CurlyClose
	Backslash
	Int
	VariableName
	Format
	EOF
)

type Token struct {
	kind TokenType
	pos  int
	len  int
}

var table = map[rune]TokenType{
	'$':  Dollar,
	':':  Colon,
	'{':  CurlyOpen,
	'}':  CurlyClose,
	'\\': Backslash,
}

type Scanner struct {
	value []rune
	pos   int
}

func isDigitCharacter(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func isVariableCharacter(ch rune) bool {
	return ch == '_' || (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func newScanner() *Scanner {
	s := Scanner{}
	s.text("")

	return &s
}

func (s *Scanner) text(value string) {
	s.value = []rune(value)
	s.pos = 0
}

func (s *Scanner) tokenText(token *Token) string {
	return string(s.value[token.pos : token.pos+token.len])
}

func (s *Scanner) next() *Token {
	valueLen := len(s.value)
	if s.pos >= valueLen {
		return &Token{kind: EOF, pos: s.pos, len: 0}
	}

	pos := s.pos
	len := 0
	ch := s.value[pos]

	// Known token types.
	var t TokenType
	if t, ok := table[ch]; ok {
		s.pos++
		return &Token{kind: t, pos: pos, len: 1}
	}

	// Number token.
	if isDigitCharacter(ch) {
		t = Int
		for pos+len < valueLen {
			ch = s.value[pos+len]
			if !isDigitCharacter(ch) {
				break
			}
			len++
		}

		s.pos += len
		return &Token{t, pos, len}
	}

	// Variable.
	if isVariableCharacter(ch) {
		t = VariableName
		for pos+len < valueLen {
			ch = s.value[pos+len]
			if !isVariableCharacter(ch) && !isDigitCharacter(ch) {
				break
			}
			len++
		}

		s.pos += len
		return &Token{t, pos, len}
	}

	// format
	t = Format
	for pos+len < valueLen {
		ch = s.value[pos+len]
		len++
		_, isStaticToken := table[ch]
		if isStaticToken || isDigitCharacter(ch) || isVariableCharacter(ch) {
			break
		}
	}

	s.pos += len
	return &Token{t, pos, len}
}

//
// Marker.
//

type Marker interface {
	children() Markers
	setChildren(markers Markers)
	parent() Marker
	setParent(p Marker)
	String() string
	len() int
}

type Markers []Marker

func (ms *Markers) append(m ...Marker) {
	*ms = append(*ms, m...)
}

func (ms Markers) String() string {
	var buf bytes.Buffer

	for _, m := range ms {
		buf.WriteString(m.String())
	}
	return buf.String()
}

type markerImpl struct {
	// _markerBrand: any;
	_children Markers
	_parent   Marker
}

func newMarkerImpl() *markerImpl {
	return newMarkerImplWithChildren(Markers{})
}

func newMarkerImplWithChildren(children Markers) *markerImpl {
	return &markerImpl{
		_children: children,
	}
}

func (mi *markerImpl) children() Markers {
	return mi._children
}

func (mi *markerImpl) setChildren(markers Markers) {
	mi._children = Markers{}
	for _, m := range markers {
		m.setParent(mi)
		mi._children = append(mi._children, m)
	}
}

func (mi *markerImpl) parent() Marker {
	return mi._parent
}

func (mi *markerImpl) setParent(p Marker) {
	mi._parent = p
}

func (mi *markerImpl) String() string {
	return ""
}

func (mi *markerImpl) len() int {
	return 0
}

//
// Text.
//

type Text struct {
	markerImpl
	data string
}

func newText(data string) *Text {
	return &Text{
		markerImpl: *newMarkerImpl(),
		data:       data,
	}
}

func (t *Text) String() string {
	return t.data
}

func (t *Text) len() int {
	return len(t.data)
}

//
// Placeholder.
//

type Placeholder struct {
	markerImpl
	index int
}

func newPlaceholder(index int, children Markers) *Placeholder {
	return &Placeholder{
		markerImpl: *newMarkerImplWithChildren(children),
		index:      index,
	}
}

func (p *Placeholder) String() string {
	return p._children.String()
}

func (p *Placeholder) isFinalTabstop() bool {
	return p.index == 0
}

func compareByIndex(a Placeholder, b Placeholder) int {
	if a.index == b.index {
		return 0
	} else if a.isFinalTabstop() {
		return 1
	} else if b.isFinalTabstop() {
		return -1
	} else if a.index < b.index {
		return -1
	} else if a.index > b.index {
		return 1
	}
	return 0
}

//
// Variable.
//

type Variable struct {
	markerImpl
	resolvedValue *string
	name          string
}

func newVariable(name string, children Markers) *Variable {
	return &Variable{
		markerImpl: *newMarkerImplWithChildren(children),
		name:       name,
	}
}

func (v *Variable) isDefined() bool {
	return v.resolvedValue != nil
}

func (v *Variable) len() int {
	if v.isDefined() {
		return len(*v.resolvedValue)
	}
	return v.markerImpl.len()
}

func (v *Variable) String() string {
	if v.isDefined() {
		return *v.resolvedValue
	}
	return v._children.String()
}

func walk(markers Markers, visitor func(marker Marker) bool) {
	var stack Markers
	copy(stack, markers)

	for len(stack) > 0 {
		// NOTE: Declare `marker` separately so that we can use the `=` operator
		// (rather than `:=`) to make it clear that we're not shadowing `stack`.
		var marker Marker
		marker, stack = stack[0], stack[1:]
		recurse := visitor(marker)
		if !recurse {
			break
		}
		stack = append(marker.children(), stack...)
	}
}

//
// TextMate Snippet.
//

type TextmateSnippet struct {
	markerImpl
	_placeholders *[]*Placeholder
}

func newTextmateSnippet(children Markers) *TextmateSnippet {
	return &TextmateSnippet{
		markerImpl:    *newMarkerImplWithChildren(children),
		_placeholders: nil,
	}
}

func (tms *TextmateSnippet) placeholders() []*Placeholder {
	if tms._placeholders == nil {
		// Fill in placeholders if they don't exist.
		tms._placeholders = &[]*Placeholder{}
		walk(tms._children, func(candidate Marker) bool {
			switch candidate.(type) {
			case *Placeholder:
				{
					*tms._placeholders = append(*tms._placeholders, candidate.(*Placeholder))
				}
			}
			return true
		})
	}
	return *tms._placeholders
}

func (tms *TextmateSnippet) offset(marker Marker) int {
	pos := 0
	found := false
	walk(tms._children, func(candidate Marker) bool {
		if candidate == marker {
			found = true
			return false
		}
		pos += candidate.len()
		return true
	})

	if !found {
		return -1
	}
	return pos
}

func (tms *TextmateSnippet) fullLen(marker Marker) int {
	ret := 0
	walk([]Marker{marker}, func(marker Marker) bool {
		ret += marker.len()
		return true
	})
	return ret
}

func (tms *TextmateSnippet) enclosingPlaceholders(placeholder Placeholder) []*Placeholder {
	ret := []*Placeholder{}
	parent := placeholder._parent
	for parent != nil {
		switch parent.(type) {
		case *Placeholder:
			{
				ret = append(ret, parent.(*Placeholder))
			}
		}
		parent = parent.parent()
	}
	return ret
}

func (tms *TextmateSnippet) text() string {
	return tms._children.String()
}

// func (tms *TextmateSnippet) resolveVariables(resolver { resolve(name: string): string }): this {
// 	walk(this.children, candidate => {
// 		if (candidate instanceof Variable) {
// 			candidate.resolvedValue = resolver.resolve(candidate.name);
// 			if (candidate.isDefined) {
// 				// remove default value from resolved variable
// 				candidate.children = [];
// 			}
// 		}
// 		return true;
// 	});
// 	return this;
// }

// func (tms *TextmateSnippet) replace(marker Marker, others Markers) {
// 	parent := marker.parent()
// 	const idx = parent.children.indexOf(marker);
// 	const newChildren = parent.children.slice(0);
// 	newChildren.splice(idx, 1, ...others);
// 	parent.children = newChildren;
// 	this._placeholders = undefined;
// }

//
// Snippet parser.
//

// static escape(value: string): string {
// 	return value.replace(/\$|}|\\/g, '\\$&');
// }

// static parse(template: string, enforceFinalTabstop?: boolean): TextmateSnippet {
// 	const marker = new SnippetParser().parse(template, true, enforceFinalTabstop);
// 	return new TextmateSnippet(marker);
// }

type SnippetParser struct {
	_scanner   Scanner
	_token     *Token
	_prevToken *Token
}

func newSnippetParser() *SnippetParser {
	return &SnippetParser{
		_scanner: *newScanner(),
	}
}

func (sp *SnippetParser) text(value string) string {
	return sp.parse(value, false, false).String()
}

// func (sp *SnippetParser) parse(value string, insertFinalTabstop, enforceFinalTabstop bool) Markers {
// 	marker := []Markers{};

// 		sp._scanner.text(value);
// 		sp._token = sp._scanner.next();
// 		while (sp._parseAny(marker) || sp._parseText(marker)) {
// 			// nothing
// 		}
// }

// * fill in default for empty placeHolders
// * compact sibling Text markers
func walkDefaults(markers Markers, placeholderDefaultValues map[int]Markers) {

	for i := 0; i < len(markers); i++ {
		thisMarker := markers[i]

		switch thisMarker.(type) {
		case *Placeholder:
			{
				pl := thisMarker.(*Placeholder)
				// fill in default values for repeated placeholders
				// like `${1:foo}and$1` becomes ${1:foo}and${1:foo}
				if defaultVal, ok := placeholderDefaultValues[pl.index]; !ok {
					placeholderDefaultValues[pl.index] = pl._children
					walkDefaults(pl._children, placeholderDefaultValues)

				} else if len(pl._children) == 0 {
					// copy children from first placeholder definition, no need to
					// recurse on them because they have been visited already
					copy(pl._children, defaultVal)
				}
			}
		case *Variable:
			{
				walkDefaults(thisMarker.children(), placeholderDefaultValues)
			}
		case *Text:
			{
				if i <= 0 {
					continue
				}

				prev := markers[i-1]
				switch prev.(type) {
				case *Text:
					{
						markers[i-1].(*Text).data += markers[i].(*Text).data
						markers = append(markers[:i], markers[i+1:]...)
						i--
					}
				}
			}
		}
	}
}

func (sp *SnippetParser) parse(value string, insertFinalTabstop bool, enforceFinalTabstop bool) Markers {
	marker := Markers{}

	sp._scanner.text(value)
	sp._token = sp._scanner.next()
	for sp._parseAny(marker) || sp._parseText(marker) {
		// nothing
	}

	placeholderDefaultValues := map[int]Markers{}
	walkDefaults(marker, placeholderDefaultValues)

	_, noFinalTabstop := placeholderDefaultValues[0]
	shouldInsertFinalTabstop := insertFinalTabstop && len(placeholderDefaultValues) > 0 || enforceFinalTabstop
	if noFinalTabstop && shouldInsertFinalTabstop {
		// the snippet uses placeholders but has no
		// final tabstop defined -> insert at the end
		marker.append(newPlaceholder(0, Markers{}))
	}

	return marker
}

func (sp *SnippetParser) _accept(kind TokenType) bool {
	if sp._token.kind == kind {
		sp._prevToken = sp._token
		sp._token = sp._scanner.next()
		return true
	}
	return false
}

func (sp *SnippetParser) _acceptAny() bool {
	sp._prevToken = sp._token
	sp._token = sp._scanner.next()
	return true
}

func (sp *SnippetParser) _parseAny(markers Markers) bool {
	if sp._parseEscaped(markers) {
		return true
	} else if sp._parseTM(markers) {
		return true
	}
	return false
}

func (sp *SnippetParser) _parseText(markers Markers) bool {
	if sp._token.kind != EOF {
		markers = append(markers, newText(sp._scanner.tokenText(sp._token)))
		sp._acceptAny()
		return true
	}
	return false
}

func (sp *SnippetParser) _parseTM(marker Markers) bool {
	if sp._accept(Dollar) {
		if sp._accept(VariableName) || sp._accept(Int) {
			// $FOO, $123
			idOrName := sp._scanner.tokenText(sp._prevToken)
			if matched, err := regexp.MatchString(`^\d+$`, idOrName); matched && err != nil {
				i, _ := strconv.Atoi(idOrName)
				marker.append(newPlaceholder(i, Markers{}))
			} else {
				marker.append(newVariable(idOrName, Markers{}))
			}
			return true

		} else if sp._accept(CurlyOpen) {
			// ${name:children}
			name := Markers{}
			children := &Markers{}
			target := &name

			for {
				if target != children && sp._accept(Colon) {
					target = children
					continue
				}

				if sp._accept(CurlyClose) {
					idOrName := name.String()
					if match, err := regexp.MatchString(`^\d+$`, idOrName); match && err != nil {
						i, _ := strconv.Atoi(idOrName)
						marker.append(newPlaceholder(i, *children))
					} else {
						marker.append(newVariable(idOrName, *children))
					}
					return true
				}

				if sp._parseAny(*target) || sp._parseText(*target) {
					continue
				}

				// fallback
				if len(*children) > 0 {
					marker.append(newText("${" + name.String() + ":"))
					marker.append(*children...)
				} else {
					marker.append(newText("${"))
					marker.append(name...)
				}
				return true
			}
		}

		marker.append(newText("$"))
		return true
	}
	return false
}

func (sp *SnippetParser) _parseEscaped(marker Markers) bool {
	if sp._accept(Backslash) {
		if sp._accept(Dollar) || sp._accept(CurlyClose) || sp._accept(Backslash) {
			// just consume them
		}
		marker.append(newText(sp._scanner.tokenText(sp._prevToken)))
		return true
	}
	return false
}
