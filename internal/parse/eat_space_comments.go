package parse

func (p *parser) eatComment() bool {
	p.panicIfContextDone()

	start := p.i

	if p.i < p.len-1 && isSpaceNotLF(p.s[p.i+1]) {
		p.i += 2
		for p.i < p.len && p.s[p.i] != '\n' {
			p.i++
		}
		p.tokens = append(p.tokens, Token{Type: COMMENT, Span: NodeSpan{start, p.i}, Raw: string(p.s[start:p.i])})
		return true
	} else {
		return false
	}
}

func (p *parser) eatSpace() {
	p.panicIfContextDone()

	for p.i < p.len && isSpaceNotLF(p.s[p.i]) {
		p.i++
	}
}

func (p *parser) eatSpaceNewline() {
	p.panicIfContextDone()

loop:
	for p.i < p.len {
		switch p.s[p.i] {
		case ' ', '\t', '\r':
		case '\n':
			p.tokens = append(p.tokens, Token{Type: NEWLINE, Span: NodeSpan{p.i, p.i + 1}})
		default:
			break loop
		}
		p.i++
	}

}

func (p *parser) eatSpaceComments() {
	p.panicIfContextDone()

loop:
	for p.i < p.len {
		switch p.s[p.i] {
		case ' ', '\t', '\r':
		case '#':
			if !p.eatComment() {
				return
			}
			continue
		default:
			break loop
		}
		p.i++
	}

}

func (p *parser) eatSpaceNewlineComment() {
	p.panicIfContextDone()

loop:
	for p.i < p.len {
		switch p.s[p.i] {
		case ' ', '\t', '\r':
		case '\n':
			p.tokens = append(p.tokens, Token{Type: NEWLINE, Span: NodeSpan{p.i, p.i + 1}})
		case '#':
			if !p.eatComment() {
				return
			}
			continue
		default:
			break loop
		}
		p.i++
	}

}

func (p *parser) eatSpaceNewlineCommaComment() {
	p.panicIfContextDone()

loop:
	for p.i < p.len {
		switch p.s[p.i] {
		case ' ', '\t', '\r':
		case '\n':
			p.tokens = append(p.tokens, Token{Type: NEWLINE, Span: NodeSpan{p.i, p.i + 1}})
		case ',':
			p.tokens = append(p.tokens, Token{Type: COMMA, Span: NodeSpan{p.i, p.i + 1}})
		case '#':
			if !p.eatComment() {
				return
			}
			continue
		default:
			break loop
		}
		p.i++
	}
}

func (p *parser) eatSpaceNewlineSemicolonComment() {
	p.panicIfContextDone()

loop:
	for p.i < p.len {
		switch p.s[p.i] {
		case ' ', '\t', '\r':
		case '\n':
			p.tokens = append(p.tokens, Token{Type: NEWLINE, Span: NodeSpan{p.i, p.i + 1}})
		case ';':
			p.tokens = append(p.tokens, Token{Type: SEMICOLON, Span: NodeSpan{p.i, p.i + 1}})
		case '#':
			if !p.eatComment() {
				return
			}
			continue
		default:
			break loop
		}
		p.i++
	}

}

func (p *parser) eatSpaceNewlineComma() {
	p.panicIfContextDone()

loop:
	for p.i < p.len {
		switch p.s[p.i] {
		case ' ', '\t', '\r':
		case '\n':
			p.tokens = append(p.tokens, Token{Type: NEWLINE, Span: NodeSpan{p.i, p.i + 1}})
		case ',':
			p.tokens = append(p.tokens, Token{Type: COMMA, Span: NodeSpan{p.i, p.i + 1}})
		default:
			break loop
		}
		p.i++
	}
}

func (p *parser) eatSpaceComma() {
	p.panicIfContextDone()

loop:
	for p.i < p.len {
		switch p.s[p.i] {
		case ' ', '\t', '\r':
		case ',':
			p.tokens = append(p.tokens, Token{Type: COMMA, Span: NodeSpan{p.i, p.i + 1}})
		default:
			break loop
		}
		p.i++
	}

}
