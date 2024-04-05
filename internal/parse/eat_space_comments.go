package parse

func (p *parser) eatComment() bool {
	p.panicIfContextDone()

	start := p.i

	if p.i < p.len-1 && p.s[p.i] == '#' && IsCommentFirstSpace(p.s[p.i+1]) {
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

func (p *parser) eatSpace() (count int) {
	p.panicIfContextDone()

	for p.i < p.len && isSpaceNotLF(p.s[p.i]) {
		count++
		p.i++
	}
	return
}

func (p *parser) eatSpaceNewline() (linefeedCount int) {
	p.panicIfContextDone()

loop:
	for p.i < p.len {
		switch p.s[p.i] {
		case ' ', '\t', '\r':
		case '\n':
			p.tokens = append(p.tokens, Token{Type: NEWLINE, Span: NodeSpan{p.i, p.i + 1}})
			linefeedCount++
		default:
			break loop
		}
		p.i++
	}
	return
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

func (p *parser) areNextSpacesNewlinesCommentsFollowedBy(r rune) bool {
	p.panicIfContextDone()

	index := p.i
loop:
	for index < p.len {
		switch p.s[index] {
		case ' ', '\t', '\r', '\n':
		case '#':
			if index == p.len-1 || !IsCommentFirstSpace(p.s[index]) {
				//Not a comment
				return false
			}

			for index < p.len && p.s[index] != '\n' {
				index++
			}

			continue
		default:
			break loop
		}
		index++
	}

	return index < p.len && p.s[index] == r
}

func (p *parser) areNextSpacesFollowedBy(r rune) bool {
	p.panicIfContextDone()

	index := p.i
loop:
	for index < p.len {
		switch p.s[index] {
		case ' ', '\t', '\r':
		default:
			break loop
		}
		index++
	}

	return index < p.len && p.s[index] == r
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
