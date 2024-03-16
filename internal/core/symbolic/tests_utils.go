package symbolic

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/parse"
)

func _makeStateAndChunk(code string, globals ...map[string]Value) (*parse.Chunk, *State, error) {
	chunk, err := parse.ParseChunkSource(parse.InMemorySource{
		NameString: "",
		CodeString: code,
	})

	state := newSymbolicState(NewSymbolicContext(nil, nil, nil), chunk)
	state.symbolicData = NewSymbolicData()
	state.setGlobal("int", ANY_INT, GlobalConst)

	//this pattern is added for testing purposes only
	state.ctx.AddNamedPattern("never", &TypePattern{
		val: NEVER,
	}, false)

	state.ctx.AddNamedPattern("int", &TypePattern{
		val: ANY_INT,
		call: func(ctx *Context, values []Value) (Pattern, error) {
			if len(values) == 0 {
				return nil, errors.New("missing argument")
			}
			intRange, ok := values[0].(*IntRange)

			if !ok {
				return nil, errors.New("argument should be an integer range")
			}
			return NewIntRangePattern(intRange), nil
		},
	}, false)
	state.ctx.AddNamedPattern("bool", &TypePattern{val: ANY_BOOL}, false)
	state.ctx.AddNamedPattern("nil", &TypePattern{val: Nil}, false)
	state.ctx.AddNamedPattern("str", &TypePattern{val: ANY_STR_LIKE}, false)
	state.ctx.AddNamedPattern("object", &TypePattern{val: NewAnyObject()}, false)
	state.ctx.AddNamedPattern("list", &TypePattern{val: NewListOf(ANY_SERIALIZABLE)}, false)
	state.ctx.AddPatternNamespace("myns", NewPatternNamespace(map[string]Pattern{
		"int": state.ctx.ResolveNamedPattern("int"),
	}), false)
	state.Module = &Module{
		mainChunk: chunk,
	}

	if len(globals) >= 1 {
		for k, v := range globals[0] {
			state.setGlobal(k, v, GlobalConst)
		}
	}

	return chunk.Node, state, err
}

func MakeTestStateAndChunk(code string, globals ...map[string]Value) (*parse.Chunk, *State) {
	node, state, err := _makeStateAndChunk(code, globals...)
	if err != nil {
		panic(err)
	}

	return node, state
}

func MakeTestStateAndChunks(code string, includedFiles map[string]string, globals ...map[string]Value) (*parse.Chunk, *State) {
	node, state, err := _makeStateAndChunk(code, globals...)
	if err != nil {
		panic(err)
	}

	state.Module.inclusionStatementMap = make(map[*parse.InclusionImportStatement]*IncludedChunk, len(includedFiles))

	for file, content := range includedFiles {
		importStmt := parse.FindNode(state.Module.mainChunk.Node, (*parse.InclusionImportStatement)(nil),
			func(stmt *parse.InclusionImportStatement, _ bool, _ []parse.Node) bool {
				pathLit, ok := stmt.Source.(*parse.RelativePathLiteral)
				return ok && pathLit.Value == file
			},
		)

		if importStmt == nil {
			panic(fmt.Errorf("import statement with source %s not found", file))
		}

		includedChunk, err := parse.ParseChunkSource(parse.InMemorySource{
			NameString: file,
			CodeString: content,
		})

		if err != nil {
			panic(fmt.Errorf("failed to parse included chunk %s", file))
		}

		state.Module.inclusionStatementMap[importStmt] = &IncludedChunk{ParsedChunkSource: includedChunk}
	}

	return node, state
}

func MakeTestStateAndImportedModules(code string, files map[string]string, globals ...map[string]Value) (*parse.Chunk, *State) {
	node, state, err := _makeStateAndChunk(code, globals...)
	if err != nil {
		panic(err)
	}

	state.Module.directlyImportedModules = map[*parse.ImportStatement]*Module{}

	for file, content := range files {
		importStmt := parse.FindNode(state.Module.mainChunk.Node, (*parse.ImportStatement)(nil), nil)

		importedModuleChunk, err := parse.ParseChunkSource(parse.InMemorySource{
			NameString: file,
			CodeString: content,
		})

		if err != nil {
			panic(fmt.Errorf("failed to parse imported module %s", file))
		}

		state.Module.directlyImportedModules[importStmt] = &Module{
			mainChunk: importedModuleChunk,
		}
	}

	return node, state
}
