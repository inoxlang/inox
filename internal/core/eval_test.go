package core

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"

	"github.com/stretchr/testify/assert"
)

const (
	RETURN_1_MODULE_HASH               = "SG2a/7YNuwBjsD2OI6bM9jZM4gPcOp9W8g51DrQeyt4="
	RETURN_NON_POS_ARG_A_MODULE_HASH   = "15Njs+OhmiW9843cgnlMib7AiUzZbGx6gn3GAebWMOA="
	RETURN_POS_ARG_A_MODULE_HASH       = "QNJpkgQeB5MA23yXpJ8L5XWLzUQIi6eDwi2HOnPTO3w="
	RETURN_PATTERN_INT_TWO_MODULE_HASH = "D9SSw63q6VesJ6tTYZZ1EJzyAW5L3FCTPxQjWfOi8F4="
	RETURN_INT_PATTERN_MODULE_HASH     = "Ub9ua2QldCOc6MvxIPVpUYOQQfQoZpYEoDJitOdKFPA="
)

func init() {
	moduleCache[RETURN_1_MODULE_HASH] = "return 1"
	moduleCache[RETURN_NON_POS_ARG_A_MODULE_HASH] = "manifest {parameters: {a: %int}}\nreturn mod-args.a"
	moduleCache[RETURN_POS_ARG_A_MODULE_HASH] = "manifest {parameters: {{name: #a, pattern: %int}}}\nreturn mod-args.a"
	moduleCache[RETURN_PATTERN_INT_TWO_MODULE_HASH] = "manifest {}\n%two = 2; return %two"
	moduleCache[RETURN_INT_PATTERN_MODULE_HASH] = "manifest {}; return %int"
}

func TestTreeWalkEval(t *testing.T) {

	testEval(t, false, func(c any, s *GlobalState, doSymbolicCheck bool) (Value, error) {
		var mod *Module

		switch val := c.(type) {
		case *Module:
			mod = val
		case string:
			chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
				NameString: "core-test",
				CodeString: val,
			}))

			mod = &Module{MainChunk: chunk}

			//if the test case provide a module we reuse the source
			if s.Module != nil {
				chunk.Source = s.Module.MainChunk.Source
				s.Module.MainChunk = chunk
				mod = s.Module
			} else {
				s.Module = mod
			}
		default:
			t.Fatalf("%#v is not a valid code argument", c)
		}

		if doSymbolicCheck {
			staticCheckData, err := StaticCheck(StaticCheckInput{
				Node:              mod.MainChunk.Node,
				Module:            mod,
				Chunk:             mod.MainChunk,
				Globals:           s.Globals,
				Patterns:          s.Ctx.namedPatterns,
				PatternNamespaces: s.Ctx.patternNamespaces,
			})
			if !assert.NoError(t, err) {
				return nil, err
			}

			s.StaticCheckData = staticCheckData

			globals := make(map[string]symbolic.ConcreteGlobalValue)
			s.Globals.Foreach(func(name string, v Value, isConstant bool) error {
				globals[name] = symbolic.ConcreteGlobalValue{Value: v, IsConstant: isConstant}
				return nil
			})

			symbCtx, err := s.Ctx.ToSymbolicValue()
			if !assert.NoError(t, err) {
				return nil, err
			}

			symbData, err := symbolic.SymbolicEvalCheck(symbolic.SymbolicEvalCheckInput{
				Node:    mod.MainChunk.Node,
				Module:  mod.ToSymbolic(),
				Globals: globals,
				Context: symbCtx,
			})

			if !assert.NoError(t, err) {
				return nil, err
			}
			s.SymbolicData.AddData(symbData)
		}

		treeWalkState := NewTreeWalkStateWithGlobal(s)
		return TreeWalkEval(mod.MainChunk.Node, treeWalkState)
	})

}

func TestBytecodeEval(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	bytecodeTest(t, false)
}

func TestOptimizedBytecodeEval(t *testing.T) {
	bytecodeTest(t, true)
}

func bytecodeTest(t *testing.T, optimize bool) {
	testEval(t, true, func(c any, s *GlobalState, doCheck bool) (Value, error) {
		var mod *Module

		switch val := c.(type) {
		case *Module:
			mod = val
			s.Module = mod
		case string:
			chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
				NameString: "core-test",
				CodeString: val,
			}))

			mod = &Module{MainChunk: chunk}

			//if the test case provide a module we reuse the source
			if s.Module != nil {
				chunk.Source = s.Module.MainChunk.Source
				s.Module.MainChunk = chunk
				mod = s.Module
			} else {
				s.Module = mod
			}
		default:
			t.Fatalf("%#v is not a valid code argument", c)
		}

		tracer := bytes.Buffer{}

		if doCheck { // TODO: enable checks by default ?
			staticCheckData, err := StaticCheck(StaticCheckInput{
				Node:              mod.MainChunk.Node,
				Module:            mod,
				Chunk:             mod.MainChunk,
				Globals:           s.Globals,
				Patterns:          s.Ctx.namedPatterns,
				PatternNamespaces: s.Ctx.patternNamespaces,
			})
			if !assert.NoError(t, err) {
				return nil, err
			}

			s.StaticCheckData = staticCheckData

			globals := make(map[string]symbolic.ConcreteGlobalValue)
			s.Globals.Foreach(func(name string, v Value, isConstant bool) error {
				globals[name] = symbolic.ConcreteGlobalValue{Value: v, IsConstant: isConstant}
				return nil
			})

			symbCtx, err := s.Ctx.ToSymbolicValue()
			if !assert.NoError(t, err) {
				return nil, err
			}

			symbData, err := symbolic.SymbolicEvalCheck(symbolic.SymbolicEvalCheckInput{
				Node:    mod.MainChunk.Node,
				Module:  mod.ToSymbolic(),
				Globals: globals,
				Context: symbCtx,
			})

			if !assert.NoError(t, err) {
				return nil, err
			}
			s.SymbolicData.AddData(symbData)
		}

		compilationCtx := NewContext(ContextConfig{})
		NewGlobalState(compilationCtx)

		res, err := EvalVM(mod, s, BytecodeEvaluationConfig{
			Tracer:             &tracer,
			OptimizeBytecode:   optimize,
			CompilationContext: compilationCtx,
		})

		if err != nil {
			//t.Log(tracer.String())
			return nil, err
		}

		return res, nil
	})
}

