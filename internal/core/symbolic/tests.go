package symbolic

import (
	"errors"
	"fmt"

	parse "github.com/inoxlang/inox/internal/parse"
)

func _makeStateAndChunk(code string, includedFiles map[string]string, globals ...map[string]SymbolicValue) (*parse.Chunk, *State, error) {
	chunk, err := parse.ParseChunkSource(parse.InMemorySource{
		NameString: "",
		CodeString: code,
	})

	state := newSymbolicState(NewSymbolicContext(nil), chunk)
	state.symbolicData = NewSymbolicData()

	state.ctx.AddNamedPattern("int", &TypePattern{
		val: &Int{},
		call: func(ctx *Context, values []SymbolicValue) (Pattern, error) {
			if len(values) == 0 {
				return nil, errors.New("missing argument")
			}
			return &IntRangePattern{}, nil
		},
	}, false)
	state.ctx.AddNamedPattern("str", &TypePattern{val: ANY_STR_LIKE}, false)
	state.ctx.AddNamedPattern("object", &TypePattern{val: NewAnyObject()}, false)
	state.ctx.AddNamedPattern("list", &TypePattern{val: NewListOf(ANY_SERIALIZABLE)}, false)
	state.ctx.AddPatternNamespace("myns", NewPatternNamespace(map[string]Pattern{
		"int": state.ctx.ResolveNamedPattern("int"),
	}), false)
	state.Module = &Module{
		MainChunk: chunk,
	}

	if len(globals) > 1 {
		for k, v := range globals[0] {
			state.setGlobal(k, v, GlobalConst)
		}
	}

	if len(includedFiles) > 0 {
		state.Module.InclusionStatementMap = make(map[*parse.InclusionImportStatement]*IncludedChunk, len(includedFiles))
	}
	for file, content := range includedFiles {
		importStmt := parse.FindNode(chunk.Node, (*parse.InclusionImportStatement)(nil), func(stmt *parse.InclusionImportStatement, _ bool) bool {
			pathLit, ok := stmt.Source.(*parse.RelativePathLiteral)
			return ok && pathLit.Value == file
		})

		includedChunk, err := parse.ParseChunkSource(parse.InMemorySource{
			NameString: file,
			CodeString: content,
		})

		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse included chunk %s", file)
		}

		state.Module.InclusionStatementMap[importStmt] = &IncludedChunk{ParsedChunk: includedChunk}
	}

	return chunk.Node, state, err
}

func MakeTestStateAndChunk(code string, globals ...map[string]SymbolicValue) (*parse.Chunk, *State) {
	node, state, err := _makeStateAndChunk(code, nil, globals...)
	if err != nil {
		panic(err)
	}
	return node, state
}

func MakeTestStateAndChunks(code string, files map[string]string, globals ...map[string]SymbolicValue) (*parse.Chunk, *State) {
	node, state, err := _makeStateAndChunk(code, files, globals...)
	if err != nil {
		panic(err)
	}
	return node, state
}
