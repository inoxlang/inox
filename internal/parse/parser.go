package parse

import (
	"context"
	"time"
)

// A parser parses a single Inox chunk, it can recover from errors.
// Note that there is no lexer.
type parser struct {
	s              []rune //chunk's code
	i              int32  //rune index
	len            int32
	inPattern      bool
	onlyChunkStart bool

	//mostly valueless tokens, the slice may be not perfectly ordered.
	tokens []Token

	noCheckFuel          int //-1 if infinite fuel
	remainingNoCheckFuel int //refueled after each context check.

	context context.Context
	cancel  context.CancelFunc

	//If not set defaults to the registered parseHyperscript function.
	parseHyperscript ParseHyperscriptFn
}

type ParserOptions struct {
	//If nil the parent context is set to context.Background().
	//The parser internally creates a child context with a timeout.
	ParentContext context.Context

	//The internal context is checked each time the 'no check fuel' is empty.
	//The 'no check fuel' defaults to DEFAULT_NO_CHECK_FUEL if NoCheckFuel is <= 0 or if context is nil.
	NoCheckFuel int

	//Defaults to DEFAULT_TIMEOUT.
	Timeout time.Duration

	//If not set defaults to the function registered by RegisterParseHypercript.
	ParseHyperscript ParseHyperscriptFn

	//Makes the parser stops after the following node type:
	// - IncludableChunkDescription if no constants are defined.
	// - GlobalVariableDeclarations if there is no IncludableChunkDescription nor Manifest.
	// - Manifest
	Start bool
}

func newParser(s []rune, opts ...ParserOptions) *parser {
	p := &parser{
		s:                    s,
		i:                    0,
		len:                  int32(len(s)),
		noCheckFuel:          -1,
		remainingNoCheckFuel: -1,
		tokens:               make([]Token, 0, len(s)/10),
		parseHyperscript:     parseHyperscript,
	}

	var (
		timeout     time.Duration   = DEFAULT_TIMEOUT
		noCheckFuel                 = DEFAULT_NO_CHECK_FUEL
		ctx         context.Context = context.Background()
	)

	if len(opts) > 0 {
		opt := opts[0]

		if opt.ParentContext != nil {
			ctx = opt.ParentContext
		}

		if opt.NoCheckFuel > 0 {
			noCheckFuel = opt.NoCheckFuel
		}

		if opt.Timeout > 0 {
			timeout = opt.Timeout
		}

		if opt.Start {
			p.onlyChunkStart = true
		}

		if opt.ParseHyperscript != nil {
			p.parseHyperscript = opt.ParseHyperscript
		}
	}

	p.context, p.cancel = context.WithTimeout(ctx, timeout)
	p.noCheckFuel = noCheckFuel
	p.remainingNoCheckFuel = noCheckFuel

	return p
}

// panicIfContextDone checks wheter he context
func (p *parser) panicIfContextDone() {
	if p.noCheckFuel == -1 {
		return
	}

	p.remainingNoCheckFuel--

	if p.remainingNoCheckFuel == 0 {
		p.remainingNoCheckFuel = p.noCheckFuel
		if p.context != nil {
			select {
			case <-p.context.Done():
				panic(p.context.Err())
			default:
				break
			}
		}
	}
}