// testEval executes the suite of evaluation tests with a given evaluation function
// that can have any implementation (tree walk, bytecode, ...).
func testEval(t *testing.T, bytecodeEval bool, Eval evalFn) {

	t.Run("integer literal", func(t *testing.T) {
		code := "1"
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)
		assert.Equal(t, Int(1), res)
	})

	t.Run("port literal", func(t *testing.T) {
		code := ":80/http"
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)
		assert.Equal(t, Port{
			Number: 80,
			Scheme: "http",
		}, res)
	})

	t.Run("quoted string literal", func(t *testing.T) {
		code := `"a"`
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)
		assert.Equal(t, Str("a"), res)
	})

	t.Run("multiline string literal", func(t *testing.T) {
		t.Run("single character", func(t *testing.T) {
			code := "`a`"
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
			assert.NoError(t, err)
			assert.Equal(t, Str("a"), res)
		})

		t.Run("linefeed", func(t *testing.T) {
			code := "`1\n2`"
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
			assert.NoError(t, err)
			assert.Equal(t, Str("1\n2"), res)
		})
		t.Run("escaped n (\\n)", func(t *testing.T) {
			code := "`1\\n2`"
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
			assert.NoError(t, err)
			assert.Equal(t, Str("1\n2"), res)
		})
	})

	t.Run("byte slice literal", func(t *testing.T) {
		code := `0x[01]`
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)
		assert.Equal(t, &ByteSlice{Bytes: []byte{1}, IsDataMutable: true}, res)
	})

	t.Run("property name literal", func(t *testing.T) {
		code := `.a`
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)
		assert.Equal(t, PropertyName("a"), res)
	})

	t.Run("boolean literal", func(t *testing.T) {
		code := `true`
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)
		assert.Equal(t, True, res)
	})

	t.Run("nil literal", func(t *testing.T) {
		code := `nil`
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)
		assert.Equal(t, Nil, res)
	})

	t.Run("absolute path literal", func(t *testing.T) {
		code := `/`
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)
		assert.Equal(t, Path("/"), res)
	})

	t.Run("relative path literal", func(t *testing.T) {
		code := `./`
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)
		assert.Equal(t, Path("./"), res)
	})

	t.Run("absolute path pattern literal", func(t *testing.T) {
		code := `%/*`
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)
		assert.Equal(t, PathPattern("/*"), res)
	})

	t.Run("relative path pattern literal", func(t *testing.T) {
		code := `%./*`
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)
		assert.Equal(t, PathPattern("./*"), res)
	})

	t.Run("named-segment path pattern literal", func(t *testing.T) {
		code := `%/home/{:username}`
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)
		assert.IsType(t, &NamedSegmentPathPattern{}, res)
	})

	t.Run("path expression", func(t *testing.T) {

		t.Run("absolute", func(t *testing.T) {
			t.Run("interpolation value is a string", func(t *testing.T) {
				code := `/home/{username}`
				res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
					"username": Str("foo"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, Path("/home/foo"), res)
			})

			t.Run("interpolation value is a string containing '/'", func(t *testing.T) {
				code := `/home/{username}`
				res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
					"username": Str("fo/o"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, Path("/home/fo/o"), res)
			})

			t.Run("interpolation value is a path containing '?'", func(t *testing.T) {
				code := `/home/{username}`
				_, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
					"username": Str("./a?x=1"),
				}), false)
				assert.Error(t, err)
			})

			t.Run("interpolation value is an absolute path", func(t *testing.T) {
				code := `/home/{path}`
				res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
					"path": Path("/foo"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, Path("/home/foo"), res)
			})

			t.Run("interpolation value is a relative path", func(t *testing.T) {
				code := `/home/{path}`
				res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
					"path": Path("./foo"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, Path("/home/foo"), res)
			})

		})

		t.Run("relative", func(t *testing.T) {

			t.Run("interpolation value is a string", func(t *testing.T) {
				code := `./home/{username}`
				res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
					"username": Str("foo"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, Path("./home/foo"), res)
			})

			t.Run("interpolation value is a string containing '/'", func(t *testing.T) {
				code := `./home/{username}`
				res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
					"username": Str("fo/o"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, Path("./home/fo/o"), res)
			})

			t.Run("interpolation value is a path containing '?'", func(t *testing.T) {
				code := `./home/{username}`
				_, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
					"username": Str("./a?x=1"),
				}), false)
				assert.Error(t, err)
			})

			t.Run("interpolation value is an absolute path", func(t *testing.T) {
				code := `./home/{path}`
				res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
					"path": Path("/foo"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, Path("./home/foo"), res)
			})

			t.Run("interpolation value is a relative path", func(t *testing.T) {
				code := `./home/{path}`
				res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
					"path": Path("./foo"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, Path("./home/foo"), res)
			})

		})

		injectionCases := []struct {
			input string
			error string
		}{
			//path
			{
				`path = "."; return /.{path}`,
				S_PATH_EXPR_PATH_LIMITATION,
			},
			{
				`path = ""; return /.{path}.`,
				S_PATH_EXPR_PATH_LIMITATION,
			},
			{
				`path = "?a=b"; return /{path}`,
				S_PATH_INTERP_RESULT_LIMITATION,
			},

			//TODO: add more tests
		}

		for _, testCase := range injectionCases {
			t.Run(testCase.input, func(t *testing.T) {
				res, err := Eval(testCase.input, NewGlobalState(NewDefaultTestContext(), nil), false)
				assert.ErrorContains(t, err, testCase.error)
				assert.Nil(t, res)
			})
		}

	})

	t.Run("path pattern expression", func(t *testing.T) {
		t.Run("path pattern expression", func(t *testing.T) {
			code := `%/home/{username}/...`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"username": Str("foo"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, PathPattern("/home/foo/..."), res)
		})

		t.Run("globbing injection", func(t *testing.T) {
			code := `%/home/{username}/...`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"username": Str("*"),
			}), false)
			assert.Error(t, err)
			assert.Nil(t, res)
		})

	})

	t.Run("HTTP scheme", func(t *testing.T) {
		code := `http://`
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)
		assert.Equal(t, Scheme("http"), res)
	})

	t.Run("HTTP host", func(t *testing.T) {
		code := `https://example.com`
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)
		assert.Equal(t, Host("https://example.com"), res)
	})

	t.Run("HTTP host pattern", func(t *testing.T) {
		code := `%https://**.example.com`
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)
		assert.Equal(t, HostPattern("https://**.example.com"), res)
	})

	t.Run("URL expression", func(t *testing.T) {
		t.Run("host interpolation", func(t *testing.T) {
			code := `https://{host}/`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"host": Str("localhost"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, URL("https://localhost/"), res)
		})

		t.Run("single path interpolation : interpolation does not contain '/'", func(t *testing.T) {
			code := `https://example.com/{path}`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"path": Str("index.html"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, URL("https://example.com/index.html"), res)
		})

		t.Run("single path interpolation : interpolation starts with '/'", func(t *testing.T) {
			code := `https://example.com/{path}`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"path": Str("/index.html"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, URL("https://example.com//index.html"), res)
		})

		t.Run("single path interpolation, no '/' in path slice", func(t *testing.T) {
			code := `https://example.com{path}`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"path": Str("index.html"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, URL("https://example.com/index.html"), res)
		})

		t.Run("path interpolation containg an encoded '?'", func(t *testing.T) {
			code := `https://example.com{path}`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"path": Str("%3F"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, URL("https://example.com/%3F"), res)
		})

		t.Run("path interpolation containg an encoded '#'", func(t *testing.T) {
			code := `https://example.com{path}`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"path": Str("%23"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, URL("https://example.com/%23"), res)
		})

		t.Run("path interpolation starting with a '@'", func(t *testing.T) {
			code := `https://example.com{path}`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"path": Str("@domain.zip"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, URL("https://example.com/@domain.zip"), res)
		})

		t.Run("host alias", func(t *testing.T) {
			code := `@api/index.html`
			ctx, _ := NewDefaultTestContext().NewWith(nil)
			ctx.AddHostAlias("api", Host("https://example.com"))
			res, err := Eval(code, NewGlobalState(ctx), false)

			assert.NoError(t, err)
			assert.Equal(t, URL("https://example.com/index.html"), res)
		})

		t.Run("query with no interpolation", func(t *testing.T) {
			code := `return https://example.com/?v=a`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), nil), false)
			assert.NoError(t, err)
			assert.Equal(t, URL("https://example.com/?v=a"), res)
		})

		t.Run("single query interpolation", func(t *testing.T) {
			code := `
				x = "a"
				return https://example.com/?v={$x}
			`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), nil), false)
			assert.NoError(t, err)
			assert.Equal(t, URL("https://example.com/?v=a"), res)
		})

		t.Run("two query interpolations", func(t *testing.T) {
			code := `
				x = "a"
				y = "b"
				return https://example.com/?v={$x}&w={$y}
			`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), nil), false)
			assert.NoError(t, err)
			assert.Equal(t, URL("https://example.com/?v=a&w=b"), res)
		})

		t.Run("query interpolation containing an encoded '#'", func(t *testing.T) {
			code := `
				x = "%23"
				return https://example.com/?v={$x}
			`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), nil), false)
			assert.NoError(t, err)
			assert.Equal(t, URL("https://example.com/?v=%23"), res)
		})

		t.Run("query interpolation with an integer value", func(t *testing.T) {
			code := `
				x = 1
				return https://example.com/?v={$x}
			`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), nil), false)
			assert.NoError(t, err)
			assert.Equal(t, URL("https://example.com/?v=1"), res)
		})

		t.Run("query interpolation with a boolean value", func(t *testing.T) {
			code := `
				x = true
				return https://example.com/?v={$x}
			`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext(), nil), false)
			assert.NoError(t, err)
			assert.Equal(t, URL("https://example.com/?v=true"), res)
		})

		injectionCases := []struct {
			input string
			error string
		}{
			//note: %2E is the URL encoding for '.'
			//port injection in path
			{
				`path = ":8080"; return https://example.com{path}`,
				S_URL_EXPR_PATH_START_LIMITATION,
			},

			//'..' injection in path
			{
				`path = "."; return https://example.com/.{path}`,
				S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "."; return https://example.com/%2E{path}`,
				S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "%2E"; return https://example.com/.{path}`,
				S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "2E"; return https://example.com/.%{path}`,
				S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "E"; return https://example.com/.%2{path}`,
				S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "%2E"; return https://example.com/%2E{path}`,
				S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "2E"; return https://example.com/%2E%{path}`,
				S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "E"; return https://example.com/%2E%2{path}`,
				S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = ""; return https://example.com/.{path}.`,
				S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = ""; return https://example.com/%2E{path}.`,
				S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = ""; return https://example.com/.{path}%2E`,
				S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = ""; return https://example.com/%2E{path}%2E`,
				S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = /.; return https://example.com{path}.`,
				S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = /.; return https://example.com{path}%2E`,
				S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = /%2E; return https://example.com{path}.`,
				S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = /%2E; return https://example.com{path}%2E`,
				S_URL_EXPR_PATH_LIMITATION,
			},

			//'?' injection in path
			{
				`path = "?a=b"; return https://example.com{path}`,
				S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			{
				`path = "x?a=b"; return https://example.com{path}`,
				S_URL_PATH_INTERP_RESULT_LIMITATION,
			},

			//'#' injection in path
			{
				`path = "#"; return https://example.com{path}`,
				S_URL_PATH_INTERP_RESULT_LIMITATION,
			},

			//'*' injection in path
			{
				`path = "*"; return https://example.com{path}`,
				S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			{
				`path = "/*"; return https://example.com{path}`,
				S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			{
				`path = "%2A"; return https://example.com{path}`,
				S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			{
				`path = "/%2A"; return https://example.com{path}`,
				S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			//TODO: add more tests

			//'#' injection in query
			{
				`x = "#id"; return https://example.com/?v={$x}`,
				S_QUERY_PARAM_VALUE_LIMITATION,
			},
			//'&' injection in query
			{
				`x = "x&admin=true"; return https://example.com/?v={$x}`,
				S_QUERY_PARAM_VALUE_LIMITATION,
			},

			//TODO: add more tests
		}

		for _, testCase := range injectionCases {
			t.Run(testCase.input, func(t *testing.T) {
				res, err := Eval(testCase.input, NewGlobalState(NewDefaultTestContext(), nil), false)
				assert.ErrorContains(t, err, testCase.error)
				assert.Nil(t, res)
			})
		}

	})

	t.Run("integer binary expression", func(t *testing.T) {
		testCases := []struct {
			code   string
			result Value
			err    error
		}{
			//addition
			{"(1 + 2)", Int(3), nil},
			{"(1 + -2)", Int(-1), nil},
			{"(9223372036854775807 + -1)", Int(9223372036854775806), nil},
			{"(-9223372036854775808 + 1)", Int(-9223372036854775807), nil},
			{"(9223372036854775807 + 1)", nil, ErrIntOverflow},
			{"(-9223372036854775808 + -1)", nil, ErrIntUnderflow},
			//substraction
			{"(1 - 2)", Int(-1), nil},
			{"(1 - -2)", Int(3), nil},
			{"(9223372036854775807 - 1)", Int(9223372036854775806), nil},
			{"(-9223372036854775808 - -1)", Int(-9223372036854775807), nil},
			{"(9223372036854775807 - -1)", nil, ErrIntOverflow},
			{"(-9223372036854775808 - 1)", nil, ErrIntUnderflow},
			//multiplication
			{"(1 * 2)", Int(2), nil},
			{"(1 * -2)", Int(-2), nil},
			{"(9223372036854775807 * -1)", -Int(9223372036854775807), nil},
			{"(9223372036854775807 * -2)", nil, ErrIntUnderflow},
			{"(9223372036854775807 * 2)", nil, ErrIntOverflow},
			{"(-9223372036854775808 * -1)", nil, ErrIntOverflow},
			{"(-9223372036854775808 * -2)", nil, ErrIntUnderflow},
			{"(-9223372036854775808 * -9223372036854775808)", nil, ErrIntUnderflow},
			//division
			{"(1 / 2)", Int(0), nil},
			{"(4 / 2)", Int(2), nil},
			{"(1 / 0)", nil, ErrIntDivisionByZero},
			{"(9223372036854775807 / -2)", Int(-4611686018427387903), nil},
			{"(9223372036854775807 / -1)", Int(-9223372036854775807), nil},
			{"(-9223372036854775808 / -2)", Int(4611686018427387904), nil},
			{"(-9223372036854775808 / -1)", nil, ErrIntOverflow},
		}

		for _, testCase := range testCases {
			t.Run(testCase.code, func(t *testing.T) {
				res, err := Eval(testCase.code, NewGlobalState(NewDefaultTestContext(), nil), false)
				if testCase.err == nil {
					assert.NoError(t, err)
					assert.Equal(t, testCase.result, res)
				} else {
					assert.ErrorIs(t, err, testCase.err)
					assert.Nil(t, res)
				}
			})
		}
	})

	t.Run("floating point binary expression", func(t *testing.T) {
		NaN := Float(math.NaN())

		testCases := []struct {
			code   string
			a      Float
			b      Float
			result Value
			err    error
		}{
			//addition
			{"(a + b)", 1, 2, Float(3), nil},
			{"(a + b)", 1, -2, Float(-1), nil},
			{"(a + b)", NaN, 1, nil, ErrNaNinfinityOperand},
			{"(a + b)", 1, NaN, nil, ErrNaNinfinityOperand},
			{"(a + b)", NaN, NaN, nil, ErrNaNinfinityOperand},
			//substraction
			{"(a - b)", 1, 2, Float(-1), nil},
			{"(a - b)", 1, -2, Float(3), nil},
			{"(a - b)", NaN, 1, nil, ErrNaNinfinityOperand},
			{"(a - b)", 1, NaN, nil, ErrNaNinfinityOperand},
			{"(a - b)", NaN, NaN, nil, ErrNaNinfinityOperand},
			//multiplication
			{"(a * b)", 1, 2, Float(2), nil},
			{"(a * b)", 1, -2, Float(-2), nil},
			{"(a * b)", NaN, 1, nil, ErrNaNinfinityOperand},
			{"(a * b)", 1, NaN, nil, ErrNaNinfinityOperand},
			{"(a * b)", NaN, NaN, nil, ErrNaNinfinityOperand},
			//division
			{"(a / b)", 1, 2, Float(0.5), nil},
			{"(a / b)", 1, -2, Float(-0.5), nil},
			{"(a / b)", NaN, 1, nil, ErrNaNinfinityOperand},
			{"(a / b)", 1, NaN, nil, ErrNaNinfinityOperand},
			{"(a / b)", NaN, NaN, nil, ErrNaNinfinityOperand},
			{"(a / b)", 1, 0, nil, ErrNaNinfinityResult},
		}

		for _, testCase := range testCases {
			name := fmt.Sprintf("%s a:%f, d:%f", testCase.code, testCase.a, testCase.b)
			t.Run(name, func(t *testing.T) {
				state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
					"a": testCase.a,
					"b": testCase.b,
				})

				res, err := Eval(testCase.code, state, false)
				if testCase.err == nil {
					assert.NoError(t, err)
					assert.Equal(t, testCase.result, res)
				} else {
					assert.ErrorIs(t, err, testCase.err)
					assert.Nil(t, res)
				}
			})
		}
	})

	t.Run("range expression", func(t *testing.T) {
		testCases := []struct {
			code   string
			result Value
			err    error
		}{
			//addition
			{"(1 .. 2)", IntRange{Start: 1, End: 2, Step: 1, inclusiveEnd: true}, nil},
			{"(1 ..< 2)", IntRange{Start: 1, End: 2, Step: 1, inclusiveEnd: false}, nil},
			{"(1B .. 2B)", QuantityRange{Start: ByteCount(1), End: ByteCount(2), inclusiveEnd: true}, nil},
			{"(1B ..< 2B)", QuantityRange{Start: ByteCount(1), End: ByteCount(2), inclusiveEnd: false}, nil},
		}

		for _, testCase := range testCases {
			t.Run(testCase.code, func(t *testing.T) {
				res, err := Eval(testCase.code, NewGlobalState(NewDefaultTestContext(), nil), false)
				if testCase.err == nil {
					assert.NoError(t, err)
					assert.Equal(t, testCase.result, res)
				} else {
					assert.ErrorIs(t, err, testCase.err)
					assert.Nil(t, res)
				}
			})
		}
	})

	t.Run("binary expression chain", func(t *testing.T) {
		testCases := []struct {
			code   string
			result Value
		}{
			//'or' chain starting with a binary expression
			{"(1 > 2 or false)", False},
			{"(1 < 2 or true)", True},
			{"(1 < 2 or 1 < 2)", True},
			{"(1 < 2 or 1 > 2)", True},
			{"(1 > 2 or 1 > 2)", False},
			{"(1 > 2 or false or false)", False},
			{"(1 < 2 or true or false)", True},
			{"(1 < 2 or 1 < 2 or false)", True},
			{"(1 < 2 or 1 > 2 or false)", True},
			{"(1 > 2 or 1 > 2 or false)", False},
			{"(1 > 2 or 1 > 2 or true)", True},

			//'or' chain starting with a literal
			{"(false or false)", False},
			{"(true or true)", True},
			{"(true or 1 < 2)", True},
			{"(true or 1 > 2)", True},
			{"(false or 1 > 2)", False},
			{"(false or false or false)", False},
			{"(true or true or false)", True},
			{"(true or 1 < 2 or false)", True},
			{"(true or 1 > 2 or false)", True},
			{"(false or 1 > 2 or false)", False},
			{"(false or 1 > 2 or true)", True},

			//'and' chain starting with a binary expression
			{"(1 > 2 and false)", False},
			{"(1 < 2 and true)", True},
			{"(1 < 2 and 1 < 2)", True},
			{"(1 < 2 and 1 > 2)", False},
			{"(1 > 2 and 1 > 2)", False},
			{"(1 > 2 and false and false)", False},
			{"(1 < 2 and true and false)", False},
			{"(1 < 2 and 1 < 2 and false)", False},
			{"(1 < 2 and 1 > 2 and false)", False},
			{"(1 > 2 and 1 > 2 and false)", False},
			{"(1 > 2 and 1 > 2 and true)", False},
			{"(1 > 2 and true and true)", False},
			{"(1 < 2 and true and true)", True},
			{"(1 < 2 and 1 < 2 and true)", True},
			{"(1 < 2 and 1 > 2 and true)", False},
			{"(1 < 2 and false and true)", False},
			{"(1 < 2 and 1 > 2 and true)", False},

			//'and' chain starting with a literal
			{"(false and false)", False},
			{"(true and true)", True},
			{"(true and 1 < 2)", True},
			{"(true and 1 > 2)", False},
			{"(false and 1 > 2)", False},
			{"(false and false and false)", False},
			{"(true and true and false)", False},
			{"(true and 1 < 2 and false)", False},
			{"(true and 1 > 2 and false)", False},
			{"(false and 1 > 2 and false)", False},
			{"(false and 1 > 2 and true)", False},
			{"(false and true and true)", False},
			{"(true and true and true)", True},
			{"(true and 1 < 2 and true)", True},
			{"(true and 1 > 2 and true)", False},
			{"(true and false and true)", False},
			{"(true and 1 > 2 and true)", False},
		}

		for _, testCase := range testCases {
			t.Run(testCase.code, func(t *testing.T) {
				res, err := Eval(testCase.code, NewGlobalState(NewDefaultTestContext(), nil), false)
				assert.NoError(t, err)
				assert.Equal(t, testCase.result, res)
			})
		}
	})

	t.Run("global variable definition", func(t *testing.T) {

		t.Run("simple value", func(t *testing.T) {
			code := `$$a = 1; return a`
			state := NewGlobalState(NewDefaultTestContext())

			res, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, Int(1), res)
		})

		t.Run("watchable", func(t *testing.T) {
			code := `$$a = {}; return a`
			state := NewGlobalState(NewDefaultTestContext())
			state.InitSystemGraph()

			res, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}
			assert.IsType(t, (*Object)(nil), res)

			//check that the global variable's value has a node in the system graph
			if !assert.Len(t, state.SystemGraph.nodes.list, 1) {
				return
			}
			node := state.SystemGraph.nodes.list[0]
			assert.Equal(t, "a", node.name)
		})
	})

	t.Run("local variable declaration", func(t *testing.T) {

		testCases := []struct {
			input          string
			error          bool
			skipIfBytecode bool
			result         Value
		}{
			{
				input: `
					var a = 1; 
					return a
				`,
				result: Int(1),
			},
			{
				input: `
					var (
						a = 1
						b = 2
					)
					return [a, b]
				`,
				result: NewWrappedValueList(Int(1), Int(2)),
			},
		}

		for _, testCase := range testCases {
			if testCase.skipIfBytecode && bytecodeEval {
				continue
			}
			t.Run(testCase.input, func(t *testing.T) {

				state := NewGlobalState(NewDefaultTestContext())
				res, err := Eval(testCase.input, state, false)
				if testCase.error {
					assert.Error(t, err)
					assert.Nil(t, res)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, testCase.result, res)
				}
			})
		}
	})

	t.Run("assignment", func(t *testing.T) {

		testCases := []struct {
			input          string
			error          bool
			skipIfBytecode bool
			result         Value
			constants      map[string]Value
			globalVars     map[string]Value
		}{
			{
				input: `
					a = 1; 
					return a
				`,
				result: Int(1),
			},
			{
				input: `
					a = 1
					a += 1
					return a
				`,
				result: Int(2),
			},
			{
				input: `
					const (
						A = 1
					)
		
					manifest {
						permissions: {
							update: {
								globals: "*"
							}
						}
					}
		
					$$A = 2;
				`,
				error:          true,
				skipIfBytecode: true,
			},
			{
				input: `
					a = {}
					a.v = 1
					return $a
				`,
				result: objFrom(ValMap{"v": Int(1)}),
			},
			{
				input: `
					a = {}
					a.v = 1
					a.v += 1
					return $a
				`,
				result: objFrom(ValMap{"v": Int(2)}),
			},
			{
				input: `
					a = {
						count: 0
						_constraints_ { (self.count == 0) }
					}
					$a.count = 1
				`,
				error: true,
			},
			{
				input: `
					a = [0] 
					$a[0] = 1
					return a
				`,
				result: newList(&ValueList{elements: []Value{Int(1)}}),
			},
			{
				input: `
					a = [1] 
					$a[0] += 1
					return a
				`,
				result: newList(&ValueList{elements: []Value{Int(2)}}),
			},
			{
				input: `
					a = 0x[00] 
					$a[0] = tobyte(1)
					return a
				`,
				constants: map[string]Value{
					"tobyte": WrapGoFunction(func(ctx *Context, i Int) Byte {
						return Byte(i)
					}),
				},
				result: NewByteSlice([]byte{1}, true, ""),
			},
			{
				input: `
					runes[0] = 'b'
					return runes
				`,
				constants: map[string]Value{
					"torune": WrapGoFunction(func(ctx *Context, i Int) Byte {
						return Byte(i)
					}),
				},
				globalVars: map[string]Value{
					"runes": NewRuneSlice([]rune("a")),
				},
				result: NewRuneSlice([]rune("b")),
			},
			{
				input: `
					a = {count:0}
					$a.count = 1
					return $a.count
				`,
				result: Int(1),
			},
			{
				input: `
					a = {count: 1}
					$a.count += 1
					return $a.count
				`,
				result: Int(2),
			},
			{
				input: `
					a = {}
					$a.count = 1; 
					return $a.count
				`,
				result: Int(1),
			},
			{
				input: `
					a = {}
					$a.count = 1; 
					$a.count += 1
					return $a.count
				`,
				result: Int(2),
			},

			{
				input: `
					a = [0] 
					$a[0:1] = [1]
					return $a
				`,
				result: newList(&ValueList{elements: []Value{Int(1)}}),
			},
		}

		for _, testCase := range testCases {
			if testCase.skipIfBytecode && bytecodeEval {
				continue
			}
			t.Run(testCase.input, func(t *testing.T) {
				state := NewGlobalState(NewDefaultTestContext(), testCase.constants)
				for k, v := range testCase.globalVars {
					state.Globals.Set(k, v)
				}
				res, err := Eval(testCase.input, state, false)
				if testCase.error {
					assert.Error(t, err)
					assert.Nil(t, res)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, testCase.result, res)
				}
			})
		}

		t.Run("assignment : LHS is a pipeline expression", func(t *testing.T) {
			code := `a = | get-data | split-lines $; return $a`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"get-data": ValOf(func(ctx *Context) Str {
					return "aaa\nbbb"
				}),
				"split-lines": ValOf(splitLines),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, newList(&ValueList{elements: []Value{Str("aaa"), Str("bbb")}}), res)
		})
	})

	t.Run("set difference", func(t *testing.T) {
		t.Run("patterns", func(t *testing.T) {
			code := `((%| 1 | 2) \ 1)`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.IsType(t, &DifferencePattern{}, res)
			patt := res.(*DifferencePattern)

			assert.IsType(t, &UnionPattern{}, patt.base)
			assert.Equal(t, &ExactValuePattern{value: Int(1)}, patt.removed)
		})
	})

	t.Run("nil coalescing", func(t *testing.T) {
		t.Run("left is nil", func(t *testing.T) {
			code := `(nil ?? 1)`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(1), res)
		})

		t.Run("left is not nil", func(t *testing.T) {
			code := `(1 ?? 2)`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(1), res)
		})
	})

	t.Run("return statement", func(t *testing.T) {
		t.Run("value", func(t *testing.T) {
			code := `return nil`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Nil, res)
		})

		t.Run("no value", func(t *testing.T) {
			code := `return`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Nil, res)
		})
	})

	t.Run("index expression", func(t *testing.T) {
		t.Run("list", func(t *testing.T) {
			code := `
				a = [0] 
				return $a[0]
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(0), res)
		})

		t.Run("tuple", func(t *testing.T) {
			code := `
				a = #[0] 
				return $a[0]
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(0), res)
		})
	})

	t.Run("slice expression", func(t *testing.T) {
		t.Run("array slice : end index is greater than the length of the array", func(t *testing.T) {
			code := `
				a = [0]
				return $a[0:100]
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, newList(&ValueList{elements: []Value{Int(0)}}), res)
		})

		t.Run("string slice : end index is greater than the length of the string", func(t *testing.T) {
			code := `
				a = "0"
				return $a[0:100]
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Str("0"), res)
		})

		t.Run("negative start", func(t *testing.T) {
			code := `
				a = ["a"]
				return $a[-1:1]
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.ErrorIs(t, err, ErrNegativeLowerIndex)
			assert.Nil(t, res)
		})

		t.Run("start and end specified", func(t *testing.T) {
			code := `
				$a = ["a"]
				return $a[0:1]
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, newList(&ValueList{elements: []Value{Str("a")}}), res)
		})

		t.Run("only start specified", func(t *testing.T) {
			code := `
				a = ["a"]
				return $a[0:]
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, newList(&ValueList{elements: []Value{Str("a")}}), res)
		})

		t.Run("only end specified", func(t *testing.T) {
			code := `
				a = ["a"]
				return $a[:1]
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, newList(&ValueList{elements: []Value{Str("a")}}), res)
		})

		t.Run("start out ouf bounds", func(t *testing.T) {
			code := `
				a = ["a"]
				return $a[1:]
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, newList(&ValueList{elements: []Value{}}), res)
		})

	})

	t.Run("quantity literal : byte count", func(t *testing.T) {
		t.Run("byte count", func(t *testing.T) {
			code := `1kB`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
			assert.NoError(t, err)

			assert.EqualValues(t, ByteCount(1_000), res)
		})

		t.Run("too large", func(t *testing.T) {
			code := `10000000000s`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
			assert.Error(t, err)
			assert.ErrorIs(t, err, ErrQuantityLooLarge)
			assert.Nil(t, res)
		})
	})

	t.Run("date literal", func(t *testing.T) {
		code := `2020y-UTC`
		res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
		assert.NoError(t, err)

		assert.EqualValues(t, Date(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)), res)
	})

	t.Run("rate literal : byte rate", func(t *testing.T) {
		t.Run("byte rate", func(t *testing.T) {
			code := `10kB/s`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
			assert.NoError(t, err)

			assert.EqualValues(t, ByteRate(10_000), res)
		})

		t.Run("simple rate", func(t *testing.T) {
			code := `10x/s`
			res, err := Eval(code, NewGlobalState(NewDefaultTestContext()), false)
			assert.NoError(t, err)

			assert.EqualValues(t, SimpleRate(10), res)
		})

	})

	t.Run("global constants", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			code := `
				const ()
			`
			state := NewGlobalState(NewDefaultTestContext())
			_, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, map[string]Value{}, state.Globals.Entries())
		})

		t.Run("single", func(t *testing.T) {
			code := `
				const (
					a = 1
				)
			`
			state := NewGlobalState(NewDefaultTestContext())
			_, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, map[string]Value{"a": Int(1)}, state.Globals.Entries())
		})
	})

	t.Run("object literal", func(t *testing.T) {
		t.Run("empty object", func(t *testing.T) {
			code := `{}`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, &Object{}, res)
		})

		t.Run("single property", func(t *testing.T) {
			code := `{a:1}`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, objFrom(ValMap{"a": Int(1)}), res)
		})

		t.Run("several properties", func(t *testing.T) {
			code := `{a:1,b:2}`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, objFrom(ValMap{"a": Int(1), "b": Int(2)}), res)
		})

		t.Run("only an implicit-key property", func(t *testing.T) {
			code := `{1}`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, objFrom(ValMap{"0": Int(1)}), res)
		})

		t.Run("spread element", func(t *testing.T) {
			code := `
				o = {name: "foo"}
				return { ...$o.{name} }
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, objFrom(ValMap{"name": Str("foo")}), res)
		})

		t.Run("empty lifetime job", func(t *testing.T) {
			code := `{ lifetimejob #job {  } }`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			obj := res.(*Object)
			if !assert.Len(t, obj.jobInstances(), 1) {
				return
			}
			jobInstance := obj.jobInstances()[0]
			assert.Equal(t, obj.Prop(state.Ctx, "0"), jobInstance.job)
			assert.Equal(t, bytecodeEval, jobInstance.routine.useBytecode)
		})

		t.Run("lifetimejob with ungranted permissions", func(t *testing.T) {
			code := `{ 
				lifetimejob "name" { 
					manifest { permissions: { read: https://example.com/index.html } }
				}
			}`

			state := NewGlobalState(NewContext(ContextConfig{
				Permissions: []Permission{RoutinePermission{Kind_: permkind.Create}},
			}))
			res, err := Eval(code, state, false)

			assert.Error(t, err)
			assert.ErrorIs(t, err, NewNotAllowedError(HttpPermission{
				Kind_:  permkind.Read,
				Entity: URL("https://example.com/index.html"),
			}))
			assert.Nil(t, res)
		})

		t.Run("lifetime job accessing self", func(t *testing.T) {
			code := `{ 
				a: 1
				lifetimejob #job { self.a = 2 } 
			}`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			obj := res.(*Object)
			if !assert.Len(t, obj.jobInstances(), 1) {
				return
			}

			jobInstance := obj.jobInstances()[0]
			assert.Equal(t, obj.Prop(state.Ctx, "0"), jobInstance.job)
			assert.Equal(t, bytecodeEval, jobInstance.routine.useBytecode)

			time.Sleep(time.Millisecond)
		})

		t.Run("lifetime job accessing patterns defined in parent state", func(t *testing.T) {
			code := `
				%p = 1
				return { 
					a: []
					lifetimejob #job { self.a = [%p, %int] } 
				}
			`
			state := NewGlobalState(NewDefaultTestContext())
			state.Ctx.AddNamedPattern("int", INT_PATTERN)
			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			obj := res.(*Object)
			time.Sleep(10 * time.Millisecond) //wait for job to finish
			assert.Equal(t, NewWrappedValueList(
				state.Ctx.ResolveNamedPattern("p"),
				state.Ctx.ResolveNamedPattern("int"),
			), obj.Prop(state.Ctx, "a"))
		})
	})

	t.Run("record literal", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			code := `#{}`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, &Record{}, res)
		})

		t.Run("single property", func(t *testing.T) {
			code := `#{a:1}`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, NewRecordFromMap(ValMap{"a": Int(1)}), res)
		})

		t.Run("several properties", func(t *testing.T) {
			code := `#{a:1,b:2}`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, NewRecordFromMap(ValMap{"a": Int(1), "b": Int(2)}), res)
		})

		t.Run("only an implicit-key property", func(t *testing.T) {
			code := `#{1}`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, NewRecordFromMap(ValMap{"0": Int(1)}), res)
		})

	})

	t.Run("dictionary literal", func(t *testing.T) {
		t.Run("literal only keys", func(t *testing.T) {
			code := `:{"name": "foo", ./path: "bar"}`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, NewDictionary(map[string]Value{
				`"name"`: Str(`foo`),
				`./path`: Str(`bar`),
			}), res)
		})

		t.Run("variable key", func(t *testing.T) {
			code := `
				k1 = "name"
				k2 = 1
				return :{k1: "foo", k2: "bar"}
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, NewDictionary(map[string]Value{
				`"name"`: Str(`foo`),
				`1`:      Str(`bar`),
			}), res)
		})

	})

	t.Run("list literal", func(t *testing.T) {
		t.Run("empty list literal", func(t *testing.T) {
			code := `[]`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, newList(&ValueList{elements: nil}), res)
		})

		t.Run("[integer]", func(t *testing.T) {
			code := `[1]`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, newList(&ValueList{elements: []Value{Int(1)}}), res)
		})

		t.Run("[integer,integer]", func(t *testing.T) {
			code := `[1,2]`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, newList(&ValueList{elements: []Value{Int(1), Int(2)}}), res)
		})

		t.Run("[...[integer]]", func(t *testing.T) {
			code := `[...[1]]`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, newList(&ValueList{elements: []Value{Int(1)}}), res)
		})

		t.Run("[integer, ...[integer]]", func(t *testing.T) {
			code := `[0, ...[1]]`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, newList(&ValueList{elements: []Value{Int(0), Int(1)}}), res)
		})
	})

	t.Run("tuple literal", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			code := `#[]`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, &Tuple{elements: []Value{}}, res)
		})

		t.Run("[integer]", func(t *testing.T) {
			code := `#[1]`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, &Tuple{elements: []Value{Int(1)}}, res)
		})

		t.Run("[integer,integer]", func(t *testing.T) {
			code := `#[1,2]`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, &Tuple{elements: []Value{Int(1), Int(2)}}, res)
		})

		t.Run("[...#[integer]]", func(t *testing.T) {
			code := `#[...#[1]]`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, &Tuple{elements: []Value{Int(1)}}, res)
		})

		t.Run("[integer, ...#[integer]]", func(t *testing.T) {
			code := `#[0, ...#[1]]`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, &Tuple{elements: []Value{Int(0), Int(1)}}, res)
		})
	})

	t.Run("multi assignement", func(t *testing.T) {
		t.Run("variable count == length", func(t *testing.T) {
			code := `
				assign a b = [1, 2]
				return [$a, $b]
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, newList(&ValueList{elements: []Value{Int(1), Int(2)}}), res)
		})

		t.Run("variable count > length", func(t *testing.T) {
			code := `assign a b = [1]`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.Error(t, err)
			assert.Nil(t, res)
		})

		t.Run("nillable: variable count > length", func(t *testing.T) {
			code := `
				assign? a b = [1]
				return [$a, $b]
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.EqualValues(t, newList(&ValueList{elements: []Value{Int(1), Nil}}), res)
		})
	})

	t.Run("if statement", func(t *testing.T) {
		t.Run("condition is true", func(t *testing.T) {
			code := `if true { return 1 }`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, Int(1), res)
		})

		t.Run("condition is false", func(t *testing.T) {
			code := `
				a = 0
				if false { 
					$a = 1 
				}
				return $a
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, Int(0), res)
		})

		t.Run("if-else, condition is false", func(t *testing.T) {
			code := `
				a = 0
				b = 0
				if false { 
					$a = 1 
				} else { 
					$b = 1 
				}
				return [$a, $b]
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, newList(&ValueList{elements: []Value{Int(0), Int(1)}}), res)
		})

	})

	t.Run("if expression", func(t *testing.T) {

		t.Run("true condition", func(t *testing.T) {
			code := `(if true 1)`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, Int(1), res)
		})

		t.Run("false condition", func(t *testing.T) {
			code := `(if false 1)`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, Nil, res)
		})

		t.Run("if-else, false condition", func(t *testing.T) {
			code := `(if false 1 else 2)`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, Int(2), res)
		})
	})

	t.Run("for statement", func(t *testing.T) {
		testCases := []struct {
			input   string
			result  Value
			globals func(ctx *Context) map[string]Value
		}{
			{
				input: `
					c = 0
					for i, e in [] {
						$c = 1
					}
					return $c
				`,
				result: Int(0),
			},
			{
				input: `
				c1 = 0
				c2 = 2
				for i, e in [5] {
					c1 = $i
					c2 = $e;
				}
				return [$c1, $c2]
			`,
				result: newList(&ValueList{elements: []Value{Int(0), Int(5)}}),
			},
			{
				input: `
				c = 0
				for e in [5] {
					c = $e
				}
				return $c
			`,
				result: Int(5),
			},
			{
				input: `
				c1 = 0
				c2 = 0
				for i, e in [5,6] {
					c1 = ($c1 + $i);
					c2 = ($c2 + $e);
				}
				return [$c1, $c2]
			`,
				result: newList(&ValueList{elements: []Value{Int(1), Int(11)}}),
			},
			{
				input: `
				c1 = 0
				$c2 = 0
				for i, e in (5 .. 6) {
					c1 = ($c1 + $i)
					c2 = ($c2 + $e);
				}
				return [$c1, $c2]
			`,
				result: newList(&ValueList{elements: []Value{Int(1), Int(11)}}),
			},
			{
				input: `
				c1 = 0
				c2 = 0;
				for i, e in (5 .. 6) {
					c1 = ($c1 + $i);
					if ($i == 1) {
						break
					}
					c2 = ($c2 + $e);
				};
				return [$c1, $c2]
			`,
				result: newList(&ValueList{elements: []Value{Int(1), Int(5)}}),
			},
			{
				input: `
				c1 = 0
				c2 = 0;
				for i, e in (5 .. 6) {
					if ($i == 0) {
						continue
					}
					c1 = ($c1 + $i);
					c2 = ($c2 + $e);
				};
				return [$c1, $c2]
			`,
				result: newList(&ValueList{elements: []Value{Int(1), Int(6)}}),
			},
			{
				input: `
				c = 0
				for (1 .. 2) {
					c = ($c + 1)
				}
				return $c
			`,
				result: Int(2),
			},
			{
				input: `
				c = 0
				%p = %| 1 | 3
				for %p n in [0 1 2 3] {
					c = ($c + $n)
				}
				return $c
			`,
				result: Int(4),
			},
			{
				input: `
				c = 0
				indexSum = 0

				%i = 3
				%p = %| 1 | 3
				for %i i, %p n in [0 1 2 3] {
					c = (c + n)
					indexSum = (indexSum + i)
				}
				return [c, indexSum]
			`,
				result: NewWrappedValueList(Int(3), Int(3)),
			},

			{
				input: `
				for (1 .. 2) {
					1
				}
			`,
				result: Nil,
			},
			{
				input: `
				for e in (1 .. 2) {
					1
				}
			`,
				result: Nil,
			},

			{
				input: `
				for i, e in (1 .. 2) {
					1
				}
			`,
				result: Nil,
			},
			{
				input: `
					elements = []
					for e in streamable {
						append(elements, e)
					}
					return elements
				`,
				globals: func(ctx *Context) map[string]Value {
					watcher := NewGenericWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN})
					watcher.InformAboutAsync(ctx, Str("a"))
					watcher.InformAboutAsync(ctx, Str("b"))

					go func() {
						time.Sleep(10 * time.Millisecond)
						watcher.Stop()
					}()
					return map[string]Value{
						"streamable": StreamSource(watcher),
						"append":     ValOf(Append),
					}
				},
				result: NewWrappedValueList(Str("a"), Str("b")),
			},
			{
				input: `
					elements = []
					for e in streamable {
						append(elements, e)
						break
					}
					return elements
				`,
				globals: func(ctx *Context) map[string]Value {
					watcher := NewGenericWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN})
					watcher.InformAboutAsync(ctx, Str("a"))
					watcher.InformAboutAsync(ctx, Str("b"))

					go func() {
						time.Sleep(10 * time.Millisecond)
						watcher.Stop()
					}()
					return map[string]Value{
						"streamable": StreamSource(watcher),
						"append":     ValOf(Append),
					}
				},
				result: NewWrappedValueList(Str("a")),
			},
			{
				input: `
					for chunked chunk in streamable {
						return chunk.data
					}
				`,
				globals: func(ctx *Context) map[string]Value {
					watcher := NewGenericWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN})
					watcher.InformAboutAsync(ctx, Str("a"))
					watcher.InformAboutAsync(ctx, Str("b"))

					go func() {
						time.Sleep(10 * time.Millisecond)
						watcher.Stop()
					}()
					return map[string]Value{
						"streamable": StreamSource(watcher),
						"append":     ValOf(Append),
					}
				},
				result: NewWrappedValueList(Str("a"), Str("b")),
			},
			{
				input: `
					elements = #[]
					for chunked chunk in streamable { 
						elements = chunk.data
						break
					} 
					return elements
				`,
				globals: func(ctx *Context) map[string]Value {
					watcher := NewGenericWatcher(WatcherConfiguration{Filter: ANYVAL_PATTERN})
					watcher.InformAboutAsync(ctx, Str("a"))
					watcher.InformAboutAsync(ctx, Str("b"))

					go func() {
						time.Sleep(10 * time.Millisecond)
						watcher.Stop()
					}()
					return map[string]Value{
						"streamable": StreamSource(watcher),
						"append":     ValOf(Append),
					}
				},
				result: NewWrappedValueList(Str("a"), Str("b")),
			},
			{
				input: `
					data = []
					for chunked chunk in streamable {
						append(data, chunk.data)
					}
					return data
				`,
				globals: func(ctx *Context) map[string]Value {
					return map[string]Value{
						"streamable": NewElementsStream(
							[]Value{Str("a"), Str("b"), Str("c"), Str("d")},
							nil,
						),
						"append": ValOf(Append),
					}
				},
				result: NewWrappedValueList(
					NewWrappedValueList(Str("a"), Str("b")),
					NewWrappedValueList(Str("c"), Str("d")),
				),
			},
			//TODO: add more tests with EOS error
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				state := NewGlobalState(NewDefaultTestContext())
				if testCase.globals != nil {
					for k, v := range testCase.globals(state.Ctx) {
						state.Globals.permanent[k] = v
					}
				}

				res, err := Eval(testCase.input, state, false)
				assert.NoError(t, err)
				assert.EqualValues(t, testCase.result, res)
			})
		}

	})

	t.Run("walk statement", func(t *testing.T) {
		GET_ENTRIES_CODE := `
			entries = []
			walk $$dir entry {
				entries = append($entries, $entry)
			}
			return $entries
		`

		regularFilename := "file.txt"
		subdirName := "subdir"
		subdir1Name := "subdir1"
		subdir2Name := "subdir2"

		testCases := []struct {
			name   string
			input  string
			result func(tempDir string, tempDirPath Path) Value
			before func(tempDir string, tempDirPath Path)
		}{
			{
				//empty dir
				input: GET_ENTRIES_CODE,
				result: func(tempDir string, tempDirPath Path) Value {
					return newList(&ValueList{
						elements: []Value{
							objFrom(ValMap{
								"name":          Str(filepath.Base(tempDir)),
								"path":          tempDirPath,
								"is-dir":        True,
								"is-regular":    False,
								"is-walk-start": True,
							}),
						},
					})
				},
			},
			{
				name:  "dir with single regular file",
				input: GET_ENTRIES_CODE,
				before: func(tempDir string, tempDirPath Path) {
					regularFilePath := filepath.Join(tempDir, regularFilename)
					os.WriteFile(regularFilePath, nil, 0o400)
				},
				result: func(tempDir string, tempDirPath Path) Value {
					regularFilePath := filepath.Join(tempDir, regularFilename)

					return newList(&ValueList{
						elements: []Value{
							objFrom(ValMap{
								"name":          Str(filepath.Base(tempDir)),
								"path":          tempDirPath,
								"is-dir":        Bool(true),
								"is-regular":    Bool(false),
								"is-walk-start": Bool(true),
							}),
							objFrom(ValMap{
								"name":          Str(regularFilename),
								"path":          Path(regularFilePath),
								"is-dir":        Bool(false),
								"is-regular":    Bool(true),
								"is-walk-start": Bool(false),
							}),
						},
					})
				},
			},
			{

				name:  "dir with a regular file and an empty subdirectory",
				input: GET_ENTRIES_CODE,
				before: func(tempDir string, tempDirPath Path) {
					regularFilePath := filepath.Join(tempDir, regularFilename)
					subdirPath := filepath.Join(tempDir, subdirName)
					os.WriteFile(regularFilePath, nil, 0o400)
					os.Mkdir(subdirPath, 0x500)
				},
				result: func(tempDir string, tempDirPath Path) Value {
					regularFilePath := filepath.Join(tempDir, regularFilename)
					subdirPath := filepath.Join(tempDir, subdirName)

					return newList(&ValueList{
						elements: []Value{
							objFrom(ValMap{
								"name":          Str(filepath.Base(tempDir)),
								"path":          Path(tempDir + "/"),
								"is-dir":        Bool(true),
								"is-regular":    Bool(false),
								"is-walk-start": Bool(true),
							}),
							objFrom(ValMap{
								"name":          Str(regularFilename),
								"path":          Path(regularFilePath),
								"is-dir":        Bool(false),
								"is-regular":    Bool(true),
								"is-walk-start": Bool(false),
							}),
							objFrom(ValMap{
								"name":          Str(subdirName),
								"path":          Path(subdirPath + "/"),
								"is-dir":        Bool(true),
								"is-regular":    Bool(false),
								"is-walk-start": Bool(false),
							}),
						},
					})
				},
			},
			{
				name: "dir with a regular file and an empty subdirectory : prune when regular file is encountered",
				input: `
					entries = []
					walk $$dir entry {
						entries = append($entries, $entry)
						if $entry.is-regular {
							prune
						}
					}
					return $entries
				`,
				before: func(tempDir string, tempDirPath Path) {
					regularFilePath := filepath.Join(tempDir, regularFilename)
					subdirPath := filepath.Join(tempDir, subdirName)
					os.WriteFile(regularFilePath, nil, 0o400)
					os.Mkdir(subdirPath, 0x500)
				},
				result: func(tempDir string, tempDirPath Path) Value {
					regularFilePath := filepath.Join(tempDir, regularFilename)

					return newList(&ValueList{
						elements: []Value{
							objFrom(ValMap{
								"name":          Str(filepath.Base(tempDir)),
								"path":          Path(tempDir + "/"),
								"is-dir":        Bool(true),
								"is-regular":    Bool(false),
								"is-walk-start": Bool(true),
							}),
							objFrom(ValMap{
								"name":          Str(regularFilename),
								"path":          Path(regularFilePath),
								"is-dir":        Bool(false),
								"is-regular":    Bool(true),
								"is-walk-start": Bool(false),
							}),
						},
					})
				},
			},
			{
				name: "dir with to subdirectories : prune when one of the dir is encountered",
				input: `
					entries = []
					walk $$dir entry {
						entries = append($entries, $entry)
						if $entry.is-walk-start {
							continue
						}
						if $entry.is-dir {
							prune
						}
					}
					return $entries
				`,
				before: func(tempDir string, tempDirPath Path) {
					subdir1Path := filepath.Join(tempDir, subdir1Name)
					subdir2Path := filepath.Join(tempDir, subdir2Name)

					os.Mkdir(subdir1Path, 0x500)
					os.Mkdir(subdir2Path, 0x500)
				},
				result: func(tempDir string, tempDirPath Path) Value {
					subdir1Path := filepath.Join(tempDir, subdir1Name)
					subdir2Path := filepath.Join(tempDir, subdir2Name)

					return newList(&ValueList{
						elements: []Value{
							objFrom(ValMap{
								"name":          Str(filepath.Base(tempDir)),
								"path":          Path(tempDir + "/"),
								"is-dir":        Bool(true),
								"is-regular":    Bool(false),
								"is-walk-start": Bool(true),
							}),
							objFrom(ValMap{
								"name":          Str(subdir1Name),
								"path":          Path(subdir1Path + "/"),
								"is-dir":        Bool(true),
								"is-regular":    Bool(false),
								"is-walk-start": Bool(false),
							}),
							objFrom(ValMap{
								"name":          Str(subdir2Name),
								"path":          Path(subdir2Path + "/"),
								"is-dir":        Bool(true),
								"is-regular":    Bool(false),
								"is-walk-start": Bool(false),
							}),
						},
					})
				},
			},
			{
				name: "dir with to subdirectories : break when one of the dir is encountered",
				input: `
					entries = []
					walk $$dir entry {
						entries = append($entries, $entry)
						if $entry.is-walk-start {
							continue
						}
						if $entry.is-dir {
							break
						}
					}
					return $entries
				`,
				before: func(tempDir string, tempDirPath Path) {
					subdir1Path := filepath.Join(tempDir, subdir1Name)
					subdir2Path := filepath.Join(tempDir, subdir2Name)

					os.Mkdir(subdir1Path, 0x500)
					os.Mkdir(subdir2Path, 0x500)
				},
				result: func(tempDir string, tempDirPath Path) Value {
					subdir1Path := filepath.Join(tempDir, subdir1Name)

					return newList(&ValueList{
						elements: []Value{
							objFrom(ValMap{
								"name":          Str(filepath.Base(tempDir)),
								"path":          Path(tempDir + "/"),
								"is-dir":        Bool(true),
								"is-regular":    Bool(false),
								"is-walk-start": Bool(true),
							}),
							objFrom(ValMap{
								"name":          Str(subdir1Name),
								"path":          Path(subdir1Path + "/"),
								"is-dir":        Bool(true),
								"is-regular":    Bool(false),
								"is-walk-start": Bool(false),
							}),
						},
					})
				},
			},
			{
				name: "for statement in body",
				input: `
					a = 0
					walk $$dir entry {
						for i in 1..10 {
							a = (a + 1)
							break
						}
						a = (a + 2)
						break
					}
					return a
				`,
				before: func(tempDir string, tempDirPath Path) {},
				result: func(tempDir string, tempDirPath Path) Value {
					return Int(3)
				},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				tempDir := t.TempDir()
				tempDirPath := Path(tempDir + "/")

				if testCase.before != nil {
					testCase.before(tempDir, tempDirPath)
				}

				ctx := NewContext(ContextConfig{
					Permissions: []Permission{
						GlobalVarPermission{permkind.Read, "*"},
						GlobalVarPermission{permkind.Update, "*"},
						GlobalVarPermission{permkind.Create, "*"},
						GlobalVarPermission{permkind.Use, "*"},
						FilesystemPermission{permkind.Read, PathPattern(tempDirPath + "...")},
					},
					Filesystem: newOsFilesystem(),
				})

				state := NewGlobalState(ctx, map[string]Value{
					"dir":    tempDirPath,
					"append": ValOf(Append),
				})
				res, err := Eval(testCase.input, state, false)
				assert.NoError(t, err)

				expectedResult := testCase.result(tempDir, tempDirPath)
				assert.Equal(t, expectedResult, res)
			})
		}

	})

	t.Run("switch statement", func(t *testing.T) {
		testCases := []struct {
			name   string
			input  string
			result Value
		}{
			{
				name: "single case (that matches)",
				input: `
				a = 0
				switch 0 { 
					0 { a = 1 } 
				}
				return a
			`,
				result: Int(1),
			},
			{
				name: "two cases: first matches",
				input: `
				a = 0; 
				b = 0; 
				switch 0 { 
					0 { a = 1 } 
					1 { b = 1} 
				}; 
				return [$a,$b]
			`,
				result: newList(&ValueList{elements: []Value{Int(1), Int(0)}}),
			},
			{
				name: "two cases: second matches",
				input: `
				a = 0
				b = 0 
				switch 1 { 
					0 { a = 1 } 
					1 { b = 1 } 
				}; 
				return [$a,$b]
			`,
				result: newList(&ValueList{elements: []Value{Int(0), Int(1)}}),
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				state := NewGlobalState(NewDefaultTestContext())
				res, err := Eval(testCase.input, state, false)
				assert.NoError(t, err)
				assert.Equal(t, testCase.result, res)
			})
		}
	})

	t.Run("match statement", func(t *testing.T) {
		testCases := []struct {
			name   string
			input  string
			result Value
		}{
			{
				name: "patterns : two cases (first matches)",
				input: `
					a = 0
					b = 0 
					match / { 
						%/* { a = 1 } 
						%/e* { b = 1} 
					}; 
					return [$a,$b]
				`,
				result: newList(&ValueList{elements: []Value{Int(1), Int(0)}}),
			},
			{
				name: "group patterns : two cases (first one matches)",
				input: `
					a = 0; 
					b = 0; 
					match /home/user { 
						%/home/{:username} m { a = m.username } 
						%/hom/{:username} { b = 1 } 
					}; 
					return [$a,$b]
				`,
				result: newList(&ValueList{elements: []Value{Str("user"), Int(0)}}),
			},
			{
				name: "group patterns : two cases (second one matches)",
				input: `
					a = 0
					b = 0
					match /e { 
						%/f* { a = 1 } 
						%/e* { b = 1} 
					} 
					return [$a,$b]
				`,
				result: newList(&ValueList{elements: []Value{Int(0), Int(1)}}),
			},
			{
				name: "equality : two cases (second one matches)",
				input: `
					a = 0; 
					b = 0; 
					match /e { 
						%/f* { a = 1 } 
						/e { b = 1} 
					}
					return [$a, $b]
				`,
				result: newList(&ValueList{elements: []Value{Int(0), Int(1)}}),
			},
			{
				name: "seconde case is not a simple value but is statically known",
				input: `
					a = 0; 
					b = 0; 
					match {a:1} { 
						%/f* { a = 1 } 
						({a:1}) { b = 1 } 
					}; 
					return [$a,$b]
				`,
				result: newList(&ValueList{elements: []Value{Int(0), Int(1)}}),
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				state := NewGlobalState(NewDefaultTestContext())
				res, err := Eval(testCase.input, state, false)
				assert.NoError(t, err)
				assert.Equal(t, testCase.result, res)
			})
		}
	})

	t.Run("upper bound range expression ", func(t *testing.T) {
		t.Run("integer ", func(t *testing.T) {
			code := `return ..10`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, IntRange{
				unknownStart: true,
				inclusiveEnd: true,
				Start:        0,
				End:          10,
				Step:         1,
			}, res)
		})

		t.Run("quantity", func(t *testing.T) {
			code := `return ..10s`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, QuantityRange{
				unknownStart: true,
				inclusiveEnd: true,
				Start:        nil,
				End:          Duration(10 * time.Second),
			}, res)
		})
	})

	t.Run("rune range expression", func(t *testing.T) {
		code := `'a'..'z'`
		state := NewGlobalState(NewDefaultTestContext())
		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, RuneRange{'a', 'z'}, res)
	})

	t.Run("function expression", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			code := `fn(){}`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			assert.IsType(t, &InoxFunction{}, res)
		})

		t.Run("captured locals", func(t *testing.T) {
			code := `
				a = 1
				b = 2
				return fn[a,b](){}
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			assert.IsType(t, &InoxFunction{}, res)
			if bytecodeEval {
				assert.Equal(t, []Value{Int(1), Int(2)}, res.(*InoxFunction).capturedLocals)
			} else {
				assert.Equal(t, map[string]Value{
					"a": Int(1),
					"b": Int(2),
				}, res.(*InoxFunction).treeWalkCapturedLocals)
			}
		})

		t.Run("captured locals should be thread safe", func(t *testing.T) {
			code := `
				obj = {}
				return fn[obj](){}
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			assert.IsType(t, &InoxFunction{}, res)
			if bytecodeEval {
				captured := res.(*InoxFunction).capturedLocals[0]
				assert.IsType(t, captured, &Object{})
				assert.True(t, captured.(*Object).IsShared())
			} else {
				captured := res.(*InoxFunction).treeWalkCapturedLocals["obj"]
				assert.IsType(t, captured, &Object{})
				assert.True(t, captured.(*Object).IsShared())
			}
		})
	})

	t.Run("function declaration", func(t *testing.T) {
		code := `fn f(){}`
		state := NewGlobalState(NewDefaultTestContext())
		_, err := Eval(code, state, false)
		assert.NoError(t, err)

		assert.Contains(t, state.Globals.Entries(), "f")
		assert.IsType(t, &InoxFunction{}, state.Globals.Get("f"))
	})

	t.Run("Inox function call", func(t *testing.T) {

		noargs := func() []Value { return nil }

		testCases := []struct {
			name                  string
			error                 bool
			input                 string
			result                Value
			checkResult           func(t *testing.T, result Value, state *GlobalState)
			isShared              bool
			isolatedCaseArguments func() []Value
			doSymbolicCheck       bool
		}{
			{
				name: "declared void function",
				input: `
					fn f(){  }
					return f()
				`,
				result:                Nil,
				isolatedCaseArguments: noargs,
			},
			{
				name: "declared non-void function",
				input: `
					fn f(){
						return 1
					}
					return f()
				`,
				isolatedCaseArguments: noargs,
				result:                Int(1),
			},
			{
				name: "declared function returning its sole argument",
				input: `
					fn f(a){
						return a
					}
					return f(1)
					`,
				isolatedCaseArguments: func() []Value { return []Value{Int(1)} },
				result:                Int(1),
			},
			{
				name: "declared function with a captured value",
				input: `
					a = 1
					fn[a] f(){ return a }
					return f()
				`,
				isolatedCaseArguments: noargs,
				result:                Int(1),
			},
			{
				name: "declared function with a captured value and a local",
				input: `
					a = 1
					fn[a] f(b){ return [a, b] }
					return f(2)
				`,
				isolatedCaseArguments: func() []Value { return []Value{Int(2)} },
				result:                newList(&ValueList{elements: []Value{Int(1), Int(2)}}),
			},
			{
				name: "declared function with two captured values",
				input: `
					a = 1
					b = 2
					fn[a, b] f() { return [a, b] }
					return f()
				`,
				isolatedCaseArguments: noargs,
				result:                newList(&ValueList{elements: []Value{Int(1), Int(2)}}),
			},
			{
				name: "declared function returning a function expression",
				input: `
					fn f() { return fn() => 1 }
					return f()
				`,
				isolatedCaseArguments: noargs,
				doSymbolicCheck:       true,
				checkResult: func(t *testing.T, result Value, state *GlobalState) {
					assert.IsType(t, (*InoxFunction)(nil), result)
				},
			},
			{
				name: "declared arrow function",
				input: `
					fn f() => 1
					return f()
				`,
				isolatedCaseArguments: noargs,
				result:                Int(1),
			},
			{
				name: "declared arrow function returning its sole argument",
				input: `
					fn f(a) => a
					return f(1)
				`,
				isolatedCaseArguments: func() []Value { return []Value{Int(1)} },
				result:                Int(1),
			},
			{
				name: "declared arrow function with a captured value",
				input: `
					a = 1
					fn[a] f() => a
					return f()
				`,
				isolatedCaseArguments: noargs,
				result:                Int(1),
			},
			{
				name: "declared arrow function with a captured value and a local",
				input: `
					a = 1
					fn[a] f(b) => [a, b]
					return f(2)
				`,
				isolatedCaseArguments: func() []Value { return []Value{Int(2)} },
				result:                newList(&ValueList{elements: []Value{Int(1), Int(2)}}),
			},
			{
				name: "declared arrow function with two captured values",
				input: `
					a = 1
					b = 2
					fn[a, b] f() => [a, b]
					return f()
				`,
				isolatedCaseArguments: noargs,
				result:                newList(&ValueList{elements: []Value{Int(1), Int(2)}}),
			},
			{
				name: "declared arrow function returning a function expression",
				input: `
					fn f() => fn() => 1
					return f()
				`,
				isolatedCaseArguments: noargs,
				doSymbolicCheck:       true,
				checkResult: func(t *testing.T, result Value, state *GlobalState) {
					assert.IsType(t, (*InoxFunction)(nil), result)
				},
			},
			{
				name:  "too many arguments",
				error: true,
				input: `
					fn f(){
						return 1
					}
					return f(1)
				`,
			},
			{
				name:  "not enough arguments",
				error: true,
				input: `
					fn f(x){
						return 1
					}
					return f()
				`,
			},
			{
				name: "variadic function with just enough arguments",
				input: `
					fn f(x, ...y){
						return [$x, $y]
					}
					return f(1)
				`,
				result: newList(&ValueList{elements: []Value{Int(1), newList(&ValueList{elements: []Value{}})}}),
			},
			{
				name: "variadic function with many arguments",
				input: `
					fn f(x, ...y){
						return [$x, $y]
					}
					return f(1, 2, 3)
				`,
				result: newList(&ValueList{
					elements: []Value{Int(1), newList(&ValueList{elements: []Value{Int(2), Int(3)}})},
				}),
			},
			{
				name:  "non-variadic function with a spread argument",
				error: true,
				input: `
					fn f(x){ return $x }
					return f(...[1])
				`,
			},
			{
				name:  "variadic function with not enough arguments",
				error: true,
				input: `
					fn f(x, ...y){ }
					return f()
				`,
			},
			{
				name: "call from another function",
				input: `
					fn g(){
						return 2
					}

					fn f(){
						return [1, g()]
					}
					return f()
				`,
				isolatedCaseArguments: noargs,
				result:                newList(&ValueList{elements: []Value{Int(1), Int(2)}}),
			},
			{
				name: "recursive function",
				input: `
					fn factorial(i){
						if (i == 0) {
							return 1
						}
						return (i * factorial( (i - 1) ))
					}
					return factorial(3)
				`,
				isolatedCaseArguments: func() []Value { return []Value{Int(3)} },
				result:                Int(6),
			},
			{
				name: "recursive function accessing a global",
				input: `
					$$a = 3
					fn rec(i %int){
						if (i == 0) {
							return 0
						}
						return (a + rec((i - 1)))
					}
					result = rec(2)
					return [result, a] # we also check that a is still accessible
				`,
				result: NewWrappedValueList(Int(6), Int(3)),
			},
			{
				name: "function calling a recursive function accessing a global",
				input: `
					$$a = 3
					fn rec(i %int){
						if (i == 0) {
							return 0
						}
						return (a + rec((i - 1)))
					}

					fn f(){
						return [rec(2), a] # we also check that a is still accessible
					}
					return f()
				`,
				isolatedCaseArguments: noargs,
				result:                NewWrappedValueList(Int(6), Int(3)),
			},
			{
				name: "method calling a recursive function accessing a global",
				input: `
					$$a = 3
					fn rec(i %int){
						if (i == 0) {
							return 0
						}
						return (a + rec((i - 1)))
					}

					obj = {
						f: fn(){
							return [rec(2), a] # we also check that a is still accessible
						}
					}
					
					return obj.f()
				`,
				isolatedCaseArguments: noargs,
				result:                NewWrappedValueList(Int(6), Int(3)),
			},
			{
				name: "return is in if statement",
				input: `
					fn f(){
						if true { return 1 }
					}
					return f()
				`,
				result: Int(1),
			},
			{
				name: "many calls of a void function with no parameters",
				input: strings.ReplaceAll(`
					fn f(){}
					many_calls
				`, "many_calls", strings.Repeat("f()\n", 10+VM_STACK_SIZE)),
				result: Nil,
			},
			{
				name: "external func : no parameters, no return value",
				input: `
					rt = go do { return fn(){} }

					f = rt.wait_result!()
					return f()
				`,
				result: Nil,
			},
			{
				name: "external func returning an integer",
				input: `
					routine = go do {
						return fn(){ return 1 }
					}

					f = routine.wait_result!()
					return f()
				`,
				isolatedCaseArguments: noargs,
				result:                Int(1),
			},
			{
				name: "external func returning an object",
				input: `
					routine = go do { 
						return fn(){ return {} } 
					}

					f = routine.wait_result!()
					return f()
				`,
				isShared:              true,
				isolatedCaseArguments: noargs,
				result:                NewObject(),
			},
			{
				name: "external func returning its argument, argument should be shared",
				input: `
					shared_value = fn(){}

					routine = go do { 
						return fn(arg){ return arg } 
					}

					f = routine.wait_result!()
					return f(shared_value)
				`,
				checkResult: func(t *testing.T, result Value, state *GlobalState) {
					if result.(*InoxFunction).originState != state {
						assert.Fail(t, "origin state of shared value is invalid")
					}
				},
			},
			{
				name: "external func : many calls of a void function with no parameters",
				input: strings.ReplaceAll(`
					routine = go do { 
						return fn(){ } 
					}

					f = routine.wait_result!()
					many_calls			
				`, "many_calls", strings.Repeat("f()\n", 10+VM_STACK_SIZE)),
				result: Nil,
			},
			{
				name: "method call",
				input: `
					o = {
						a: 1
						getA: fn() => self.a
					}
					return o.getA()
				`,
				result: Int(1),
			},
			{
				name: "method call within method call",
				input: `
					o = {
						b: {
							b: 1
							getB: fn() => self.b
						}
						getA: fn() => self.b.getB()
					}
					return o.getA()
				`,
				result: Int(1),
			},
			{
				name: "several method calls within method call",
				input: `
					o = {
						a: 2
						b: {
							b: 1
							getB: fn() => self.b
						}
						getA: fn() => [self.a, self.b.getB(), self.a, self.b.getB()]
					}
					return o.getA()
				`,
				result: NewWrappedValueList(Int(2), Int(1), Int(2), Int(1)),
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				state := NewGlobalState(NewDefaultTestContext())
				res, err := Eval(testCase.input, state, testCase.doSymbolicCheck)
				if testCase.error {
					assert.Error(t, err)
					assert.Nil(t, res)
				} else {
					assert.NoError(t, err)

					if testCase.checkResult != nil {
						testCase.checkResult(t, res, state)
					} else {
						expected := testCase.result
						if testCase.isShared && utils.Ret0(IsSharable(expected, state)) {
							expected = Share(expected.(PotentiallySharable), state)
						}
						assert.Equal(t, expected, res)
					}
				}
			})

			if bytecodeEval && testCase.isolatedCaseArguments != nil {
				t.Run("isolated_call_"+testCase.name, func(t *testing.T) {
					state := NewGlobalState(NewDefaultTestContext())
					lastOpeningParenIndex := strings.LastIndexByte(testCase.input, '(')
					input := testCase.input[:lastOpeningParenIndex]

					val, err := Eval(input, state, testCase.doSymbolicCheck)
					if !assert.NoError(t, err) {
						return
					}

					fn := val.(*InoxFunction)
					res, err := fn.Call(state, nil, testCase.isolatedCaseArguments(), nil)

					if testCase.error {
						assert.Error(t, err)
						assert.Nil(t, res)
					} else {
						assert.NoError(t, err)

						if testCase.checkResult != nil {
							testCase.checkResult(t, res, state)
						} else {
							expected := testCase.result
							if testCase.isShared && utils.Ret0(IsSharable(expected, state)) {
								expected = Share(expected.(PotentiallySharable), state)
							}
							assert.Equal(t, expected, res)
						}
					}
				})
			}
		}
	})

	t.Run("Go function call", func(t *testing.T) {
		testCases := []struct {
			name            string
			error           bool
			input           string
			globalVariables map[string]Value
			makeGlobals     func(t *testing.T) map[string]Value
			result          Value
		}{
			{
				name:  "variadic: arg count < non-variadic-param-count",
				input: "gofunc()",
				error: true,
				globalVariables: map[string]Value{
					"gofunc": WrapGoFunction(func(ctx *Context, x Int, xs ...Int) {}),
				},
			},
			{
				name:  "variadic: arg count == non-variadic-param-count",
				input: "gofunc(1)",
				globalVariables: map[string]Value{
					"gofunc": WrapGoFunction(func(ctx *Context, x Int, xs ...Int) Int {
						return x
					}),
				},
				result: Int(1),
			},
			{
				name:  "variadic: arg count == 1 + non-variadic-param-count",
				input: "gofunc(1, 2)",
				globalVariables: map[string]Value{
					"gofunc": WrapGoFunction(func(ctx *Context, x Int, xs ...Int) Int {
						return Int(x + xs[0])
					}),
				},
				result: Int(3),
			},
			{
				name: "shared values should be unwrapped",
				input: `
					$rt = go {globals: {gofunc: gofunc, x: {a: 1}}} do {
						return gofunc(x)
					}
		
					$rt.wait_result()
					return nil
				`,
				makeGlobals: func(t *testing.T) map[string]Value {
					return map[string]Value{
						"gofunc": WrapGoFunction(func(ctx *Context, obj *Object) {
							assert.Equal(t, map[string]Value{"a": Int(1)}, obj.EntryMap())
						}),
					}
				},
				result: Nil,
			},
			{
				name: "go functions should not 'share' their arguments",
				input: `
					$rt = go {globals: {gofunc: gofunc, x: {a: 1}}} do {
						shared_value = fn(){}
						return gofunc(shared_value)
					}
		
					$rt.wait_result()
					return nil
				`,
				makeGlobals: func(t *testing.T) map[string]Value {
					return map[string]Value{
						"gofunc": WrapGoFunction(func(ctx *Context, sharedValue *InoxFunction) {
							assert.False(t, sharedValue.IsShared())

						}),
					}
				},
				result: Nil,
			},
			{
				name: "Go functions should not 'share' their arguments",
				input: `
					$rt = go {globals: {gofunc: gofunc}} do {
						shared_value = fn(){}
						return gofunc(shared_value)
					}
		
					$rt.wait_result()
					return nil
				`,
				makeGlobals: func(t *testing.T) map[string]Value {
					return map[string]Value{
						"gofunc": WrapGoFunction(func(ctx *Context, sharedValue *InoxFunction) {
							assert.False(t, sharedValue.IsShared())

						}),
					}
				},
				result: Nil,
			},
			//TODO: add following tests when Go methods & closures can be shared
			// {
			// 	name: "Go methods should 'share' their arguments",
			// 	input: `
			// 		$rt = go {globals: {gofunc: gofunc}} do {
			// 			shared_value = fn(){}
			// 			return gofunc(shared_value)
			// 		}

			// 		$rt.wait_result()
			// 		return nil
			// 	`,
			// 	makeGlobals: func(t *testing.T) map[string]Value {
			// 		return map[string]Value{
			// 			"gofunc": WrapGoFunction(func(ctx *Context, sharedValue *InoxFunction) {
			// 				assert.True(t, sharedValue.IsShared())
			// 			}),
			// 		}
			// 	},
			// 	result: Nil,
			// },
			// {
			// 	name: "Go closures should 'share' their arguments",
			// 	input: `
			// 		$rt = go {globals: {gofunc: gofunc}} do {
			// 			shared_value = fn(){}
			// 			return gofunc(shared_value)
			// 		}

			// 		$rt.wait_result()
			// 		return nil
			// 	`,
			// 	makeGlobals: func(t *testing.T) map[string]Value {
			// 		return map[string]Value{
			// 			"gofunc": WrapGoClosure(func(ctx *Context, sharedValue *InoxFunction) {
			// 				assert.True(t, sharedValue.IsShared())
			// 			}),
			// 		}
			// 	},
			// 	result: Nil,
			// },
			{
				name:  "(must) call with two results",
				input: "return gofunc!()",
				globalVariables: map[string]Value{
					"gofunc": WrapGoFunction(func(ctx *Context) (Int, error) {
						return 3, nil
					}),
				},
				result: Int(3),
			},
			{
				name:  "GoValue returned",
				input: "return getuser()",
				globalVariables: map[string]Value{
					"getuser": WrapGoFunction(func(ctx *Context) GoValue {
						return testMutableGoValue{Name: "Foo"}
					}),
				},
				result: testMutableGoValue{Name: "Foo"},
			},
			{
				name:  "[]string returned, should be converted to a list",
				input: "return getNames()",
				globalVariables: map[string]Value{
					"getNames": WrapGoFunction(func(ctx *Context) []Str {
						return []Str{"string"}
					}),
				},
				result: newList(&ValueList{elements: []Value{Str("string")}}),
			},
			{
				name:  "method",
				input: "return $$user.getName()",
				globalVariables: map[string]Value{
					"user": testMutableGoValue{"Foo", ""},
				},
				result: Str("Foo"),
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				globals := testCase.globalVariables
				if testCase.makeGlobals != nil {
					globals = testCase.makeGlobals(t)
				}

				state := NewGlobalState(NewDefaultTestContext())
				for k, v := range globals {
					state.Globals.Set(k, v)
				}
				state.Logger = zerolog.New(state.Out)
				state.Out = os.Stdout

				res, err := Eval(testCase.input, state, false)
				if testCase.error {
					assert.Error(t, err)
					assert.Nil(t, res)
				} else {
					assert.NoError(t, err)
					assert.Equal(t, testCase.result, res)
				}
			})
		}

	})

	t.Run("pattern call", func(t *testing.T) {
		code := `%mypattern(1..10)`

		state := NewGlobalState(NewDefaultTestContext())
		state.Ctx.AddNamedPattern("mypattern", NewRegexPattern(".*"))

		res, err := Eval(code, state, false)
		assert.NoError(t, err)

		expectedPattern, _ := NewRegexPattern(".*").Call([]Value{
			IntRange{
				Start:        1,
				End:          10,
				inclusiveEnd: true,
				Step:         1,
			},
		})

		assert.Equal(t, expectedPattern, res)
	})

	t.Run("function pattern definition,", func(t *testing.T) {
		code := `
			%intfn = %fn() %int
			return %intfn
		`

		ctx := NewDefaultTestContext()
		ctx.AddNamedPattern("int", DEFAULT_NAMED_PATTERNS["int"])
		state := NewGlobalState(ctx)
		res, err := Eval(code, state, true)
		assert.NoError(t, err)

		assert.IsType(t, &FunctionPattern{}, res)
	})

	t.Run("function pattern matching,", func(t *testing.T) {
		code := `
			fn f() %int { 
				return 1
			}
			%intfn = %fn() %int
			return (f match %intfn)
		`

		ctx := NewDefaultTestContext()
		ctx.AddNamedPattern("int", DEFAULT_NAMED_PATTERNS["int"])
		state := NewGlobalState(ctx)

		res, err := Eval(code, state, true)
		assert.NoError(t, err)

		assert.Equal(t, True, res)
	})

	t.Run("pattern conversion expression,", func(t *testing.T) {
		code := `%(1)`
		ctx := NewDefaultTestContext()
		state := NewGlobalState(ctx)

		res, err := Eval(code, state, true)
		assert.NoError(t, err)

		assert.Equal(t, NewExactValuePattern(Int(1)), res)
	})

	t.Run("pipeline statement", func(t *testing.T) {

		t.Run("pipeline statement", func(t *testing.T) {
			code := `get-data | split-lines $`
			var dollarVarValue Str
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"get-data": ValOf(func(ctx *Context) Str {
					return "aaa\nbbb"
				}),
				"split-lines": ValOf(func(ctx *Context, s Str) []Str {
					dollarVarValue = s
					return splitLines(ctx, s)
				}),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			if bytecodeEval {
				assert.Equal(t, Nil, res)
			} else {
				assert.Equal(t, NewWrappedValueList(Str("aaa"), Str("bbb")), res)
			}

			assert.Equal(t, Str("aaa\nbbb"), dollarVarValue)
		})

		t.Run("original value of anonymous variable is restored", func(t *testing.T) {
			code := `
				$ = 1
				get-data | split-lines $;
				return $
			`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"get-data": ValOf(func(ctx *Context) Str {
					return "aaa\nbbb"
				}),
				"split-lines": ValOf(splitLines),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(1), res)
		})

	})

	t.Run("pipeline expression", func(t *testing.T) {
		code := `
			result = | idt [1, "a", 2] | filter $ %int
			return result
		`
		state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
			"idt": ValOf(func(ctx *Context, v Value) Value {
				return v
			}),
			"filter": ValOf(Filter),
		})
		state.Ctx.AddNamedPattern("int", INT_PATTERN)

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, NewWrappedValueList(Int(1), Int(2)), res)
	})

	t.Run("member expression", func(t *testing.T) {
		testCases := []struct {
			error           bool
			input           string
			globalVariables map[string]Value
			result          Value
			pre             func(expected Value, actual Value, state *GlobalState)
		}{
			{
				input:  "$a = {v: 1}; return $a.v",
				result: Int(1),
			},
			{
				input:  "return ({a: 1}).a",
				result: Int(1),
			},
			{
				input: "return $$goval.secret",
				error: true,
				globalVariables: map[string]Value{
					"goval": ValOf(testMutableGoValue{Name: "Foo", secret: "secret"}),
				},
				result: Nil,
			},
			{
				input:  "return ({}).?a",
				result: Nil,
			},
			{
				input:  "$a = {v: 1}; return $a.(\"v\")",
				result: Int(1),
			},
			{
				input: `
					rt = go do {
						return {x: 1}
					}
		
					res = rt.wait_result!()
					return res.x
				`,
				result: Int(1),
			},
			{
				input: `
					rt = go do {
						return {x: {}}
					}
		
					res = rt.wait_result!()
					return res.x
				`,
				result: NewObject(),
				pre: func(expected, actual Value, state *GlobalState) {
					expected = Share(expected.(PotentiallySharable), state)
					actual.(SystemPart).DetachFromSystem()
				},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				state := NewGlobalState(NewDefaultTestContext())
				for k, v := range testCase.globalVariables {
					state.Globals.Set(k, v)
				}
				res, err := Eval(testCase.input, state, false)
				if testCase.error {
					assert.Error(t, err)
					assert.Nil(t, res)
				} else {
					assert.NoError(t, err)
					expected := testCase.result
					if testCase.pre != nil {
						testCase.pre(expected, res, state)
					}
					assert.Equal(t, expected, res)
				}
			})
		}
	})

	t.Run("dynamic member expression", func(t *testing.T) {

		testCases := []struct {
			error       bool
			input       string
			globals     map[string]Value
			result      Value
			makeResult  func(state *GlobalState) Value
			checkResult func(t *testing.T, state *GlobalState, actual Value)
		}{
			{
				input: "$a = {v: 1}; return $a.<v",
				checkResult: func(t *testing.T, state *GlobalState, actual Value) {
					if !assert.IsType(t, &DynamicValue{}, actual) {
						return
					}

					dyn := actual.(*DynamicValue)

					assert.Equal(t, map[string]Value{"v": Int(1)}, dyn.value.(*Object).EntryMap())
					assert.Equal(t, Str("v"), dyn.opData0)
				},
			},
			{
				input: "$a = {obj: {a: 1}}; return $a.<obj.a",
				checkResult: func(t *testing.T, state *GlobalState, actual Value) {
					if !assert.IsType(t, &DynamicValue{}, actual) {
						return
					}

					dyn := actual.(*DynamicValue)

					assert.Equal(t, map[string]Value{"a": Int(1)}, dyn.value.(*Object).EntryMap())
					assert.Equal(t, Str("a"), dyn.opData0)
				},
			},
			{
				input: "return ({a: 1}).<a",
				checkResult: func(t *testing.T, state *GlobalState, actual Value) {
					if !assert.IsType(t, &DynamicValue{}, actual) {
						return
					}

					dyn := actual.(*DynamicValue)

					assert.Equal(t, map[string]Value{"a": Int(1)}, dyn.value.(*Object).EntryMap())
					assert.Equal(t, Str("a"), dyn.opData0)
				},
			},
			{
				input: "return $$goval.<secret",
				error: true,
				globals: map[string]Value{
					"goval": ValOf(testMutableGoValue{Name: "Foo", secret: "secret"}),
				},
				result: Nil,
			},
			{
				input: `
					rt = go do {
						return {x: 1}
					}
		
					res = rt.wait_result!()
					return res.<x
				`,
				checkResult: func(t *testing.T, state *GlobalState, actual Value) {
					if !assert.IsType(t, &DynamicValue{}, actual) {
						return
					}

					dyn := actual.(*DynamicValue)

					assert.Equal(t, map[string]Value{"x": Int(1)}, dyn.value.(*Object).EntryMap())
					assert.Equal(t, Str("x"), dyn.opData0)
				},
			},
			{
				input: `
					rt = go do {
						return {x: {}}
					}
		
					res = rt.wait_result!()
					return res.<x
				`,
				checkResult: func(t *testing.T, state *GlobalState, actual Value) {
					if !assert.IsType(t, &DynamicValue{}, actual) {
						return
					}

					dyn := actual.(*DynamicValue)

					innerObj := NewObjectFromMap(nil, state.Ctx)
					innerObj.Share(state)

					assert.Equal(t, map[string]Value{"x": innerObj}, dyn.value.(*Object).EntryMap())
					assert.Equal(t, Str("x"), dyn.opData0)
				},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				state := NewGlobalState(NewDefaultTestContext())
				for k, v := range testCase.globals {
					state.Globals.Set(k, v)
				}

				res, err := Eval(testCase.input, state, false)
				if testCase.error {
					assert.Error(t, err)
					assert.Nil(t, res)
				} else {
					assert.NoError(t, err)
					expected := testCase.result
					if testCase.checkResult != nil {
						testCase.checkResult(t, state, res)
					} else {
						if testCase.makeResult != nil {
							expected = testCase.makeResult(state)
						}
						assert.Equal(t, expected, res)
					}
				}
			})
		}
	})

	t.Run("extraction expression", func(t *testing.T) {
		code := `return ({a:1}).{a}`
		state := NewGlobalState(NewDefaultTestContext())
		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, objFrom(ValMap{"a": Int(1)}), res)
	})

	t.Run("key list expression", func(t *testing.T) {

		t.Run("empty", func(t *testing.T) {
			code := `return .{}`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, KeyList{}, res)
		})

		t.Run("single element", func(t *testing.T) {
			code := `return .{name}`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, KeyList{"name"}, res)
		})

	})

	t.Run("lazy expression : @ <integer>", func(t *testing.T) {
		code := `@(1)`
		state := NewGlobalState(NewDefaultTestContext())
		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.EqualValues(t, AstNode{
			Node: &parse.IntLiteral{
				NodeBase: parse.NodeBase{
					Span: parse.NodeSpan{Start: 2, End: 3},
					ValuelessTokens: []parse.Token{
						{Type: parse.OPENING_PARENTHESIS, Span: parse.NodeSpan{Start: 1, End: 2}},
						{Type: parse.CLOSING_PARENTHESIS, Span: parse.NodeSpan{Start: 3, End: 4}},
					},
				},
				Raw:   "1",
				Value: 1,
			},
			chunk: state.Module.MainChunk,
		}, res)
	})

	t.Run("inclusion import statement", func(t *testing.T) {

		t.Run("single included file with no dependecies", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return a
			`, map[string]string{"./dep.ix": "a = 1"})

			mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := NewGlobalState(NewDefaultTestContext())
			state.Module = mod
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(1), res)
		})

		t.Run("single included file which itself includes a file", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep2.ix
				return a
			`, map[string]string{
				"./dep2.ix": `
					import ./dep1.ix
				`,
				"./dep1.ix": "a = 1",
			})

			mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := NewGlobalState(NewDefaultTestContext())
			state.Module = mod
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(1), res)
		})

		t.Run("two included files with no dependecies", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep1.ix
				import ./dep2.ix
				return (a + b)
			`, map[string]string{
				"./dep1.ix": "a = 1",
				"./dep2.ix": "b = 2",
			})

			mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := NewGlobalState(NewDefaultTestContext())
			state.Module = mod
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(3), res)
		})

		t.Run("single included file accessing a global", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return a
			`, map[string]string{"./dep.ix": "a = myglobal"})

			mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"myglobal": Int(1),
			})
			state.Module = mod
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(1), res)
		})

		t.Run("single included file accessing a global in a function", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return f()
			`, map[string]string{"./dep.ix": `
				fn f(){
					return myglobal
				}
			`})

			mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"myglobal": Int(1),
			})
			state.Module = mod
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(1), res)
		})

		t.Run("single included file accessing a global in a lifetime job", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				sleep 10ms
				return obj.a
			`, map[string]string{"./dep.ix": `
				obj = {
					a: 0
					lifetimejob #job {
						self.a = myglobal
					}
				}
			`})

			mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"myglobal": Int(1),
				"sleep":    WrapGoFunction(Sleep),
			})
			state.Module = mod
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(1), res)
		})

		t.Run("included file defining a pattern", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return %p
			`, map[string]string{"./dep.ix": "%p = %str"})

			mod, err := ParseLocalModule(LocalModuleParsingConfig{ModuleFilepath: modpath, Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := NewGlobalState(NewDefaultTestContext())
			state.Ctx.AddNamedPattern("str", STR_PATTERN)
			state.Module = mod
			res, err := Eval(mod, state, false)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, STR_PATTERN, res)
		})
	})

	t.Run("import statement", func(t *testing.T) {
		t.Run("no globals, no required permissions", func(t *testing.T) {
			code := strings.ReplaceAll(`
				import importname https://modules.com/return_1.ix {
					validation: "<hash>"
				}
				return $$importname
			`, "<hash>", RETURN_1_MODULE_HASH)
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, Int(1), res)
		})

		t.Run("imported module returns the positional 'a' argument", func(t *testing.T) {
			code := strings.ReplaceAll(`
				import importname https://modules.com/return_global_a.ix {
					validation: "<hash>"
					arguments: {1}
				}
				return $$importname
			`, "<hash>", RETURN_POS_ARG_A_MODULE_HASH)

			ctx := NewDefaultTestContext()
			ctx.AddNamedPattern("int", INT_PATTERN)

			state := NewGlobalState(ctx)
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, Int(1), res)
		})

		t.Run("imported module returns the non-positional 'a' argument", func(t *testing.T) {
			code := strings.ReplaceAll(`
				import importname https://modules.com/return_global_a.ix {
					validation: "<hash>"
					arguments: {a: 1}
				}
				return $$importname
			`, "<hash>", RETURN_NON_POS_ARG_A_MODULE_HASH)

			ctx := NewDefaultTestContext()
			ctx.AddNamedPattern("int", INT_PATTERN)

			state := NewGlobalState(ctx)
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, Int(1), res)
		})

		t.Run("imported module returns the %two pattern (same pattern is defined in module)", func(t *testing.T) {
			code := strings.ReplaceAll(`
				%two = 1

				import two_patt https://modules.com/return_global_a.ix {
					validation: "<hash>"
					arguments: {}
				}
				return $$two_patt
			`, "<hash>", RETURN_PATTERN_INT_TWO_MODULE_HASH)

			ctx := NewDefaultTestContext()
			ctx.AddNamedPattern("int", INT_PATTERN)

			state := NewGlobalState(ctx)
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, NewExactValuePattern(Int(2)), res)
		})

		t.Run("imported module returns the %two pattern", func(t *testing.T) {
			code := strings.ReplaceAll(`
				import two_patt https://modules.com/return_global_a.ix {
					validation: "<hash>"
					arguments: {}
				}
				return $$two_patt
			`, "<hash>", RETURN_PATTERN_INT_TWO_MODULE_HASH)

			ctx := NewDefaultTestContext()
			ctx.AddNamedPattern("int", INT_PATTERN)

			state := NewGlobalState(ctx)
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, NewExactValuePattern(Int(2)), res)
		})

		t.Run("imported module returns the %int pattern (base pattern)", func(t *testing.T) {
			code := strings.ReplaceAll(`
				import int_pattern https://modules.com/return_global_a.ix {
					validation: "<hash>"
					arguments: {}
				}
				return $$int_pattern
			`, "<hash>", RETURN_INT_PATTERN_MODULE_HASH)

			ctx := NewDefaultTestContext()
			ctx.AddNamedPattern("int", INT_PATTERN)

			state := NewGlobalState(ctx)
			//we copy the pattern in order to later check that the importer's pattern is not passed to the imported module.
			intPatternCopy := *INT_PATTERN

			state.GetBasePatternsForImportedModule = func() (map[string]Pattern, map[string]*PatternNamespace) {
				return map[string]Pattern{"int": &intPatternCopy}, nil
			}

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Same(t, &intPatternCopy, res)
		})

		t.Run("local module", func(t *testing.T) {
			code := strings.ReplaceAll(`
				import importname ./return_a.ix  {
					validation: "<hash>"
					arguments: {a: 1}
				}
				return $$importname
			`, "<hash>", RETURN_NON_POS_ARG_A_MODULE_HASH)

			ctx := NewContext(ContextConfig{
				Permissions: []Permission{
					GlobalVarPermission{permkind.Read, "*"},
					GlobalVarPermission{permkind.Update, "*"},
					GlobalVarPermission{permkind.Create, "*"},
					GlobalVarPermission{permkind.Use, "*"},

					FilesystemPermission{permkind.Read, PathPattern("/...")},
					RoutinePermission{permkind.Create},
				},
				Filesystem: newOsFilesystem(),
			})
			ctx.AddNamedPattern("int", INT_PATTERN)
			state := NewGlobalState(ctx)

			state.Module = &Module{
				MainChunk: &parse.ParsedChunk{
					Source: parse.SourceFile{Resource: "/mytest", ResourceDir: "/", NameString: "/mytest"},
				},
			}
			state.GetBasePatternsForImportedModule = func() (map[string]Pattern, map[string]*PatternNamespace) {
				return DEFAULT_NAMED_PATTERNS, DEFAULT_PATTERN_NAMESPACES
			}

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, Int(1), res)
		})
	})

	t.Run("spawn expression", func(t *testing.T) {
		t.Run("single expression", func(t *testing.T) {
			code := `
				fn f(){
					return 1
				}
				routine = go do f()
				return routine.wait_result!()
			`
			state := NewGlobalState(NewDefaultTestContext())
			state.Logger = zerolog.New(state.Out)
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(1), res)
		})

		t.Run("empty embedded module", func(t *testing.T) {
			code := `
				go do { }
			`
			state := NewGlobalState(NewDefaultTestContext())
			_, err := Eval(code, state, false)
			assert.NoError(t, err)
		})

		t.Run("embedded module returns a simple value", func(t *testing.T) {
			code := `
				rt = go do { 
					return 1
				}
	
				return rt.wait_result!()
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(1), res)
		})

		t.Run("embedded module returns a data structure", func(t *testing.T) {
			code := `
				rt = go do { 
					return { }
				}
	
				return rt.wait_result!()
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.IsType(t, &Object{}, res)
			assert.True(t, res.(*Object).IsShared())
		})

		t.Run("allow <runtime manifest>", func(t *testing.T) {
			code := `
				$$URL = https://example.com/
				rt = go {allow: {read: URL}} do { 
	
				}
	
				rt.wait_result!()
			`
			state := NewGlobalState(NewDefaultTestContext())
			_, err := Eval(code, state, false)
			assert.NoError(t, err)
		})

		t.Run("pass an additional global to a single expression embedded module", func(t *testing.T) {
			code := `
				rt = go {globals: {b: 2}} do idt(b)
				return rt.wait_result!()
			`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": WrapGoFunction(func(ctx *Context, arg Value) Value {
					return arg
				}),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(2), res)
		})

		t.Run("pass an additional global to a embedded module (block)", func(t *testing.T) {
			code := `
				rt = go {globals: {b: 2}} do { 
					return b
				}
	
				return rt.wait_result!()
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(2), res)
		})

		t.Run("group (used once)", func(t *testing.T) {
			code := `
				group = RoutineGroup()
				go {group: group} do { }
	
				return group
			`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"RoutineGroup": WrapGoFunction(NewRoutineGroup),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.IsType(t, &RoutineGroup{}, res)
			assert.Len(t, res.(*RoutineGroup).routines, 1)
		})

		t.Run("group (used twice)", func(t *testing.T) {
			code := `
				group = RoutineGroup()
				go {group: group} do { }
				go {group: group} do { }
	
				return group
			`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"RoutineGroup": WrapGoFunction(NewRoutineGroup),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.IsType(t, &RoutineGroup{}, res.(GoValue))

			assert.Len(t, res.(*RoutineGroup).routines, 2)
		})

		t.Run("call passed Inox function", func(t *testing.T) {
			code := `
				fn f(){
					return 2
				}
				rt = go {globals: {f: f}} do {
					return f() # f is external for the embedded module
				}
	
				return rt.wait_result!()
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(2), res)
		})

		t.Run("call passed Inox function that access a captured global", func(t *testing.T) {
			code := `
				$$a = 1
				fn f(){
					return a
				}
				rt = go {globals: {f: f}} do {
					$$a = 2

					return f()
				}
	
				return rt.wait_result!()
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, true)
			assert.NoError(t, err)
			assert.Equal(t, Int(1), res)
		})

		t.Run("call a function accessing a global variable within a passed Inox function that captured this global", func(t *testing.T) {
			code := `
				$$a = 1
				fn f(){
					b = a
					func = fn(){
						return a
					}
					return func()
				}
				rt = go {globals: {f: f}} do {
					$$a = 2

					return f()
				}
	
				return rt.wait_result!()
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, true)
			assert.NoError(t, err)
			assert.Equal(t, Int(1), res)
		})

		t.Run("compute a Mapping entry that access a captured global", func(t *testing.T) {
			code := `
				$$a = 1
				mapping = Mapping {
					%/... => a
				}
				rt = go {globals: {m: mapping}} do {
					$$a = 2

					return m.compute(/)
				}
	
				return rt.wait_result!()
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, true)
			assert.NoError(t, err)
			assert.Equal(t, Int(1), res)
		})

		//TODO: add more tests on global capture

		t.Run("call passed Go func", func(t *testing.T) {
			called := false
			code := `
				group = RoutineGroup()
				rt = go {group: group} do gofunc()
	
				return rt.wait_result!()
			`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"gofunc": ValOf(func(ctx *Context) Int {
					called = true
					return 2
				}),
				"RoutineGroup": WrapGoFunction(NewRoutineGroup),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.True(t, called)
			assert.Equal(t, Int(2), res)
		})

		t.Run("spawner & routine access a shared value in a synchronized block", func(t *testing.T) {
			goroutineIncCount := 5_000
			if bytecodeEval {
				goroutineIncCount = 50_000
			}

			code := strings.ReplaceAll(`
				shared = {a: 0}
				rt = go {globals: {shared: shared}} do {
					for i in 1..<count> {
						synchronized(shared) {
							shared.a += 1
						}
					}
				}

				sleep;

				for i in 1..<count> {
					synchronized(shared) {
						shared.a += 1
					}
				}

				rt.wait_result!()
				return shared.a
			`, "<count>", strconv.Itoa(goroutineIncCount))
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"sleep": ValOf(func(ctx *Context) {
					time.Sleep(time.Millisecond)
				}),
			})
			state.Logger = zerolog.New(state.Out)
			state.Out = os.Stdout

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(2*goroutineIncCount), res)
		})

		t.Run("spawner & routine access a shared value without synchronization", func(t *testing.T) {
			goroutineIncCount := 5_000
			if bytecodeEval {
				goroutineIncCount = 50_000
			}

			code := strings.ReplaceAll(`
				shared = {a: 0}
				rt = go {globals: {shared: shared}} do {
					for i in 1..<count> {
						shared.a += 1
					}
				}

				sleep;

				for i in 1..<count> {
					shared.a += 1
				}

				rt.wait_result!()
			`, "<count>", strconv.Itoa(goroutineIncCount))
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"sleep": ValOf(func(ctx *Context) {
					time.Sleep(time.Millisecond)
				}),
			})
			state.Logger = zerolog.New(state.Out)
			state.Out = os.Stdout

			timedOut := atomic.Bool{}

			// timeout
			go func() {
				<-time.After(10 * time.Second)
				state.Ctx.Cancel()
				timedOut.Store(true)
			}()

			_, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.False(t, timedOut.Load())
		})

		t.Run("embedded module yields once and has no return statement", func(t *testing.T) {
			code := `
				rt = go do { 
					yield 0
				}
	
				result = rt.wait_result!()

				step_results = []
				for step in rt.steps {
					append(step_results, step.result)
				}
				return [result, step_results]
			`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"append": ValOf(Append),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, NewWrappedValueList(
				Nil,
				NewWrappedValueList(Int(0)),
			), res)
		})

		t.Run("embedded module yields twice and has no return statement", func(t *testing.T) {
			code := `
				rt = go do { 
					yield 0

					yield 1
				}
	
				result = rt.wait_result!()

				step_results = []
				for step in rt.steps {
					append(step_results, step.result)
				}
				return [result, step_results]
			`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"append": ValOf(Append),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, NewWrappedValueList(
				Nil,
				NewWrappedValueList(Int(0), Int(1)),
			), res)
		})

		t.Run("embedded module yields once and has a return statement", func(t *testing.T) {
			code := `
				rt = go do { 
					yield 0
					return "final result"
				}
	
				result = rt.wait_result!()

				step_results = []
				for step in rt.steps {
					append(step_results, step.result)
				}
				return [result, step_results]
			`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"append": ValOf(Append),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, NewWrappedValueList(
				Str("final result"),
				NewWrappedValueList(Int(0)),
			), res)
		})

		t.Run("patterns declared by an embedded module should not be declared in top level module's context", func(t *testing.T) {
			code := `
				%p1 = 1
				rt = go {} do {
					%p2 = 2
				}
	
				rt.wait_result!()
			`
			state := NewGlobalState(NewDefaultTestContext())
			_, err := Eval(code, state, false)
			assert.NoError(t, err)

			assert.NotNil(t, state.Ctx.ResolveNamedPattern("p1"))
			assert.Nil(t, state.Ctx.ResolveNamedPattern("p2"))
		})
	})

	t.Run("mapping expression", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			code := `Mapping{}`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, &Mapping{
				keys:                         map[string]Value{},
				preComputedStaticEntryValues: map[string]Value{},
				dynamicEntries:               map[string]*parse.DynamicMappingEntry{},
				patterns: []struct {
					string
					Pattern
				}{},
			}, res)
		})

		t.Run("not empty", func(t *testing.T) {
			code := `Mapping{ 
				0 => 1  
				1 => f()
				n 2 => n 
			}`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"f": WrapGoFunction(func(ctx *Context) Int {
					return -1
				}),
			})
			res, err := Eval(code, state, false)
			mod := parse.MustParseChunk(code)

			assert.NoError(t, err)
			assert.Equal(t, &Mapping{
				keys: map[string]Value{
					"0": Int(0),
					"1": Int(1),
					"2": Int(2),
				},
				preComputedStaticEntryValues: map[string]Value{
					"0": Int(1),
				},
				staticEntries: map[string]*parse.StaticMappingEntry{
					"1": parse.FindNode(mod, &parse.StaticMappingEntry{}, nil),
				},
				dynamicEntries: map[string]*parse.DynamicMappingEntry{
					"2": parse.FindNode(mod, &parse.DynamicMappingEntry{}, nil),
				},
				patterns: []struct {
					string
					Pattern
				}{},
			}, res)
		})

		t.Run("pattern identifier keys", func(t *testing.T) {
			code := `Mapping{ %str => 1  n %int => n }`
			state := NewGlobalState(NewDefaultTestContext())
			state.Ctx.AddNamedPattern("str", STR_PATTERN)
			state.Ctx.AddNamedPattern("int", INT_PATTERN)

			res, err := Eval(code, state, false)
			mod := parse.MustParseChunk(code)

			assert.NoError(t, err)
			assert.Equal(t, &Mapping{
				keys: map[string]Value{
					"%str": STR_PATTERN,
					"%int": INT_PATTERN,
				},
				preComputedStaticEntryValues: map[string]Value{
					"%str": Int(1),
				},
				dynamicEntries: map[string]*parse.DynamicMappingEntry{
					"%int": parse.FindNode(mod, &parse.DynamicMappingEntry{}, nil),
				},
				patterns: []struct {
					string
					Pattern
				}{
					{"%str", STR_PATTERN},
					{"%int", INT_PATTERN},
				},
			}, res)
		})

		t.Run("should not be sharable if one of the captured globals is not sharable", func(t *testing.T) {
			code := `
				$$a = 1
				return Mapping{ 
					0 => notsharable  
					1 => a
				}
			`
			state := NewGlobalState(NewDefaultTestContext())
			state.Globals.Set("notsharable", testMutableGoValue{})

			res, err := Eval(code, state, true)

			if !assert.NoError(t, err) {
				return
			}
			assert.False(t, utils.Ret0(res.(*Mapping).IsSharable(state)))
		})

		t.Run("should be sharable if all of the captured globals are sharable", func(t *testing.T) {
			code := `
				$$a = 1
				$$b = 2
				return Mapping{ 
					0 => a
					1 => b
				}
			`
			state := NewGlobalState(NewDefaultTestContext())
			state.Globals.Set("notsharable", testMutableGoValue{})

			res, err := Eval(code, state, true)

			if !assert.NoError(t, err) {
				return
			}
			assert.True(t, utils.Ret0(res.(*Mapping).IsSharable(state)))
		})

	})

	t.Run("udata literal", func(t *testing.T) {

		t.Run("not empty", func(t *testing.T) {
			code := `udata 0 { 1 {2} 3 }`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &UData{
				Root: Int(0),
				HiearchyEntries: []UDataHiearchyEntry{
					{
						Value: Int(1),
						Children: []UDataHiearchyEntry{
							{Value: Int(2)},
						},
					},
					{
						Value: Int(3),
					},
				},
			}, res)
		})
	})

	t.Run("Mapping", func(t *testing.T) {
		t.Run("compute() with inexisting key", func(t *testing.T) {
			code := `
				m = Mapping{}
				return m.compute(0)
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Nil, res)
		})

		t.Run("compute() with existing static entry", func(t *testing.T) {
			code := `
				m = Mapping{0 => 1}
				return m.compute(0)
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(1), res)
		})

		t.Run("compute() with existing dynamic value static key entry", func(t *testing.T) {
			code := `
				$$v = "a"
				m = Mapping{0 => v}
				return m.compute(0)
			`
			state := NewGlobalState(NewDefaultTestContext())

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Str("a"), res)
		})

		t.Run("compute() with existing dynamic entry key", func(t *testing.T) {
			code := `
				m = Mapping{ n 0 => n}
				return m.compute(0)
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(0), res)
		})

		t.Run("compute() with existing dynamic entry key & group matching var", func(t *testing.T) {
			code := `
				m = Mapping{ p %/{:name} m => [p, m] }
				return m.compute(/a)
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, NewWrappedValueList(
				Path("/a"),
				NewObjectFromMap(ValMap{
					"0":    Path("/a"),
					"name": Str("a"),
				}, state.Ctx),
			), res)
		})

		t.Run("compute() with existing dynamic entry key in many goroutines", func(t *testing.T) {
			code := `
				$$m = Mapping{ n 0 => n }

				group = RoutineGroup()

				for 1..10_000 {
					go {globals: .{m}, group: group} do {
						return m.compute(0)
					}
				}

				return group.wait_results!()
			`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"RoutineGroup": WrapGoFunction(NewRoutineGroup),
			})
			state.Out = os.Stdout
			state.Logger = zerolog.New(state.Out)

			res, err := Eval(code, state, true)
			if !assert.NoError(t, err) {
				return
			}

			assert.IsType(t, &List{}, res)
			for _, e := range res.(*List).GetOrBuildElements(state.Ctx) {
				if !assert.Equal(t, Int(0), e) {
					return
				}
			}
		})

		t.Run("compute() with existing dynamic entry key (accessing a global variable) in many goroutines", func(t *testing.T) {
			code := `
				$$a = 1
				$$m = Mapping{ n 0 => a }

				group = RoutineGroup()

				for 1..10_000 {
					go {globals: .{m}, group: group} do {
						return m.compute(0)
					}
				}

				return group.wait_results!()
			`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"RoutineGroup": WrapGoFunction(NewRoutineGroup),
			})

			state.Out = os.Stdout
			state.Logger = zerolog.New(state.Out)

			res, err := Eval(code, state, true)
			if !assert.NoError(t, err) {
				return
			}

			assert.IsType(t, &List{}, res)
			for _, e := range res.(*List).GetOrBuildElements(state.Ctx) {
				if !assert.Equal(t, Int(1), e) {
					return
				}
			}
		})

		t.Run("compute() with key matching 2 patterns: the first one should be selected", func(t *testing.T) {
			code := `
				m = Mapping{ 
					%/ => 0
					%/... => 1
				}
				return m.compute(/)
			`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, Int(0), res)
		})

	})

	t.Run("concatenation expression", func(t *testing.T) {
		t.Run("single string element", func(t *testing.T) {
			code := `concat "a"`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, Str("a"), res)
		})

		t.Run("two string-like elements", func(t *testing.T) {
			code := `concat "a" "b"`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &StringConcatenation{
				elements: []StringLike{Str("a"), Str("b")},
				totalLen: 2,
			}, res)
		})

		t.Run("single byteslice element", func(t *testing.T) {
			code := `concat 0d[12]`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &ByteSlice{IsDataMutable: false, Bytes: []byte{12}}, res)
		})

		t.Run("two bytes-like elements", func(t *testing.T) {
			code := `concat 0d[12] 0d[34]`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{})
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &BytesConcatenation{
				elements: []BytesLike{
					&ByteSlice{IsDataMutable: false, Bytes: []byte{12}},
					&ByteSlice{IsDataMutable: false, Bytes: []byte{34}},
				},
				totalLen: 2,
			}, res)
		})

		t.Run("modifying an element of the concatenation should not change the concatenation value", func(t *testing.T) {
			code := `
				bytes = 0d[12]
				c = concat bytes 0d[34]
				bytes[0] = tobyte(24)
				return c
			`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"tobyte": WrapGoFunction(func(ctx *Context, i Int) Byte {
					return Byte(i)
				}),
			})
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &BytesConcatenation{
				elements: []BytesLike{
					&ByteSlice{IsDataMutable: false, Bytes: []byte{12}},
					&ByteSlice{IsDataMutable: false, Bytes: []byte{34}},
				},
				totalLen: 2,
			}, res)
		})

		t.Run("two tuples", func(t *testing.T) {
			code := `concat #[1] #["a"]`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{})
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, NewTuple([]Value{Int(1), Str("a")}), res)
		})

		t.Run("string element followed by a spread element with a single item", func(t *testing.T) {
			code := `concat "a" ...["b"]`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, "ab", res.(*StringConcatenation).GetOrBuildString())
		})

		t.Run("string element followed by a spread element with two items", func(t *testing.T) {
			code := `concat "a" ...["b", "c"]`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, "abc", res.(*StringConcatenation).GetOrBuildString())
		})

		t.Run("alternation of normal & spread elements", func(t *testing.T) {
			code := `concat "a" ...["b", "c"] "d" ...["e", "f"]`
			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, "abcdef", res.(*StringConcatenation).GetOrBuildString())
		})
	})

	t.Run("a value passed to a routine and then returned by it should not be wrapped", func(t *testing.T) {
		called := false

		code := `
			rt = go {globals: {gofunc: $$gofunc}} do {
				return $$gofunc
			}

			f = rt.wait_result!()
			return f()
		`

		_ctx := NewDefaultTestContext()
		state := NewGlobalState(_ctx, map[string]Value{
			"gofunc": ValOf(func(ctx *Context) Int {
				called = true

				if ctx != _ctx {
					t.Fatal("the context should be the main one")
				}
				return 0
			}),
		})
		_, err := Eval(code, state, false)
		assert.True(t, called)
		assert.NoError(t, err)
	})

	t.Run("dropped permissions", func(t *testing.T) {
		code := `
			drop-perms {
				read: {
					globals: "*"
				}
			}
		`

		state := NewGlobalState(NewDefaultTestContext())
		_, err := Eval(code, state, false)

		assert.True(t, state.Ctx.HasPermission(GlobalVarPermission{Kind_: permkind.Use, Name: "*"}))
		assert.False(t, state.Ctx.HasPermission(GlobalVarPermission{Kind_: permkind.Read, Name: "*"}))

		assert.NoError(t, err)
	})

	t.Run("host alias definition", func(t *testing.T) {
		code := `@localhost = https://localhost`

		state := NewGlobalState(NewDefaultTestContext())
		res, err := Eval(code, state, false)

		assert.NoError(t, err)
		assert.Equal(t, Nil, res)
		assert.Equal(t, Host("https://localhost"), state.Ctx.ResolveHostAlias("localhost"))
	})

	t.Run("pattern definition", func(t *testing.T) {

		t.Run("identifier : RHS is a string literal", func(t *testing.T) {
			code := `%s = "s"; return %s`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, NewExactStringPattern(Str("s")), res)
		})

		t.Run("RHS is an unprefixed object pattern", func(t *testing.T) {
			code := `%o = {}; return %o`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, NewInexactObjectPattern(map[string]Pattern{}), res)
		})

		t.Run("RHS is a prefixed object pattern", func(t *testing.T) {
			code := `%o = %{}; return %o`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, NewInexactObjectPattern(map[string]Pattern{}), res)
		})

		t.Run("RHS is an unprefixed object pattern with a unprefixed property pattern", func(t *testing.T) {
			code := `%o = {a: int}; return %o`

			state := NewGlobalState(NewDefaultTestContext())
			state.Ctx.AddNamedPattern("int", INT_PATTERN)
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, NewInexactObjectPattern(map[string]Pattern{
				"a": INT_PATTERN,
			}), res)
		})

		t.Run("pattern definition & identifiers : RHS is another pattern identifier", func(t *testing.T) {
			code := `%p = "p"; 
			%s = %p; 
			return %s`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, NewExactStringPattern(Str("p")), res)
		})

		t.Run("pattern definition & identifiers : minimal lazy", func(t *testing.T) {
			code := `
				%s = @ %p
				prev = %s
				%p = "p"
				return [$prev, %s]
			`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			prev := res.(*List).At(state.Ctx, 0)
			pattern := res.(*List).At(state.Ctx, 1)
			assert.IsType(t, (*DynamicStringPatternElement)(nil), prev)
			assert.Equal(t, &DynamicStringPatternElement{"p", state.Ctx}, pattern)
		})

		t.Run("pattern definition & identifiers : lazy", func(t *testing.T) {
			code := `
				%s = @ %str( %p "a" )
				prev = %s
				%p = "p"
				return [$prev, %s]
			`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			prev := res.(*List).At(state.Ctx, 0)
			pattern := res.(*List).At(state.Ctx, 1)
			assert.IsType(t, (*SequenceStringPattern)(nil), prev)
			assert.Same(t, prev, pattern)
		})

		t.Run("pattern definition: sequence string pattern: single element", func(t *testing.T) {
			code := `
				%s = %str( 'a'..'z' )
				return %s
			`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.IsType(t, (*SequenceStringPattern)(nil), res)
			patt := res.(*SequenceStringPattern)
			assert.Len(t, patt.elements, 1)
		})

	})

	t.Run("pattern namespace definition", func(t *testing.T) {

		t.Run("RHS is an empty object literal", func(t *testing.T) {
			code := `%namespace. = {}`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, Nil, res)

			assert.Equal(t, map[string]*PatternNamespace{
				"namespace": {
					Patterns: map[string]Pattern{},
				},
			}, state.Ctx.GetPatternNamespaces())
		})

		t.Run("RHS is an object literal with patterns & non-pattern values", func(t *testing.T) {
			code := `%namespace. = {one: 1, empty_obj: %{}}`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, Nil, res)

			assert.Equal(t, map[string]*PatternNamespace{
				"namespace": {
					Patterns: map[string]Pattern{
						"one": &ExactValuePattern{value: Int(1)},
						"empty_obj": &ObjectPattern{
							entryPatterns: map[string]Pattern{},
							inexact:       true,
						},
					},
				},
			}, state.Ctx.GetPatternNamespaces())
		})
	})

	t.Run("pattern namespace member", func(t *testing.T) {
		code := `
			%namespace. = {one: 1}
			return %namespace.one
		`
		state := NewGlobalState(NewDefaultTestContext())
		res, err := Eval(code, state, false)

		assert.NoError(t, err)
		assert.Equal(t, &ExactValuePattern{value: Int(1)}, res)
	})

	t.Run("object pattern literal", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			code := `%{}`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &ObjectPattern{
				inexact:       true,
				entryPatterns: map[string]Pattern{},
			}, res)
		})

		t.Run("not empty", func(t *testing.T) {
			code := `%s = "s"; return %{name: %s, count: 2}`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &ObjectPattern{
				inexact: true,
				entryPatterns: map[string]Pattern{
					"name":  NewExactStringPattern(Str("s")),
					"count": &ExactValuePattern{value: Int(2)},
				},
			}, res)
		})

		t.Run("unprefixed named pattern", func(t *testing.T) {
			code := `%s = "s"; return %{name: s}`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &ObjectPattern{
				inexact: true,
				entryPatterns: map[string]Pattern{
					"name": NewExactStringPattern(Str("s")),
				},
			}, res)
		})

		t.Run("spread", func(t *testing.T) {

			//TODO: add tests with several spread

			t.Run("single-property object pattern after properties", func(t *testing.T) {
				code := `
					%s = "s"
					%user = %{name: "foo"}
					return %{s: %s, ...%user}
				`

				state := NewGlobalState(NewDefaultTestContext())
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, &ObjectPattern{
					inexact: true,
					entryPatterns: map[string]Pattern{
						"s":    NewExactStringPattern(Str("s")),
						"name": NewExactStringPattern(Str("foo")),
					},
				}, res)
			})

			t.Run("single-optional-property object pattern after properties", func(t *testing.T) {
				code := `
					%s = "s"
					%user = %{name?: "foo"}
					return %{s: %s, ...%user}
				`

				state := NewGlobalState(NewDefaultTestContext())
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, &ObjectPattern{
					inexact: true,
					entryPatterns: map[string]Pattern{
						"s":    NewExactStringPattern(Str("s")),
						"name": NewExactStringPattern(Str("foo")),
					},
					optionalEntries: map[string]struct{}{"name": {}},
				}, res)
			})

			t.Run("two-property object pattern after properties", func(t *testing.T) {
				code := `
					%s = "s"
					%user = %{name: "foo", age: 30}
					return %{s: %s, ...%user}
				`

				state := NewGlobalState(NewDefaultTestContext())
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, &ObjectPattern{
					inexact: true,
					entryPatterns: map[string]Pattern{
						"s":    NewExactStringPattern(Str("s")),
						"name": NewExactStringPattern(Str("foo")),
						"age":  &ExactValuePattern{value: Int(30)},
					},
				}, res)
			})

			t.Run("single-property object pattern before properties", func(t *testing.T) {
				code := `
					%s = "s"
					%user = %{name: "foo"}
					return %{...%user, s: %s}
				`

				state := NewGlobalState(NewDefaultTestContext())
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, &ObjectPattern{
					inexact: true,
					entryPatterns: map[string]Pattern{
						"s":    NewExactStringPattern(Str("s")),
						"name": NewExactStringPattern(Str("foo")),
					},
				}, res)
			})

			t.Run("two-property object pattern before properties", func(t *testing.T) {
				code := `
					%s = "s"
					%user = %{name: "foo", age: 30}
					return %{...%user, s: %s}
				`

				state := NewGlobalState(NewDefaultTestContext())
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, &ObjectPattern{
					inexact: true,
					entryPatterns: map[string]Pattern{
						"s":    NewExactStringPattern(Str("s")),
						"name": NewExactStringPattern(Str("foo")),
						"age":  &ExactValuePattern{value: Int(30)},
					},
				}, res)
			})

			t.Run("complex", func(t *testing.T) {
				code := `
					%s = "s"
					%user = %{name: "foo"}
					return %{...%user, friends: %[]%user}
				`

				state := NewGlobalState(NewDefaultTestContext())
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, &ObjectPattern{
					inexact: true,
					entryPatterns: map[string]Pattern{
						"friends": NewListPatternOf(&ObjectPattern{
							entryPatterns: map[string]Pattern{
								"name": NewExactStringPattern(Str("foo")),
							},
							inexact: true,
						}),
						"name": NewExactStringPattern(Str("foo")),
					},
				}, res)
			})

			t.Run("spread element is not an object pattern", func(t *testing.T) {
				code := `%s = "s"; return %{...%s}`

				state := NewGlobalState(NewDefaultTestContext())
				res, err := Eval(code, state, false)

				assert.Error(t, err)
				assert.Nil(t, res)
			})
		})

	})

	t.Run("list pattern literal", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			code := `%[]`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &ListPattern{
				elementPatterns: make([]Pattern, 0),
			}, res)
		})

		t.Run("single element: integer literal", func(t *testing.T) {
			code := `%[ 2 ]`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &ListPattern{
				elementPatterns: []Pattern{
					&ExactValuePattern{value: Int(2)},
				},
			}, res)
		})

		t.Run("single element: empty object pattern", func(t *testing.T) {
			code := `%[ {} ]`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &ListPattern{
				elementPatterns: []Pattern{
					NewInexactObjectPattern(map[string]Pattern{}),
				},
			}, res)
		})

		t.Run("single element: empty object", func(t *testing.T) {
			code := `%[ %(#{}) ]`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &ListPattern{
				elementPatterns: []Pattern{
					NewExactValuePattern(NewEmptyRecord()),
				},
			}, res)
		})

		t.Run("single element: an object pattern literal", func(t *testing.T) {
			code := `%[ %{} ]`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &ListPattern{
				elementPatterns: []Pattern{
					&ObjectPattern{
						inexact:       true,
						entryPatterns: map[string]Pattern{},
					},
				},
			}, res)
		})

		t.Run("general element is an object pattern literal", func(t *testing.T) {
			code := `%[]%{}`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &ListPattern{
				elementPatterns: nil,
				generalElementPattern: &ObjectPattern{
					inexact:       true,
					entryPatterns: map[string]Pattern{},
				},
			}, res)
		})

		t.Run("general element is an unprefixed object pattern literal", func(t *testing.T) {
			code := `%[]%{}`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &ListPattern{
				elementPatterns: nil,
				generalElementPattern: &ObjectPattern{
					inexact:       true,
					entryPatterns: map[string]Pattern{},
				},
			}, res)
		})

		t.Run("general element is an unprefixed named pattern", func(t *testing.T) {
			code := `%[]int`

			state := NewGlobalState(NewDefaultTestContext())
			state.Ctx.AddNamedPattern("int", INT_PATTERN)
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &ListPattern{
				elementPatterns:       nil,
				generalElementPattern: INT_PATTERN,
			}, res)
		})
	})

	t.Run("union pattern", func(t *testing.T) {
		code := `%| 1 | 2`

		state := NewGlobalState(NewDefaultTestContext())
		res, err := Eval(code, state, false)

		assert.NoError(t, err)
		assert.Equal(t, []Pattern{
			&ExactValuePattern{value: Int(1)},
			&ExactValuePattern{value: Int(2)},
		}, res.(*UnionPattern).cases)
	})

	t.Run("regex literal : empty", func(t *testing.T) {
		code := "%`a`"

		state := NewGlobalState(NewDefaultTestContext())
		res, err := Eval(code, state, false)

		assert.NoError(t, err)
		assert.IsType(t, &RegexPattern{}, res)
	})

	t.Run("assertion statement: true", func(t *testing.T) {

		t.Run("true", func(t *testing.T) {
			code := "assert true"

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, Nil, res)
		})

		t.Run("false", func(t *testing.T) {
			code := "assert false"

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.Error(t, err)
			assert.Nil(t, res)
		})

	})

	t.Run("testsuite expression", func(t *testing.T) {

		t.Run("no manifest", func(t *testing.T) {
			code := `return testsuite "name" {}`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			if !assert.IsType(t, &TestSuite{}, res) {
				return
			}
			assert.Equal(t, Str("name"), res.(*TestSuite).meta)
		})

		t.Run("no meta", func(t *testing.T) {
			code := `return testsuite {}`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			if !assert.IsType(t, &TestSuite{}, res) {
				return
			}
			assert.Equal(t, Nil, res.(*TestSuite).meta)
		})

	})

	t.Run("testcase expression", func(t *testing.T) {

		t.Run("no manifest", func(t *testing.T) {
			code := `return testcase "name" {}`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			if !assert.IsType(t, &TestCase{}, res) {
				return
			}
			assert.Equal(t, Str("name"), res.(*TestCase).meta)
		})

		t.Run("no meta", func(t *testing.T) {
			code := `return testcase {}`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			if !assert.IsType(t, &TestCase{}, res) {
				return
			}
			assert.Equal(t, Nil, res.(*TestCase).meta)
		})

	})

	t.Run("lifetimejob expression", func(t *testing.T) {

		t.Run("no manifest", func(t *testing.T) {
			code := `return lifetimejob "name" {}`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			if !assert.IsType(t, &LifetimeJob{}, res) {
				return
			}
			assert.Equal(t, Str("name"), res.(*LifetimeJob).meta)
		})

	})

	t.Run("testsuite statement", func(t *testing.T) {

		t.Run("empty", func(t *testing.T) {
			code := `testsuite "name" {}`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, Nil, res)
		})

		t.Run("empty test case", func(t *testing.T) {
			code := `testsuite "name" {
				testcase {}
			}`

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, Nil, res)
		})

		t.Run("manifest with ungranted permissions", func(t *testing.T) {
			code := `testsuite "name" {
				manifest {    
					permissions: { read: https://example.com/index.html }
				}
			}`

			state := NewGlobalState(NewContext(ContextConfig{
				Permissions: []Permission{RoutinePermission{Kind_: permkind.Create}},
			}))
			res, err := Eval(code, state, false)

			assert.Error(t, err)
			assert.ErrorIs(t, err, NewNotAllowedError(HttpPermission{
				Kind_:  permkind.Read,
				Entity: URL("https://example.com/index.html"),
			}))
			assert.Nil(t, res)
		})

		t.Run("test case with failing assertion", func(t *testing.T) {
			code := `testsuite "name" {
				testcase {
					assert false
				}
			}`

			state := NewGlobalState(NewDefaultTestContext())
			state.Out = os.Stdout
			state.Logger = zerolog.New(state.Out)
			res, err := Eval(code, state, false)

			e := errors.Unwrap(err)
			for {
				unwrapped := errors.Unwrap(e)
				if unwrapped == nil {
					break
				}
				e = unwrapped
			}

			assert.IsType(t, &AssertionError{}, e)
			assert.Nil(t, res)
		})
	})

	t.Run("testcase statement", func(t *testing.T) {

		t.Run("manifest with ungranted permissions", func(t *testing.T) {
			code := `testcase "name" {
				manifest {    
					permissions: { read: https://example.com/index.html }
				}
			}`

			state := NewGlobalState(NewContext(ContextConfig{
				Permissions: []Permission{RoutinePermission{Kind_: permkind.Create}},
			}))
			res, err := Eval(code, state, false)

			assert.Error(t, err)
			assert.ErrorIs(t, err, NewNotAllowedError(HttpPermission{
				Kind_:  permkind.Read,
				Entity: URL("https://example.com/index.html"),
			}))
			assert.Nil(t, res)
		})
	})

	t.Run("string template literal", func(t *testing.T) {

		replace := func(s string) string {
			return strings.ReplaceAll(s, "|", "`")
		}

		t.Run("no interpolation", func(t *testing.T) {
			code := replace(`
				%digit = %str('0'..'9')
				return %digit|3|
			`)

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, CheckedString{
				str:                 "3",
				matchingPatternName: "digit",
				matchingPattern:     state.Ctx.ResolveNamedPattern("digit"),
			}, res)
		})

		t.Run("no pattern, no interpolation", func(t *testing.T) {
			code := replace(`return |3|`)

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, Str("3"), res)
		})

		t.Run("valid interpolations", func(t *testing.T) {
			code := replace(`
				%sql. = {
					stmt: %str( %|.*| )
					int: %str( '0'..'9'+ )
				}
				unsanitized_id = "5"
				return %sql.stmt|SELECT * FROM users WHERE id = {{int:$unsanitized_id}}|
			`)

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, CheckedString{
				str:                 "SELECT * FROM users WHERE id = 5",
				matchingPatternName: "sql.stmt",
				matchingPattern:     state.Ctx.ResolvePatternNamespace("sql").Patterns["stmt"],
			}, res)
		})

		t.Run("valid interpolation with conversion", func(t *testing.T) {
			code := replace(`
				return %ns.any_str|integer = {{int_str.from:5}}|
			`)

			state := NewGlobalState(NewDefaultTestContext())
			state.Ctx.AddPatternNamespace("ns", &PatternNamespace{
				Patterns: map[string]Pattern{
					"any_str": STR_PATTERN,
					"int_str": utils.Ret0(INT_PATTERN.StringPattern()),
				},
			})
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, CheckedString{
				str:                 "integer = 5",
				matchingPatternName: "ns.any_str",
				matchingPattern:     state.Ctx.ResolvePatternNamespace("ns").Patterns["any_str"],
			}, res)
		})

		t.Run("invalid interpolation", func(t *testing.T) {
			code := replace(`
				%sql. = {
					stmt: %str( %|.*| )
					int: %str( '0'..'9'+ )
				}
				unsanitized_id = "e5"
				return %sql.stmt|SELECT * FROM users WHERE id = {{int:$unsanitized_id}}|
			`)

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.Error(t, err)
			assert.Nil(t, res)
		})

		t.Run("final string does not match pattern", func(t *testing.T) {
			code := replace(`
				%sql. = {
					stmt: %str( %|x.*| )
					int: %str( '0'..'9'+ )
				}
				unsanitized_id = "5"
				return %sql.stmt|SELECT * FROM users WHERE id = {{int:$unsanitized_id}}|
			`)

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.Error(t, err)
			assert.Nil(t, res)
		})

		t.Run("no pattern, leading interpolation", func(t *testing.T) {
			code := replace(`
				s = "1"
				return |{{s}}2|
			`)

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, Str("12"), res)
		})

		t.Run("no pattern, trailing interpolation", func(t *testing.T) {
			code := replace(`
				s = "2"
				return |1{{s}}|
			`)

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, Str("12"), res)
		})

		t.Run("no pattern, interpolation & escaped n (\\n)", func(t *testing.T) {
			code := replace(`
				s = "1"
				return |{{s}}\n2|
			`)

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, Str("1\n2"), res)
		})

		t.Run("no pattern, interpolation & linefeed", func(t *testing.T) {
			code := replace("s = \"1\"; return |{{s}}\n2|")

			state := NewGlobalState(NewDefaultTestContext())
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, Str("1\n2"), res)
		})
	})

	t.Run("sendval expression", func(t *testing.T) {

		t.Run("supersystem receiver", func(t *testing.T) {
			code := `
				system = {
					part: {
						lifetimejob #send-value {
							sendval 1 to supersys
						}
					}

					lifetimejob #send-value {}
				}

				return system
			`

			ctx := NewDefaultTestContext()
			defer ctx.Cancel()
			state := NewGlobalState(ctx)

			res, err := Eval(code, state, true)
			if !assert.NoError(t, err) {
				return
			}
			w := WatchReceivedMessages(ctx, res.(*Object))
			message, err := w.WaitNext(ctx, nil, time.Second)
			if !assert.NoError(t, err) {
				return
			}

			assert.IsType(t, Message{}, message)
			assert.Equal(t, Int(1), message.(Message).data)
		})

		t.Run("supersystem does not exist", func(t *testing.T) {
			code := `
				system = {
					part: {
						lifetimejob #send-value {
							sendval 1 to supersys
						}
					}
				}

				return system
			`

			ctx := NewDefaultTestContext()
			defer ctx.Cancel()
			state := NewGlobalState(ctx)

			res, err := Eval(code, state, true)
			if !assert.NoError(t, err) {
				return
			}
			w := WatchReceivedMessages(ctx, res.(*Object))
			_, err = w.WaitNext(ctx, nil, time.Second)
			if !assert.ErrorIs(t, err, ErrWatchTimeout) {
				return
			}
		})
	})

	t.Run("transaction", func(t *testing.T) {

		newState := func() (*GlobalState, **Transaction) {
			_tx := new(*Transaction)

			return NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"start_tx": ValOf(func(ctx *Context) *Transaction {
					*_tx = StartNewTransaction(ctx)
					return *_tx
				}),
				"do_reversible_side_effect": ValOf(func(ctx *Context) {
					effect := &reversibleEffect{}
					if ctx.HasCurrentTx() {
						if err := ctx.GetTx().AddEffect(ctx, effect); err != nil {
							panic(err)
						}
					}
				}),
				"do_irreversible_side_effect": ValOf(func(ctx *Context) {
					effect := &irreversibleEffect{}
					if ctx.HasCurrentTx() {
						if err := ctx.GetTx().AddEffect(ctx, effect); err != nil {
							panic(err)
						}
					}
				}),
			}), _tx
		}

		t.Run("effects", func(t *testing.T) {
			t.Run("single, reversible side effect", func(t *testing.T) {
				code := `
					tx = start_tx()
					do_reversible_side_effect()
				`

				state, tx := newState()
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, Nil, res)
				assert.NotNil(t, tx)
				assert.Equal(t, []Effect{
					&reversibleEffect{applied: false},
				}, (*tx).effects)
			})

			t.Run("single, irreversible side effect", func(t *testing.T) {
				code := `
					tx = start_tx()
					do_irreversible_side_effect()
					return "after"
				`

				state, tx := newState()
				res, err := Eval(code, state, false)

				assert.ErrorIs(t, err, ErrCannotAddIrreversibleEffect)
				assert.Nil(t, res)
				assert.NotNil(t, tx)
				assert.Empty(t, (*tx).effects)
			})
		})

		t.Run("commit", func(t *testing.T) {
			t.Run("single, reversible side effect", func(t *testing.T) {
				code := `
					tx = start_tx()
					do_reversible_side_effect()
					tx.commit()
				`

				state, tx := newState()
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, Nil, res)
				assert.NotNil(t, tx)
				assert.Equal(t, []Effect{
					&reversibleEffect{applied: true},
				}, (*tx).effects)
			})
		})

	})

	t.Run("map fn", func(t *testing.T) {
		t.Run("recursive map calls", func(t *testing.T) {
			code := `
				fn rec(list %iterable){
				    assert (list match %[]%iterable)
					return map(list, rec)
				}

				return rec([ [ [], [] ], [ [], [] ]])
			`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"map": WrapGoFunction(Map),
			})
			state.Ctx.AddNamedPattern("iterable", ITERABLE_PATTERN)

			res, err := Eval(code, state, true)
			assert.NoError(t, err)
			assert.EqualValues(t, NewWrappedValueList(
				NewWrappedValueList(NewWrappedValueListFrom([]Value{}), NewWrappedValueListFrom([]Value{})),
				NewWrappedValueList(NewWrappedValueListFrom([]Value{}), NewWrappedValueListFrom([]Value{})),
			), res)
		})

		t.Run("recursive map calls witin a function called in isolation", func(t *testing.T) {
			code := `
				fn rec(list %iterable){
				    assert (list match %[]%iterable)
					return map(list, rec)
				}

				fn isolated(){
					return rec([ [ [], [] ], [ [], [] ]])
				}

				return isolated
			`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"map": WrapGoFunction(Map),
			})
			state.Ctx.AddNamedPattern("iterable", ITERABLE_PATTERN)

			val, err := Eval(code, state, true)
			if !assert.NoError(t, err) {
				return
			}

			fn := val.(*InoxFunction)
			res, err := fn.Call(state, nil, nil, nil)
			if !assert.NoError(t, err) {
				return
			}

			assert.EqualValues(t, NewWrappedValueList(
				NewWrappedValueList(NewWrappedValueListFrom([]Value{}), NewWrappedValueListFrom([]Value{})),
				NewWrappedValueList(NewWrappedValueListFrom([]Value{}), NewWrappedValueListFrom([]Value{})),
			), res)
		})

		t.Run("recursive map calls witin a method called in isolation", func(t *testing.T) {
			code := `
				fn rec(list %iterable){
				    assert (list match %[]%iterable)
					return map(list, rec)
				}

				obj = {
					isolated: fn(){
						return rec([ [ [], [] ], [ [], [] ]])
					}
				}
		
				return obj.isolated
			`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"map": WrapGoFunction(Map),
			})
			state.Ctx.AddNamedPattern("iterable", ITERABLE_PATTERN)

			val, err := Eval(code, state, true)
			if !assert.NoError(t, err) {
				return
			}

			fn := val.(*InoxFunction)
			res, err := fn.Call(state, nil, nil, nil)
			if !assert.NoError(t, err) {
				return
			}

			assert.EqualValues(t, NewWrappedValueList(
				NewWrappedValueList(NewWrappedValueListFrom([]Value{}), NewWrappedValueListFrom([]Value{})),
				NewWrappedValueList(NewWrappedValueListFrom([]Value{}), NewWrappedValueListFrom([]Value{})),
			), res)
		})
	})

	t.Run("XML expression", func(t *testing.T) {
		__idt := WrapGoFunction(func(ctx *Context, e *XMLElement) *XMLElement {
			return e
		})

		createNamespace := func() *Record {
			return NewRecordFromMap(ValMap{symbolic.FROM_XML_FACTORY_NAME: __idt})
		}

		t.Run("element", func(t *testing.T) {
			code := `idt<div></div>`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewXmlElement("div", nil, []Value{Str("")}), val)
		})

		t.Run("integer attribute", func(t *testing.T) {
			code := `idt<div a=1></div>`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewXmlElement("div", []XMLAttribute{{name: "a", value: Int(1)}}, []Value{Str("")}), val)
		})

		t.Run("string attribute", func(t *testing.T) {
			code := `idt<div a="b"></div>`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewXmlElement("div", []XMLAttribute{{name: "a", value: Str("b")}}, []Value{Str("")}), val)
		})

		t.Run("attribute without value", func(t *testing.T) {
			code := `idt<div a></div>`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewXmlElement("div", []XMLAttribute{{name: "a", value: DEFAULT_XML_ATTR_VALUE}}, []Value{Str("")}), val)
		})

		t.Run("value of attribute should be HTML escaped", func(t *testing.T) {
			code := `idt<div a="<"></div>`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewXmlElement("div", []XMLAttribute{{name: "a", value: Str("<")}}, []Value{Str("")}), val)
		})

		t.Run("linefeed", func(t *testing.T) {
			code := "idt<div>\n</div>"
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewXmlElement("div", nil, []Value{Str("\n")}), val)
		})

		t.Run("empty child", func(t *testing.T) {
			code := "idt<div><span></span></div>"
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewXmlElement("div", nil, []Value{
				Str(""),
				NewXmlElement("span", nil, []Value{Str("")}),
				Str(""),
			}), val)
		})

		t.Run("single attribute + empty child", func(t *testing.T) {
			code := "idt<div a=1><span></span></div>"
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewXmlElement("div",
				[]XMLAttribute{{name: "a", value: Int(1)}},
				[]Value{
					Str(""),
					NewXmlElement("span", nil, []Value{Str("")}),
					Str(""),
				}), val)
		})

		t.Run("two attributes + empty child", func(t *testing.T) {
			code := "idt<div a=1 b=2><span></span></div>"
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewXmlElement("div",
				[]XMLAttribute{
					{name: "a", value: Int(1)},
					{name: "b", value: Int(2)},
				},
				[]Value{
					Str(""),
					NewXmlElement("span", nil, []Value{Str("")}),
					Str(""),
				}), val)
		})

		t.Run("linefeed followed by empty child", func(t *testing.T) {
			code := "idt<div>\n<span></span></div>"
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewXmlElement("div", nil, []Value{
				Str("\n"),
				NewXmlElement("span", nil, []Value{Str("")}),
				Str(""),
			}), val)
		})

		t.Run("non-empty child", func(t *testing.T) {
			code := "idt<div><span>1</span></div>"
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewXmlElement("div", nil, []Value{
				Str(""),
				NewXmlElement("span", nil, []Value{Str("1")}),
				Str(""),
			}), val)
		})

		t.Run("two empty children", func(t *testing.T) {
			code := "idt<div><span></span><span></span></div>"
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewXmlElement("div", nil, []Value{
				Str(""),
				NewXmlElement("span", nil, []Value{Str("")}),
				Str(""),
				NewXmlElement("span", nil, []Value{Str("")}),
				Str(""),
			}), val)
		})

		t.Run("child + grandchild", func(t *testing.T) {
			code := "idt<div><span><span></span></span></div>"
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewXmlElement("div", nil, []Value{
				Str(""),
				NewXmlElement("span", nil, []Value{
					Str(""),
					NewXmlElement("span", nil, []Value{Str("")}),
					Str(""),
				}),
				Str(""),
			}), val)
		})

		t.Run("non-zero stack height", func(t *testing.T) {
			code := `
				a = "1"
				b = "2"
				node = idt<div a="a">
					<div> </div> <div> </div>
				</div>
			`
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			_, err := Eval(code, state, false)
			assert.NoError(t, err)
		})

		t.Run("interpolation: XML element", func(t *testing.T) {
			code := "idt<div>{idt<span></span>}</div>"
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewXmlElement("div", nil, []Value{
				Str(""),
				NewXmlElement("span", nil, []Value{Str("")}),
				Str(""),
			}), val)
		})

		t.Run("interpolation: string", func(t *testing.T) {
			code := "idt<div>{\"a\"}</div>"
			state := NewGlobalState(NewDefaultTestContext(), map[string]Value{
				"idt": createNamespace(),
			})

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, NewXmlElement("div", nil, []Value{
				Str(""),
				Str("a"),
				Str(""),
			}), val)
		})
	})

}

