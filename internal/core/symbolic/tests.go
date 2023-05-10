package internal

import (
	"errors"

	parse "github.com/inoxlang/inox/internal/parse"
)

func _makeStateAndChunk(code string, globals ...map[string]SymbolicValue) (*parse.Chunk, *State, error) {
	chunk, err := parse.ParseChunkSource(parse.InMemorySource{
		NameString: "",
		CodeString: code,
	})

	state := newSymbolicState(NewSymbolicContext(), chunk)
	state.symbolicData = NewSymbolicData()

	state.ctx.AddNamedPattern("int", &TypePattern{
		val: &Int{},
		call: func(ctx *Context, values []SymbolicValue) (Pattern, error) {
			if len(values) == 0 {
				return nil, errors.New("missing argument")
			}
			return &IntRangePattern{}, nil
		},
	})
	state.ctx.AddNamedPattern("str", &TypePattern{val: ANY_STR_LIKE})
	state.ctx.AddNamedPattern("obj", &TypePattern{val: NewAnyObject()})
	state.ctx.AddNamedPattern("list", &TypePattern{val: NewListOf(ANY)})
	state.ctx.AddPatternNamespace("myns", NewPatternNamespace(map[string]Pattern{
		"int": state.ctx.ResolveNamedPattern("int"),
	}))
	state.Module = &Module{
		MainChunk: chunk,
	}

	if len(globals) > 1 {
		for k, v := range globals[0] {
			state.setGlobal(k, v, GlobalConst)
		}
	}

	return chunk.Node, state, err
}

func MakeTestStateAndChunk(code string, globals ...map[string]SymbolicValue) (*parse.Chunk, *State) {
	node, state, err := _makeStateAndChunk(code)
	if err != nil {
		panic(err)
	}
	return node, state
}
