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
}

type ParserOptions struct {
	//The context is checked each time the 'no check fuel' is empty.
	//The 'no check fuel' defauls to DEFAULT_NO_CHECK_FUEL if NoCheckFuel is <= 0 or if context is nil.
	NoCheckFuel int

	//This option is ignored if noCheckFuel is <= 0.
	//The default context context.Background().
	Context context.Context

	//Defaults to DEFAULT_TIMEOUT.
	Timeout time.Duration

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
	}

	var (
		timeout     time.Duration   = DEFAULT_TIMEOUT
		noCheckFuel                 = DEFAULT_NO_CHECK_FUEL
		ctx         context.Context = context.Background()
	)

	if len(opts) > 0 {
		opt := opts[0]
		if opt.Context != nil && opt.NoCheckFuel > 0 {
			if opt.Timeout > 0 {
				timeout = opt.Timeout
			}
			ctx = opt.Context
		}
		if opt.Start {
			p.onlyChunkStart = true
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