func TestTreeWalkDebug(t *testing.T) {
	//TODO: add test with included chunks

	testDebugModeEval(t, func(code string, breakpointLines ...int) (any, *Context, *parse.ParsedChunk, *Debugger) {
		state := NewGlobalState(NewDefaultTestContext())
		treeWalkState := NewTreeWalkStateWithGlobal(state)

		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "core-test",
			CodeString: code,
		}))

		nextBreakPointId := int32(INITIAL_BREAKPOINT_ID)
		breakpoints, err := GetBreakpointsFromLines(breakpointLines, chunk, &nextBreakPointId)
		if !assert.NoError(t, err) {
			assert.Fail(t, "failed to get breakpoints from lines "+err.Error())
		}

		debugger := NewDebugger(DebuggerArgs{
			Logger:             zerolog.New(io.Discard),
			InitialBreakpoints: breakpoints,
		})
		debugger.AttachAndStart(treeWalkState)

		state.Module = &Module{MainChunk: chunk}
		return treeWalkState, state.Ctx, chunk, debugger
	}, func(n parse.Node, state any) (Value, error) {
		result, err := TreeWalkEval(n, state.(*TreeWalkState))

		return result, err
	})
}

func testDebugModeEval(
	t *testing.T,
	setup func(code string, breakpointLines ...int) (any, *Context, *parse.ParsedChunk, *Debugger),
	eval func(n parse.Node, state any) (Value, error),
) {

	t.Run("shallow", func(t *testing.T) {

		t.Run("successive breakpoints", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				a = 1
				a = 2
				a = 3
				return a
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.Cancel()

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[1]: {}, //a = 2
					chunk.Node.Statements[2]: {}, //a = 3
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
				}

				controlChan <- DebugCommandContinue{}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
				}

				controlChan <- DebugCommandContinue{}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop},
				{Reason: BreakpointStop},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)}, {"a": Int(2)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   3,
						StatementStartColumn: 5,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   4,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("successive breakpoints set by line", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(
				`a = 1
				a = 2
				a = 3
				return a
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.Cancel()

			controlChan <- DebugCommandSetBreakpoints{
				Chunk:             chunk,
				BreakPointsByLine: []int{2, 3}, //a = 2 & a = 3
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
				}

				controlChan <- DebugCommandContinue{}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
				}

				controlChan <- DebugCommandContinue{}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop},
				{Reason: BreakpointStop},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)}, {"a": Int(2)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   2,
						StatementStartColumn: 5,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   3,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("successive breakpoints set by line during initialization", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(
				`a = 1
				a = 2
				a = 3
				return a
			`, 2, 3) //a = 2 & a = 3

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.Cancel()

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
				}

				controlChan <- DebugCommandContinue{}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet

				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
				}

				controlChan <- DebugCommandContinue{}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop},
				{Reason: BreakpointStop},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)}, {"a": Int(2)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   2,
						StatementStartColumn: 5,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   3,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("breakpoint & two steps", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				a = 1
				a = 2
				a = 3
				return a
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.Cancel()

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[0]: {}, //a = 1
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				controlChan <- DebugCommandNextStep{}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
				}

				controlChan <- DebugCommandNextStep{}

				event = <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 3'
				controlChan <- DebugCommandGetScopes{
					func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
				}

				//get stack trace while stopped at 'a = 3'
				controlChan <- DebugCommandGetStackTrace{
					func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
				}

				controlChan <- DebugCommandContinue{}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(3), result)

			assert.Equal(t, []ProgramStoppedEvent{
				{Reason: BreakpointStop},
				{Reason: StepStop},
				{Reason: StepStop},
			}, stoppedEvents)

			assert.Equal(t, []map[string]Value{{}, {}}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)}, {"a": Int(2)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[1],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   3,
						StatementStartColumn: 5,
					},
				},
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   4,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("pause", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				a = 1
				sleep 1s
				a = 2
				return a
			`)

			global := WrapGoFunction(Sleep)
			ctx.GetClosestState().Globals.Set("sleep", global)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.Cancel()

			var stoppedEvents []ProgramStoppedEvent
			var globalScopes []map[string]Value
			var localScopes []map[string]Value
			var stackTraces [][]StackFrameInfo

			go func() {
				//wait to make sure the pause command will be sent during the sleep(1s) call
				time.Sleep(10 * time.Millisecond)

				controlChan <- DebugCommandPause{}

				event := <-stoppedChan
				event.Breakpoint = nil //not checked yet
				stoppedEvents = append(stoppedEvents, event)

				//get scopes while stopped at 'a = 2'
				controlChan <- DebugCommandGetScopes{
					func(globalScope, localScope map[string]Value) {
						globalScopes = append(globalScopes, globalScope)
						localScopes = append(localScopes, localScope)
					},
				}

				//get stack trace while stopped at 'a = 2'
				controlChan <- DebugCommandGetStackTrace{
					func(trace []StackFrameInfo) {
						stackTraces = append(stackTraces, trace)
					},
				}

				controlChan <- DebugCommandContinue{}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(2), result)

			assert.Equal(t, []ProgramStoppedEvent{{Reason: PauseStop}}, stoppedEvents)

			assert.Equal(t, []map[string]Value{
				{"sleep": global},
			}, globalScopes)

			assert.Equal(t, []map[string]Value{
				{"a": Int(1)},
			}, localScopes)

			assert.Equal(t, [][]StackFrameInfo{
				{
					{
						Name:                 "core-test",
						Node:                 chunk.Node.Statements[2],
						Chunk:                chunk,
						Id:                   1,
						StartLine:            1,
						StartColumn:          1,
						StatementStartLine:   4,
						StatementStartColumn: 5,
					},
				},
			}, stackTraces)
		})

		t.Run("close debugger while program stopped at breakpoint", func(t *testing.T) {
			state, ctx, chunk, debugger := setup(`
				a = 1
				a = 2
				return a
			`)

			controlChan := debugger.ControlChan()
			stoppedChan := debugger.StoppedChan()

			defer ctx.Cancel()

			controlChan <- DebugCommandSetBreakpoints{
				Chunk: chunk,
				BreakpointsAtNode: map[parse.Node]struct{}{
					chunk.Node.Statements[0]: {}, //a = 1

					//this breakpoint should be ignored because the debugger should be closed when it is reached
					chunk.Node.Statements[1]: {}, //a = 2
				},
			}

			time.Sleep(10 * time.Millisecond) //wait for the debugger to set the breakpoints

			go func() {
				<-stoppedChan
				//a = 1

				controlChan <- DebugCommandCloseDebugger{}
			}()

			result, err := eval(chunk.Node, state)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, Int(2), result)
		})
	})
}

func TestSpawnRoutine(t *testing.T) {

	t.Run("spawning a routine without the required permission should fail", func(t *testing.T) {
		ctx := NewContext(ContextConfig{
			Permissions: []Permission{
				GlobalVarPermission{Kind_: permkind.Read, Name: "*"},
				GlobalVarPermission{Kind_: permkind.Use, Name: "*"},
				GlobalVarPermission{Kind_: permkind.Create, Name: "*"},
			},
		})
		state := NewGlobalState(ctx)
		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "routine-test",
			CodeString: "",
		}))

		routine, err := SpawnRoutine(RoutineSpawnArgs{
			SpawnerState: state,
			Globals:      GlobalVariablesFromMap(map[string]Value{}, nil),
			Module: &Module{
				MainChunk:  chunk,
				ModuleKind: UserRoutineModule,
			},
		})
		assert.Nil(t, routine)
		assert.Error(t, err)
	})

	t.Run("a routine should have access to globals passed to it", func(t *testing.T) {
		state := NewGlobalState(NewContext(ContextConfig{
			Permissions: []Permission{
				GlobalVarPermission{Kind_: permkind.Read, Name: "*"},
				GlobalVarPermission{Kind_: permkind.Use, Name: "*"},
				GlobalVarPermission{Kind_: permkind.Create, Name: "*"},
				RoutinePermission{permkind.Create},
			},
		}))
		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "routine-test",
			CodeString: "return $$x",
		}))

		routine, err := SpawnRoutine(RoutineSpawnArgs{
			SpawnerState: state,
			Globals: GlobalVariablesFromMap(map[string]Value{
				"x": Int(1),
			}, nil),
			Module: &Module{
				MainChunk:  chunk,
				ModuleKind: UserRoutineModule,
			},
		})
		assert.NoError(t, err)

		res, err := routine.WaitResult(nil)
		assert.NoError(t, err)
		assert.Equal(t, Int(1), res)
	})

	t.Run("the result of a routine should be shared if it is sharable", func(t *testing.T) {
		state := NewGlobalState(NewContext(ContextConfig{
			Permissions: []Permission{
				GlobalVarPermission{Kind_: permkind.Read, Name: "*"},
				GlobalVarPermission{Kind_: permkind.Use, Name: "*"},
				GlobalVarPermission{Kind_: permkind.Create, Name: "*"},
				RoutinePermission{permkind.Create},
			},
		}))
		chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
			NameString: "routine-test",
			CodeString: "return {a: 1}",
		}))

		routine, err := SpawnRoutine(RoutineSpawnArgs{
			SpawnerState: state,
			Globals:      GlobalVariablesFromMap(map[string]Value{}, nil),
			Module: &Module{
				MainChunk:  chunk,
				ModuleKind: UserRoutineModule,
			},
		})
		assert.NoError(t, err)

		res, err := routine.WaitResult(nil)
		assert.NoError(t, err)
		if !assert.IsType(t, &Object{}, res) {
			return
		}
		obj := res.(*Object)
		assert.True(t, obj.IsShared())
		assert.Equal(t, map[string]Value{"a": Int(1)}, obj.EntryMap())
	})

}

func TestToBool(t *testing.T) {

	testCases := []struct {
		name  string
		input Value
		ok    bool
	}{
		{"nil slice", (KeyList)(nil), false},
		{"empty, not-nil slice", KeyList{}, false},
		{"not empty slice", KeyList{"a"}, true},
		{"not empty pointer", &testMutableGoValue{}, true},
		{"empty pointer", (*testMutableGoValue)(nil), false},
		{"unitialized struct", testMutableGoValue{}, true},
		{"empty string", Str(""), false},
		{"not empty string", Str("1"), true},
		{"empty list", NewWrappedValueList(), false},
		{"not empty list", NewWrappedValueList(Int(1)), true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assert.True(t, testCase.ok == coerceToBool(testCase.input))
		})
	}
}

func TestGetQuantity(t *testing.T) {
	//TODO
}

func NewDefaultTestContext() *Context {
	return NewContext(ContextConfig{
		Permissions: []Permission{
			GlobalVarPermission{permkind.Read, "*"},
			GlobalVarPermission{permkind.Update, "*"},
			GlobalVarPermission{permkind.Create, "*"},
			GlobalVarPermission{permkind.Use, "*"},

			HttpPermission{permkind.Read, HostPattern("https://**")},
			RoutinePermission{permkind.Create},
		},
		Filesystem: newOsFilesystem(),
	})
}

type evalFn = func(chunkStringOrModule any, state *GlobalState, doSymbolicCheck bool) (Value, error)

func splitLines(ctx *Context, s Str) (slice []Str) {
	for _, e := range strings.Split(string(s), "\n") {
		slice = append(slice, Str(e))
	}
	return
}

type reversibleEffect struct {
	applied bool
}

func (e *reversibleEffect) Resources() []ResourceName {
	return nil
}

func (e *reversibleEffect) PermissionKind() PermissionKind {
	return permkind.Create
}
func (e *reversibleEffect) Reversability(*Context) Reversability {
	return Reversible
}

func (e *reversibleEffect) IsApplied() bool {
	return e.applied
}

func (e *reversibleEffect) Apply(*Context) error {
	if e.applied {
		return ErrEffectAlreadyApplied
	}
	e.applied = true
	return nil
}

func (e *reversibleEffect) Reverse(*Context) error {
	return nil
}

type irreversibleEffect struct {
	applied bool
}

func (e *irreversibleEffect) Resources() []ResourceName {
	return nil
}

func (e *irreversibleEffect) PermissionKind() PermissionKind {
	return permkind.Create
}
func (e *irreversibleEffect) Reversability(*Context) Reversability {
	return Irreversible
}

func (e *irreversibleEffect) IsApplied() bool {
	return e.applied
}

func (e *irreversibleEffect) Apply(*Context) error {
	if e.applied {
		return ErrEffectAlreadyApplied
	}
	e.applied = true
	return nil
}

func (e *irreversibleEffect) Reverse(*Context) error {
	return nil
}
