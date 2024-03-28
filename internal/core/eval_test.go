package core_test

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-billy/v5/helper/polyfill"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/osfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	permkind "github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/globalnames"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/testconfig"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/inoxlang/inox/internal/utils/fsutils"
	"github.com/rs/zerolog"

	"github.com/stretchr/testify/assert"
)

const (
	RETURN_1_MODULE_HASH               = "gtY+/Y/VOxkFgAmefByH5GU8j+b7xtpLu1QLY39BqkE="
	RETURN_NON_POS_ARG_A_MODULE_HASH   = "15Njs+OhmiW9843cgnlMib7AiUzZbGx6gn3GAebWMOA="
	RETURN_POS_ARG_A_MODULE_HASH       = "QNJpkgQeB5MA23yXpJ8L5XWLzUQIi6eDwi2HOnPTO3w="
	RETURN_PATTERN_INT_TWO_MODULE_HASH = "HyCSyqI5UdPFc6c8IuSBw6huA6Iwv0TES0mHLx1DaIY="
	RETURN_INT_PATTERN_MODULE_HASH     = "Ub9ua2QldCOc6MvxIPVpUYOQQfQoZpYEoDJitOdKFPA="
)

var (
	NewDefaultTestContext = core.NewDefaultTestContext
)

func init() {
	core.UpdateModuleImportCache(RETURN_1_MODULE_HASH, "manifest{}; return 1")
	core.UpdateModuleImportCache(RETURN_NON_POS_ARG_A_MODULE_HASH, "manifest {parameters: {a: %int}}\nreturn mod-args.a")
	core.UpdateModuleImportCache(RETURN_POS_ARG_A_MODULE_HASH, "manifest {parameters: {{name: #a, pattern: %int}}}\nreturn mod-args.a")
	core.UpdateModuleImportCache(RETURN_PATTERN_INT_TWO_MODULE_HASH, "manifest {}\npattern two = 2; return %two")
	core.UpdateModuleImportCache(RETURN_INT_PATTERN_MODULE_HASH, "manifest {}; return %int")

	core.RegisterSymbolicGoFunction(toByte, func(ctx *symbolic.Context, i *symbolic.Int) *symbolic.Byte {
		return symbolic.ANY_BYTE
	})

	core.RegisterSymbolicGoFunction(isClientInsecureAndStateful, func(ctx *symbolic.Context, h *symbolic.Host) {})

}

func TestTreeWalkEval(t *testing.T) {
	testEval(t, false, makeTreeWalkEvalFunc(t))
}

func TestOptimizedBytecodeEval(t *testing.T) {
	//bytecodeTest(t, true)
}

func TestEvalWithRecycledTreeWalkEvalState(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}
	testEval(t, false, makeRecylingTreeWalkEvalFunc(t))
}

func bytecodeTest(t *testing.T, optimize bool) {
	testEval(t, true, func(c any, s *core.GlobalState, doCheck bool) (core.Value, error) {
		var mod *core.Module

		switch val := c.(type) {
		case *core.Module:
			mod = val
			s.Module = mod
		case parse.SourceFile:
			chunk := utils.Must(parse.ParseChunkSource(val))

			mod = &core.Module{MainChunk: chunk, TopLevelNode: chunk.Node}

			//if the test case provide a module we reuse the source
			if s.Module != nil {
				chunk.Source = s.Module.MainChunk.Source
				s.Module.MainChunk = chunk
				mod = s.Module
			} else {
				s.Module = mod
			}
		case string:
			chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
				NameString: "core-test",
				CodeString: val,
			}))

			mod = &core.Module{MainChunk: chunk, TopLevelNode: chunk.Node}

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

		//tracer := bytes.Buffer{}

		if doCheck { // TODO: enable checks by default ?
			staticCheckData, err := core.StaticCheck(core.StaticCheckInput{
				State:             s,
				Node:              mod.MainChunk.Node,
				Module:            mod,
				Chunk:             mod.MainChunk,
				Globals:           s.Globals,
				Patterns:          s.Ctx.GetNamedPatterns(),
				PatternNamespaces: s.Ctx.GetPatternNamespaces(),
			})
			if !assert.NoError(t, err) {
				return nil, err
			}

			s.StaticCheckData = staticCheckData

			globals := make(map[string]symbolic.ConcreteGlobalValue)
			s.Globals.Foreach(func(name string, v core.Value, isConstant bool) error {
				globals[name] = symbolic.ConcreteGlobalValue{Value: v, IsConstant: isConstant}
				return nil
			})

			symbCtx, err := s.Ctx.ToSymbolicValue()
			if !assert.NoError(t, err) {
				return nil, err
			}

			symbData, err := symbolic.EvalCheck(symbolic.EvalCheckInput{
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

		compilationCtx := core.NewContext(core.ContextConfig{})
		defer compilationCtx.CancelGracefully()

		core.NewGlobalState(compilationCtx)

		panic(errors.New("evaluating using transpiled Inox code is not supported yet"))

		// res, err := EvalVM(mod, s, BytecodeEvaluationConfig{
		// 	Tracer:             &tracer,
		// 	OptimizeBytecode:   optimize,
		// 	CompilationContext: compilationCtx,
		// })

		// if err != nil {
		// 	//t.Log(tracer.String())
		// 	return nil, err
		// }

		// return res, nil
	})
}

// testEval executes the suite of evaluation tests with a given evaluation function
// that can have any implementation (tree walk, bytecode, ...).
func testEval(t *testing.T, bytecodeEval bool, Eval evalFn) {
	permissiveLthreadLimit := core.MustMakeNotAutoDepletingCountLimit(core.THREADS_SIMULTANEOUS_INSTANCES_LIMIT_NAME, 100_000)

	if false {
		runtime.GC()
		startMemStats := new(runtime.MemStats)
		runtime.ReadMemStats(startMemStats)

		defer utils.AssertNoMemoryLeak(t, startMemStats, 100_000, utils.AssertNoMemoryLeakOptions{
			PreSleepDurationMillis: 100,
		})
	}

	t.Run("integer literal", func(t *testing.T) {
		code := "1"
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.Int(1), res)
	})

	t.Run("port literal", func(t *testing.T) {
		code := ":80/http"
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.Port{
			Number: 80,
			Scheme: "http",
		}, res)
	})

	t.Run("quoted string literal", func(t *testing.T) {
		code := `"a"`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.String("a"), res)
	})

	t.Run("multiline string literal", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("single character", func(t *testing.T) {
			code := "`a`"
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.String("a"), res)
		})

		t.Run("linefeed", func(t *testing.T) {
			code := "`1\n2`"
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.String("1\n2"), res)
		})
		t.Run("escaped n (\\n)", func(t *testing.T) {
			code := "`1\\n2`"
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.String("1\n2"), res)
		})
	})

	t.Run("byte slice literal", func(t *testing.T) {
		code := `0x[01]`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.NewByteSlice([]byte{1}, true, ""), res)
	})

	t.Run("property name literal", func(t *testing.T) {
		code := `.a`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.PropertyName("a"), res)
	})

	t.Run("long value-path  literal", func(t *testing.T) {
		code := `.a.b`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.NewLongValuePath([]core.ValuePathSegment{core.PropertyName("a"), core.PropertyName("b")}), res)
	})

	t.Run("boolean literal", func(t *testing.T) {
		code := `true`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.True, res)
	})

	t.Run("nil literal", func(t *testing.T) {
		code := `nil`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.Nil, res)
	})

	t.Run("absolute path literal", func(t *testing.T) {
		code := `/`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.Path("/"), res)
	})

	t.Run("relative path literal", func(t *testing.T) {
		code := `./`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.Path("./"), res)
	})

	t.Run("absolute path pattern literal", func(t *testing.T) {
		code := `%/*`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.PathPattern("/*"), res)
	})

	t.Run("relative path pattern literal", func(t *testing.T) {
		code := `%./*`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.PathPattern("./*"), res)
	})

	t.Run("named-segment path pattern literal", func(t *testing.T) {
		code := `%/home/{:username}`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.IsType(t, &core.NamedSegmentPathPattern{}, res)
	})

	t.Run("path expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("absolute", func(t *testing.T) {
			t.Run("interpolation value is a string", func(t *testing.T) {
				code := `/home/{username}`
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
					"username": core.String("foo"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, core.Path("/home/foo"), res)
			})

			t.Run("interpolation value is a string containing '/'", func(t *testing.T) {
				code := `/home/{username}`
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()
				res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
					"username": core.String("fo/o"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, core.Path("/home/fo/o"), res)
			})

			t.Run("interpolation value is a path containing '?'", func(t *testing.T) {
				code := `/home/{username}`
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				_, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
					"username": core.String("./a?x=1"),
				}), false)
				assert.Error(t, err)
			})

			t.Run("interpolation value is an absolute path", func(t *testing.T) {
				code := `/home/{path}`
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
					"path": core.Path("/foo"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, core.Path("/home/foo"), res)
			})

			t.Run("interpolation value is a relative path", func(t *testing.T) {
				code := `/home/{path}`
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
					"path": core.Path("./foo"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, core.Path("/home/foo"), res)
			})

		})

		t.Run("relative", func(t *testing.T) {

			t.Run("interpolation value is a string", func(t *testing.T) {
				code := `./home/{username}`
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
					"username": core.String("foo"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, core.Path("./home/foo"), res)
			})

			t.Run("interpolation value is a string containing '/'", func(t *testing.T) {
				code := `./home/{username}`
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
					"username": core.String("fo/o"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, core.Path("./home/fo/o"), res)
			})

			t.Run("interpolation value is a path containing '?'", func(t *testing.T) {
				code := `./home/{username}`
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				_, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
					"username": core.String("./a?x=1"),
				}), false)
				assert.Error(t, err)
			})

			t.Run("interpolation value is an absolute path", func(t *testing.T) {
				code := `./home/{path}`
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
					"path": core.Path("/foo"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, core.Path("./home/foo"), res)
			})

			t.Run("interpolation value is a relative path", func(t *testing.T) {
				code := `./home/{path}`
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
					"path": core.Path("./foo"),
				}), false)
				assert.NoError(t, err)
				assert.Equal(t, core.Path("./home/foo"), res)
			})

		})

		injectionCases := []struct {
			input string
			error string
		}{
			//path
			{
				`path = "."; return /.{path}`,
				core.S_PATH_EXPR_PATH_LIMITATION,
			},
			{
				`path = ""; return /.{path}.`,
				core.S_PATH_EXPR_PATH_LIMITATION,
			},
			{
				`path = "?a=b"; return /{path}`,
				core.S_PATH_INTERP_RESULT_LIMITATION,
			},

			//TODO: add more tests
		}

		for _, testCase := range injectionCases {
			t.Run(testCase.input, func(t *testing.T) {
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				res, err := Eval(testCase.input, core.NewGlobalState(ctx, nil), false)
				assert.ErrorContains(t, err, testCase.error)
				assert.Nil(t, res)
			})
		}

	})

	t.Run("path pattern expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("path pattern expression", func(t *testing.T) {
			code := `%/home/{username}/...`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
				"username": core.String("foo"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, core.PathPattern("/home/foo/..."), res)
		})

		t.Run("globbing injection", func(t *testing.T) {
			code := `%/home/{username}/...`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
				"username": core.String("*"),
			}), false)
			assert.Error(t, err)
			assert.Nil(t, res)
		})

	})

	t.Run("HTTP scheme", func(t *testing.T) {
		code := `http://`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.Scheme("http"), res)
	})

	t.Run("HTTP host", func(t *testing.T) {
		code := `https://example.com`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.Host("https://example.com"), res)
	})

	t.Run("HTTP host pattern", func(t *testing.T) {
		code := `%https://**.example.com`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.HostPattern("https://**.example.com"), res)
	})

	t.Run("core.URL expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("host interpolation", func(t *testing.T) {
			code := `https://{host}/`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
				"host": core.String("localhost"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, core.URL("https://localhost/"), res)
		})

		t.Run("single path interpolation : interpolation does not contain '/'", func(t *testing.T) {
			code := `https://example.com/{path}`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
				"path": core.String("index.html"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, core.URL("https://example.com/index.html"), res)
		})

		t.Run("single path interpolation : interpolation starts with '/'", func(t *testing.T) {
			code := `https://example.com/{path}`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
				"path": core.String("/index.html"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, core.URL("https://example.com//index.html"), res)
		})

		t.Run("single path interpolation, no '/' in path slice", func(t *testing.T) {
			code := `https://example.com{path}`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
				"path": core.String("index.html"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, core.URL("https://example.com/index.html"), res)
		})

		t.Run("path interpolation containg an encoded '?'", func(t *testing.T) {
			code := `https://example.com{path}`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
				"path": core.String("%3F"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, core.URL("https://example.com/%3F"), res)
		})

		t.Run("path interpolation containg an encoded '#'", func(t *testing.T) {
			code := `https://example.com{path}`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
				"path": core.String("%23"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, core.URL("https://example.com/%23"), res)
		})

		t.Run("path interpolation starting with a '@'", func(t *testing.T) {
			code := `https://example.com{path}`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval(code, core.NewGlobalState(ctx, map[string]core.Value{
				"path": core.String("@domain.zip"),
			}), false)
			assert.NoError(t, err)
			assert.Equal(t, core.URL("https://example.com/@domain.zip"), res)
		})

		t.Run("host interpolation", func(t *testing.T) {
			code := `$api/index.html`

			ctx := core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{core.GlobalVarPermission{Kind_: permkind.Read, Name: "*"}},
			})
			state := core.NewGlobalState(ctx)
			defer ctx.CancelGracefully()

			state.Globals.Set("api", core.Host("https://example.com"))

			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.URL("https://example.com/index.html"), res)
		})

		t.Run("query with no interpolation", func(t *testing.T) {
			code := `return https://example.com/?v=a`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval(code, core.NewGlobalState(ctx, nil), false)
			assert.NoError(t, err)
			assert.Equal(t, core.URL("https://example.com/?v=a"), res)
		})

		t.Run("single query interpolation", func(t *testing.T) {
			code := `
				x = "a"
				return https://example.com/?v={$x}
			`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval(code, core.NewGlobalState(ctx, nil), false)
			assert.NoError(t, err)
			assert.Equal(t, core.URL("https://example.com/?v=a"), res)
		})

		t.Run("two query interpolations", func(t *testing.T) {
			code := `
				x = "a"
				y = "b"
				return https://example.com/?v={$x}&w={$y}
			`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval(code, core.NewGlobalState(ctx, nil), false)
			assert.NoError(t, err)
			assert.Equal(t, core.URL("https://example.com/?v=a&w=b"), res)
		})

		t.Run("query interpolation containing an encoded '#'", func(t *testing.T) {
			code := `
				x = "%23"
				return https://example.com/?v={$x}
			`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval(code, core.NewGlobalState(ctx, nil), false)
			assert.NoError(t, err)
			assert.Equal(t, core.URL("https://example.com/?v=%23"), res)
		})

		t.Run("query interpolation with an integer value", func(t *testing.T) {
			code := `
				x = 1
				return https://example.com/?v={$x}
			`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval(code, core.NewGlobalState(ctx, nil), false)
			assert.NoError(t, err)
			assert.Equal(t, core.URL("https://example.com/?v=1"), res)
		})

		t.Run("query interpolation with a boolean value", func(t *testing.T) {
			code := `
				x = true
				return https://example.com/?v={$x}
			`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval(code, core.NewGlobalState(ctx, nil), false)
			assert.NoError(t, err)
			assert.Equal(t, core.URL("https://example.com/?v=true"), res)
		})

		injectionCases := []struct {
			input string
			error string
		}{
			//note: %2E is the core.URL encoding for '.'
			//port injection in path
			{
				`path = ":8080"; return https://example.com{path}`,
				core.S_URL_EXPR_PATH_START_LIMITATION,
			},

			//'..' injection in path
			{
				`path = "."; return https://example.com/.{path}`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "."; return https://example.com/%2E{path}`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "."; return https://example.com/%2e{path}`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "%2E"; return https://example.com/.{path}`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "%2e"; return https://example.com/.{path}`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "2E"; return https://example.com/.%{path}`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "E"; return https://example.com/.%2{path}`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "e"; return https://example.com/.%2{path}`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "%2E"; return https://example.com/%2E{path}`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "%2e"; return https://example.com/%2e{path}`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "2E"; return https://example.com/%2E%{path}`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "2e"; return https://example.com/%2e%{path}`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "E"; return https://example.com/%2E%2{path}`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = "e"; return https://example.com/%2e%2{path}`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = ""; return https://example.com/.{path}.`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = ""; return https://example.com/%2E{path}.`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = ""; return https://example.com/%2e{path}.`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = ""; return https://example.com/.{path}%2E`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = ""; return https://example.com/.{path}%2e`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = ""; return https://example.com/%2E{path}%2E`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = ""; return https://example.com/%2e{path}%2e`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = /.; return https://example.com{path}.`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = /.; return https://example.com{path}%2E`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = /.; return https://example.com{path}%2e`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = /%2E; return https://example.com{path}.`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = /%2e; return https://example.com{path}.`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = /%2E; return https://example.com{path}%2E`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},
			{
				`path = /%2e; return https://example.com{path}%2e`,
				core.S_URL_EXPR_PATH_LIMITATION,
			},

			//'\' injection in path
			//note: %5C is the core.URL encoding for '\'
			{
				`path = "/\\"; return https://example.com{path}`,
				core.S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			{
				`path = "/%5C"; return https://example.com{path}`,
				core.S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			{
				`path = "/\\.\\."; return https://example.com{path}`,
				core.S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			{
				`path = "/%5C.%5C."; return https://example.com{path}`,
				core.S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			{
				`path = "/%5C%2E%5C%2E"; return https://example.com{path}`,
				core.S_URL_PATH_INTERP_RESULT_LIMITATION,
			},

			//'?' injection in path
			{
				`path = "?a=b"; return https://example.com{path}`,
				core.S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			{
				`path = "x?a=b"; return https://example.com{path}`,
				core.S_URL_PATH_INTERP_RESULT_LIMITATION,
			},

			//'#' injection in path
			{
				`path = "#"; return https://example.com{path}`,
				core.S_URL_PATH_INTERP_RESULT_LIMITATION,
			},

			//'*' injection in path
			//note: %2A is the core.URL encoding for '*'
			{
				`path = "*"; return https://example.com{path}`,
				core.S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			{
				`path = "/*"; return https://example.com{path}`,
				core.S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			{
				`path = "%2A"; return https://example.com{path}`,
				core.S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			{
				`path = "/%2A"; return https://example.com{path}`,
				core.S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			{
				`path = "%2a"; return https://example.com{path}`,
				core.S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			{
				`path = "/%2a"; return https://example.com{path}`,
				core.S_URL_PATH_INTERP_RESULT_LIMITATION,
			},
			//TODO: add more tests

			//'#' injection in query
			{
				`x = "#id"; return https://example.com/?v={$x}`,
				core.S_QUERY_PARAM_VALUE_LIMITATION,
			},
			//'&' injection in query
			{
				`x = "x&admin=true"; return https://example.com/?v={$x}`,
				core.S_QUERY_PARAM_VALUE_LIMITATION,
			},

			//TODO: add more tests
		}

		for _, testCase := range injectionCases {
			t.Run(testCase.input, func(t *testing.T) {
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				res, err := Eval(testCase.input, core.NewGlobalState(ctx, nil), false)
				assert.ErrorContains(t, err, testCase.error)
				assert.Nil(t, res)
			})
		}

	})

	t.Run("binary comparisons except ==", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		testCases := []struct {
			code   string
			result core.Bool
			err    error
		}{
			// <
			{"(1 < 2)", core.True, nil},
			{"(1 < 1)", core.False, nil},
			{"(2 < 1)", core.False, nil},
			{"(1s < 2s)", core.True, nil},
			{"(1s < 1s)", core.False, nil},
			{"(2s < 1s)", core.False, nil},
			// <=
			{"(1 <= 2)", core.True, nil},
			{"(1 <= 1)", core.True, nil},
			{"(2 <= 1)", core.False, nil},
			{"(1s <= 2s)", core.True, nil},
			{"(1s <= 1s)", core.True, nil},
			{"(2s <= 1s)", core.False, nil},
			// >
			{"(2 > 1)", core.True, nil},
			{"(1 > 1)", core.False, nil},
			{"(1 > 2)", core.False, nil},
			{"(2s > 1s)", core.True, nil},
			{"(1s > 1s)", core.False, nil},
			{"(1s > 2s)", core.False, nil},
			// >=
			{"(2 >= 1)", core.True, nil},
			{"(1 >= 1)", core.True, nil},
			{"(1 >= 2)", core.False, nil},
			{"(2s >= 1s)", core.True, nil},
			{"(1s >= 1s)", core.True, nil},
			{"(1s >= 2s)", core.False, nil},
		}

		for _, testCase := range testCases {
			t.Run(testCase.code, func(t *testing.T) {
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				res, err := Eval(testCase.code, core.NewGlobalState(ctx, nil), false)
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

	t.Run("integer binary expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		testCases := []struct {
			code   string
			result core.Value
			err    error
		}{
			//addition
			{"(1 + 2)", core.Int(3), nil},
			{"(-0 + 2)", core.Int(2), nil},
			{"(0 + 2)", core.Int(2), nil},
			{"(1 + -2)", core.Int(-1), nil},
			{"(1 + -0)", core.Int(1), nil},
			{"(1 + 0)", core.Int(1), nil},
			{"(9223372036854775807 + -1)", core.Int(9223372036854775806), nil},
			{"(-9223372036854775808 + 1)", core.Int(-9223372036854775807), nil},
			{"(9223372036854775807 + 1)", nil, core.ErrIntOverflow},
			{"(-9223372036854775808 + -1)", nil, core.ErrIntUnderflow},
			//substraction
			{"(1 - 2)", core.Int(-1), nil},
			{"(-0 - 2)", core.Int(-2), nil},
			{"(0 - 2)", core.Int(-2), nil},
			{"(1 - -2)", core.Int(3), nil},
			{"(1 - -0)", core.Int(1), nil},
			{"(1 - 0)", core.Int(1), nil},
			{"(9223372036854775807 - 1)", core.Int(9223372036854775806), nil},
			{"(-9223372036854775808 - -1)", core.Int(-9223372036854775807), nil},
			{"(9223372036854775807 - -1)", nil, core.ErrIntOverflow},
			{"(-9223372036854775808 - 1)", nil, core.ErrIntUnderflow},
			//multiplication
			{"(1 * 2)", core.Int(2), nil},
			{"(1 * -2)", core.Int(-2), nil},
			{"(9223372036854775807 * -1)", -core.Int(9223372036854775807), nil},
			{"(9223372036854775807 * -2)", nil, core.ErrIntUnderflow},
			{"(9223372036854775807 * 2)", nil, core.ErrIntOverflow},
			{"(-9223372036854775808 * -1)", nil, core.ErrIntOverflow},
			{"(-9223372036854775808 * -2)", nil, core.ErrIntUnderflow},
			{"(-9223372036854775808 * -9223372036854775808)", nil, core.ErrIntUnderflow},
			//division
			{"(1 / 2)", core.Int(0), nil},
			{"(4 / 2)", core.Int(2), nil},
			{"(1 / 0)", nil, core.ErrIntDivisionByZero},
			{"(9223372036854775807 / -2)", core.Int(-4611686018427387903), nil},
			{"(9223372036854775807 / -1)", core.Int(-9223372036854775807), nil},
			{"(-9223372036854775808 / -2)", core.Int(4611686018427387904), nil},
			{"(-9223372036854775808 / -1)", nil, core.ErrIntOverflow},
		}

		for _, testCase := range testCases {
			t.Run(testCase.code, func(t *testing.T) {
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				res, err := Eval(testCase.code, core.NewGlobalState(ctx, nil), true)
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
		//testconfig.SetParallelization(t)

		NaN := core.Float(math.NaN())

		testCases := []struct {
			code   string
			a      core.Float
			b      core.Float
			result core.Value
			err    error
		}{
			//addition
			{"(a + b)", 1, 2, core.Float(3), nil},
			{"(a + b)", 1, -2, core.Float(-1), nil},
			{"(a + b)", NaN, 1, nil, core.ErrNaNinfinityOperand},
			{"(a + b)", 1, NaN, nil, core.ErrNaNinfinityOperand},
			{"(a + b)", NaN, NaN, nil, core.ErrNaNinfinityOperand},
			//substraction
			{"(a - b)", 1, 2, core.Float(-1), nil},
			{"(a - b)", 1, -2, core.Float(3), nil},
			{"(a - b)", NaN, 1, nil, core.ErrNaNinfinityOperand},
			{"(a - b)", 1, NaN, nil, core.ErrNaNinfinityOperand},
			{"(a - b)", NaN, NaN, nil, core.ErrNaNinfinityOperand},
			//multiplication
			{"(a * b)", 1, 2, core.Float(2), nil},
			{"(a * b)", 1, -2, core.Float(-2), nil},
			{"(a * b)", NaN, 1, nil, core.ErrNaNinfinityOperand},
			{"(a * b)", 1, NaN, nil, core.ErrNaNinfinityOperand},
			{"(a * b)", NaN, NaN, nil, core.ErrNaNinfinityOperand},
			//division
			{"(a / b)", 1, 2, core.Float(0.5), nil},
			{"(a / b)", 1, -2, core.Float(-0.5), nil},
			{"(a / b)", NaN, 1, nil, core.ErrNaNinfinityOperand},
			{"(a / b)", 1, NaN, nil, core.ErrNaNinfinityOperand},
			{"(a / b)", NaN, NaN, nil, core.ErrNaNinfinityOperand},
			{"(a / b)", 1, 0, nil, core.ErrNaNinfinityResult},
		}

		for _, testCase := range testCases {
			name := fmt.Sprintf("%s a:%f, b:%f", testCase.code, testCase.a, testCase.b)
			t.Run(name, func(t *testing.T) {
				state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
					"a": testCase.a,
					"b": testCase.b,
				})

				res, err := Eval(testCase.code, state, true)
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

	t.Run("pseudo arithmetic", func(t *testing.T) {
		//testconfig.SetParallelization(t)

		testCases := []struct {
			code   string
			a      core.Value
			b      core.Value
			result core.Value
			err    error
		}{
			//addition
			{"(a + b)", core.ONE_HOUR, core.ONE_HOUR, 2 * core.ONE_HOUR, nil},
			{"(a + b)", core.DateTime(time.Time{}), core.ONE_HOUR, core.DateTime(time.Time{}.Add(time.Hour)), nil},
			{"(a + b)", core.ONE_HOUR, core.DateTime(time.Time{}), core.DateTime(time.Time{}.Add(time.Hour)), nil},
			{"(a + b)", core.MAX_DURATION, core.Duration(time.Nanosecond), nil, core.ErrQuantityOverflow},

			//substraction
			{"(a - b)", core.ONE_HOUR, core.ONE_MINUTE, core.ONE_HOUR - core.ONE_MINUTE, nil},
			{"(a - b)", core.Duration(time.Nanosecond), core.MAX_DURATION, nil, core.ErrQuantityUnderflow},
			{"(a - b)", core.DateTime(time.Time{}), core.ONE_HOUR, core.DateTime(time.Time{}.Add(-time.Hour)), nil},
		}

		for _, testCase := range testCases {
			name := fmt.Sprintf("%s a:%#v, b:%#v", testCase.code, testCase.a, testCase.b)
			t.Run(name, func(t *testing.T) {
				state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
					"a": testCase.a,
					"b": testCase.b,
				})

				res, err := Eval(testCase.code, state, true)
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

	t.Run("binary in/not-in", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		testCases := []struct {
			code   string
			result core.Bool
			err    error
		}{
			{"(1 in 1..2)", core.True, nil},
			{"(1 not-in 1..2)", core.False, nil},
			{"(0 in 1..2)", core.False, nil},
			{"(0 not-in 1..2)", core.True, nil},

			{"(1 in [1, 2])", core.True, nil},
			{"(1 not-in [1, 2])", core.False, nil},
			{"(0 in [1, 2])", core.False, nil},
			{"(0 not-in [1, 2])", core.True, nil},

			{"(1 in {a: 1})", core.True, nil},
			{"(1 not-in {a: 1, b: 2})", core.False, nil},
			{"(0 in {a: 1, b: 2})", core.False, nil},
			{"(0 not-in {a: 1, b: 2})", core.True, nil},
		}

		for _, testCase := range testCases {
			t.Run(testCase.code, func(t *testing.T) {
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				res, err := Eval(testCase.code, core.NewGlobalState(ctx, nil), false)
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

	t.Run("other binary expressions", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		testCases := []struct {
			code   string
			result core.Value
			err    error
		}{
			{`(1 is 2)`, core.False, nil},
			{`(1 is 1)`, core.True, nil},
			{`({} is {})`, core.False, nil},
			{`obj = {}; return (obj is obj)`, core.True, nil},

			{`(1 is-not 2)`, core.True, nil},
			{`(1 is-not 1)`, core.False, nil},
			{`({} is-not {})`, core.True, nil},
			{`obj = {}; return (obj is-not obj)`, core.False, nil},

			{`(1 match %int)`, core.True, nil},
			{`("1" match %int)`, core.False, nil},
			{`({a: 1} match %{a: 1})`, core.True, nil},
			{`({} match %{a: 1})`, core.False, nil},

			{`("a" keyof {})`, core.False, nil},
			{`("a" keyof {a: 1})`, core.True, nil},
			{`("aa" keyof {"a": "aa"})`, core.False, nil},

			{`("A" substrof "")`, core.False, nil},
			{`("" substrof "")`, core.True, nil},
			{`("A" substrof "A")`, core.True, nil},
			{`("" substrof "A")`, core.True, nil},
			{`("A" substrof "AA")`, core.True, nil},
			{`("AA" substrof "A")`, core.False, nil},

			{`("A" substrof 0d[])`, core.False, nil},
			{`("" substrof 0d[])`, core.True, nil},
			{`("A" substrof 0d[65])`, core.True, nil},
			{`("" substrof 0d[65])`, core.True, nil},
			{`("A" substrof 0d[65 65])`, core.True, nil},
			{`("AA" substrof 0d[65])`, core.False, nil},

			{`(0d[65] substrof "")`, core.False, nil},
			{`(0d[] substrof "")`, core.True, nil},
			{`(0d[65] substrof "A")`, core.True, nil},
			{`(0d[] substrof "A")`, core.True, nil},
			{`(0d[65] substrof "AA")`, core.True, nil},
			{`(0d[65 65] substrof "A")`, core.False, nil},

			{`(%int \ 1)`, core.NewDifferencePattern(core.INT_PATTERN, core.NewExactValuePattern(core.Int(1))), nil},

			{`(1 ?? 2)`, core.Int(1), nil},
			{`(nil ?? 1)`, core.Int(1), nil},
			{`(nil ?? [])`, core.NewWrappedValueList(), nil},
		}

		for _, testCase := range testCases {
			t.Run(testCase.code, func(t *testing.T) {
				state := core.NewGlobalState(NewDefaultTestContext(), nil)
				state.Ctx.AddNamedPattern("int", core.INT_PATTERN)

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

	t.Run("binary urlof", func(t *testing.T) {
		ctx := NewDefaultTestContext()
		defer ctx.CancelGracefully()

		state := core.NewGlobalState(ctx, nil)

		obj := core.NewObject()
		obj.SetURLOnce(ctx, "ldb://main/")
		state.Globals.Set("obj_with_url", obj)

		res, err := Eval("(ldb://main/ urlof obj_with_url)", state, false)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, core.True, res)

		res, err = Eval("(ldb://main/x urlof obj_with_url)", state, false)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, core.False, res)

		res, err = Eval("(ldb://main/x urlof {})", state, false)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, core.False, res)

		res, err = Eval("(ldb://main/x urlof 1)", state, false)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, core.False, res)
	})

	t.Run("integer unary expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("negating the smallest integer should throw an error", func(t *testing.T) {
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval("(- -9223372036854775808)", core.NewGlobalState(ctx, nil), false)
			assert.ErrorIs(t, err, core.ErrNegationWithOverflow)
			assert.Nil(t, res)
		})

		t.Run("negating zero should return zero", func(t *testing.T) {
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval("(- 0)", core.NewGlobalState(ctx, nil), false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(0), res)
		})

		t.Run("negating negative zero should return zero", func(t *testing.T) {
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			res, err := Eval("(- -0)", core.NewGlobalState(ctx, nil), false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(0), res)
		})
	})

	t.Run("range expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		testCases := []struct {
			code   string
			result core.Value
			err    error
		}{
			//addition
			{"(1 .. 2)", core.NewIntRange(1, 2), nil},
			{"(1 ..< 2)", core.NewIntRange(1, 1), nil},
			{"(1.0 .. 2.0)", core.NewFloatRange(1, 2, true), nil},
			{"(1.0 ..< 2.0)", core.NewFloatRange(1, 2, false), nil},
		}

		for _, testCase := range testCases {
			t.Run(testCase.code, func(t *testing.T) {
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				res, err := Eval(testCase.code, core.NewGlobalState(ctx, nil), false)
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

	t.Run("binary pair expression", func(t *testing.T) {
		code := `(1,2)`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, core.NewOrderedPair(core.Int(1), core.Int(2)), res)
	})

	t.Run("binary expression chain", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		testCases := []struct {
			code   string
			result core.Value
		}{
			//'or' chain starting with a binary expression
			{"(1 > 2 or false)", core.False},
			{"(1 < 2 or true)", core.True},
			{"(1 < 2 or 1 < 2)", core.True},
			{"(1 < 2 or 1 > 2)", core.True},
			{"(1 > 2 or 1 > 2)", core.False},
			{"(1 > 2 or false or false)", core.False},
			{"(1 < 2 or true or false)", core.True},
			{"(1 < 2 or 1 < 2 or false)", core.True},
			{"(1 < 2 or 1 > 2 or false)", core.True},
			{"(1 > 2 or 1 > 2 or false)", core.False},
			{"(1 > 2 or 1 > 2 or true)", core.True},

			//'or' chain starting with a literal
			{"(false or false)", core.False},
			{"(true or true)", core.True},
			{"(true or 1 < 2)", core.True},
			{"(true or 1 > 2)", core.True},
			{"(false or 1 > 2)", core.False},
			{"(false or false or false)", core.False},
			{"(true or true or false)", core.True},
			{"(true or 1 < 2 or false)", core.True},
			{"(true or 1 > 2 or false)", core.True},
			{"(false or 1 > 2 or false)", core.False},
			{"(false or 1 > 2 or true)", core.True},

			//'and' chain starting with a binary expression
			{"(1 > 2 and false)", core.False},
			{"(1 < 2 and true)", core.True},
			{"(1 < 2 and 1 < 2)", core.True},
			{"(1 < 2 and 1 > 2)", core.False},
			{"(1 > 2 and 1 > 2)", core.False},
			{"(1 > 2 and false and false)", core.False},
			{"(1 < 2 and true and false)", core.False},
			{"(1 < 2 and 1 < 2 and false)", core.False},
			{"(1 < 2 and 1 > 2 and false)", core.False},
			{"(1 > 2 and 1 > 2 and false)", core.False},
			{"(1 > 2 and 1 > 2 and true)", core.False},
			{"(1 > 2 and true and true)", core.False},
			{"(1 < 2 and true and true)", core.True},
			{"(1 < 2 and 1 < 2 and true)", core.True},
			{"(1 < 2 and 1 > 2 and true)", core.False},
			{"(1 < 2 and false and true)", core.False},
			{"(1 < 2 and 1 > 2 and true)", core.False},

			//'and' chain starting with a literal
			{"(false and false)", core.False},
			{"(true and true)", core.True},
			{"(true and 1 < 2)", core.True},
			{"(true and 1 > 2)", core.False},
			{"(false and 1 > 2)", core.False},
			{"(false and false and false)", core.False},
			{"(true and true and false)", core.False},
			{"(true and 1 < 2 and false)", core.False},
			{"(true and 1 > 2 and false)", core.False},
			{"(false and 1 > 2 and false)", core.False},
			{"(false and 1 > 2 and true)", core.False},
			{"(false and true and true)", core.False},
			{"(true and true and true)", core.True},
			{"(true and 1 < 2 and true)", core.True},
			{"(true and 1 > 2 and true)", core.False},
			{"(true and false and true)", core.False},
			{"(true and 1 > 2 and true)", core.False},
		}

		for _, testCase := range testCases {
			t.Run(testCase.code, func(t *testing.T) {
				ctx := NewDefaultTestContext()
				defer ctx.CancelGracefully()

				res, err := Eval(testCase.code, core.NewGlobalState(ctx, nil), false)
				assert.NoError(t, err)
				assert.Equal(t, testCase.result, res)
			})
		}
	})

	t.Run("local variable declaration", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		testCases := []struct {
			input          string
			error          bool
			skipIfBytecode bool
			result         core.Value
		}{
			{
				input: `
					var a = 1; 
					return a
				`,
				result: core.Int(1),
			},
			{
				input: `
					var (
						a = 1
						b = 2
					)
					return [a, b]
				`,
				result: core.NewWrappedValueList(core.Int(1), core.Int(2)),
			},
		}

		for _, testCase := range testCases {
			if testCase.skipIfBytecode && bytecodeEval {
				continue
			}
			t.Run(testCase.input, func(t *testing.T) {

				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
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

	t.Run("global variable declaration", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		testCases := []struct {
			input          string
			error          bool
			skipIfBytecode bool
			result         core.Value
		}{
			{
				input: `
					globalvar a = 1; 
					return a
				`,
				result: core.Int(1),
			},
			{
				input: `
					globalvar (
						a = 1
						b = 2
					)
					return [a, b]
				`,
				result: core.NewWrappedValueList(core.Int(1), core.Int(2)),
			},
		}

		for _, testCase := range testCases {
			if testCase.skipIfBytecode && bytecodeEval {
				continue
			}
			t.Run(testCase.input, func(t *testing.T) {

				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
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
		testconfig.AllowParallelization(t)

		testCases := []struct {
			input          string
			error          bool
			skipIfBytecode bool
			result         core.Value
			constants      map[string]core.Value
			globalVars     map[string]core.Value
			doSymbolicEval bool
		}{
			{
				input: `
					a = 1; 
					return a
				`,
				result: core.Int(1),
			},
			{
				input: `
					a = 1
					a += 1
					return a
				`,
				result: core.Int(2),
			},
			{
				input: `
					a = 1
					a -= 1
					return a
				`,
				result: core.Int(0),
			},
			{
				input: `
					a = 2
					a *= 3
					return a
				`,
				result: core.Int(6),
			},
			{
				input: `
					a = 6
					a /= 2
					return a
				`,
				result: core.Int(3),
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
		
					$A = 2;
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
				result: core.NewObjectFromMapNoInit(core.ValMap{"v": core.Int(1)}),
			},
			{
				input: `
					a = {}
					a.v = 1
					a.v += 1
					return $a
				`,
				result: core.NewObjectFromMapNoInit(core.ValMap{"v": core.Int(2)}),
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
					struct MyStruct {
						count int
					}
					s = new MyStruct
					s.count = 2
					s.count += 1
					return s.count
				`,
				result:         core.Int(3),
				doSymbolicEval: true,
			},
			{
				input: `
					struct MyStruct {
						inner *Inner
					}
					struct Inner {
						count int
					}
					s = new MyStruct
					s.inner = new Inner

					s.inner.count = 2
					s.inner.count += 1
					return s.inner.count
				`,
				result:         core.Int(3),
				doSymbolicEval: true,
			},
			{
				input: `
					struct MyStruct {
						count int
					}
					s = new MyStruct
					$s.count = 2
					$s.count += 1
					return $s.count
				`,
				result:         core.Int(3),
				doSymbolicEval: true,
			},
			{
				input: `
					struct MyStruct {
						inner *Inner
					}
					struct Inner {
						count int
					}
					$s = new MyStruct
					$s.inner = new Inner

					$s.inner.count = 2
					$s.inner.count += 1
					return $s.inner.count
				`,
				result:         core.Int(3),
				doSymbolicEval: true,
			},
			{
				input: `
					a = [0] 
					$a[0] = 1
					return a
				`,
				result: core.NewWrappedValueList(core.Int(1)),
			},
			{
				input: `
					a = [1] 
					$a[0] += 1
					return a
				`,
				result: core.NewWrappedValueList(core.Int(2)),
			},
			{
				input: `
					a = 0x[00] 
					$a[0] = tobyte(1)
					return a
				`,
				constants: map[string]core.Value{
					"tobyte": core.WrapGoFunction(func(ctx *core.Context, i core.Int) core.Byte {
						return core.Byte(i)
					}),
				},
				result: core.NewByteSlice([]byte{1}, true, ""),
			},
			{
				input: `
					runes[0] = 'b'
					return runes
				`,
				constants: map[string]core.Value{
					"torune": core.WrapGoFunction(func(ctx *core.Context, i core.Int) core.Byte {
						return core.Byte(i)
					}),
				},
				globalVars: map[string]core.Value{
					"runes": core.NewRuneSlice([]rune("a")),
				},
				result: core.NewRuneSlice([]rune("b")),
			},
			{
				input: `
					a = {count:0}
					$a.count = 1
					return $a.count
				`,
				result: core.Int(1),
			},
			{
				input: `
					a = {count: 1}
					$a.count += 1
					return $a.count
				`,
				result: core.Int(2),
			},
			{
				input: `
					a = {}
					$a.count = 1; 
					return $a.count
				`,
				result: core.Int(1),
			},
			{
				input: `
					a = {}
					$a.count = 1; 
					$a.count += 1
					return $a.count
				`,
				result: core.Int(2),
			},

			{
				input: `
					a = [0] 
					$a[0:1] = [1]
					return $a
				`,
				result: core.NewWrappedValueList(core.Int(1)),
			},
		}

		for _, testCase := range testCases {
			if testCase.skipIfBytecode && bytecodeEval {
				continue
			}
			t.Run(testCase.input, func(t *testing.T) {
				state := core.NewGlobalState(NewDefaultTestContext(), testCase.constants)
				for k, v := range testCase.globalVars {
					state.Globals.Set(k, v)
				}
				state.Ctx.AddNamedPattern("int", core.INT_PATTERN)
				state.Ctx.AddNamedPattern("bool", core.BOOL_PATTERN)

				res, err := Eval(testCase.input, state, testCase.doSymbolicEval)
				if testCase.error {
					if !assert.Error(t, err) {
						return
					}
					assert.Nil(t, res)
				} else {
					if !assert.NoError(t, err) {
						return
					}
					assert.Equal(t, testCase.result, res)
				}
			})
		}

		t.Run("assignment : LHS is a pipeline expression", func(t *testing.T) {
			code := `a = | get-data | split-lines $; return $a`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"get-data": core.ValOf(func(ctx *core.Context) core.String {
					return "aaa\nbbb"
				}),
				"split-lines": core.ValOf(splitLines),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.NewWrappedValueList(core.String("aaa"), core.String("bbb")), res)
		})
	})

	t.Run("set difference", func(t *testing.T) {
		t.Run("patterns", func(t *testing.T) {
			code := `((%| 1 | 2) \ 1)`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.IsType(t, &core.DifferencePattern{}, res)
			patt := res.(*core.DifferencePattern)

			assert.IsType(t, &core.UnionPattern{}, patt.Base())
			assert.Equal(t, core.NewExactValuePattern(core.Int(1)), patt.Removed())
		})
	})

	t.Run("nil coalescing", func(t *testing.T) {
		t.Run("left is nil", func(t *testing.T) {
			code := `(nil ?? 1)`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(1), res)
		})

		t.Run("left is not nil", func(t *testing.T) {
			code := `(1 ?? 2)`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(1), res)
		})
	})

	t.Run("return statement", func(t *testing.T) {
		t.Run("value", func(t *testing.T) {
			code := `return nil`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
		})

		t.Run("no value", func(t *testing.T) {
			code := `return`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
		})
	})

	t.Run("index expression", func(t *testing.T) {
		t.Run("list", func(t *testing.T) {
			code := `
				a = [0] 
				return $a[0]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(0), res)
		})

		t.Run("tuple", func(t *testing.T) {
			code := `
				a = #[0] 
				return $a[0]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(0), res)
		})
	})

	t.Run("slice expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("array slice : end index is greater than the length of the array", func(t *testing.T) {
			code := `
				a = [0]
				return $a[0:100]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.NewWrappedValueList(core.Int(0)), res)
		})

		t.Run("string slice : end index is greater than the length of the string", func(t *testing.T) {
			code := `
				a = "0"
				return $a[0:100]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.String("0"), res)
		})

		t.Run("negative start", func(t *testing.T) {
			code := `
				a = ["a"]
				return $a[-1:1]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.ErrorIs(t, err, core.ErrNegativeLowerIndex)
			assert.Nil(t, res)
		})

		t.Run("start and end specified", func(t *testing.T) {
			code := `
				$a = ["a"]
				return $a[0:1]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.NewWrappedValueList(core.String("a")), res)
		})

		t.Run("only start specified", func(t *testing.T) {
			code := `
				a = ["a"]
				return $a[0:]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.NewWrappedValueList(core.String("a")), res)
		})

		t.Run("only end specified", func(t *testing.T) {
			code := `
				a = ["a"]
				return $a[:1]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.NewWrappedValueList(core.String("a")), res)
		})

		t.Run("start out ouf bounds", func(t *testing.T) {
			code := `
				a = ["a"]
				return $a[1:]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.NewWrappedValueListFrom(make([]core.Serializable, 0)), res)
		})

	})

	t.Run("quantity literal : byte count", func(t *testing.T) {
		t.Run("byte count", func(t *testing.T) {
			code := `1kB`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			assert.EqualValues(t, core.ByteCount(1_000), res)
		})

		t.Run("too large", func(t *testing.T) {
			code := `10000000000s`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.Error(t, err)
			assert.ErrorIs(t, err, core.ErrQuantityLooLarge)
			assert.Nil(t, res)
		})
	})

	t.Run("year literal", func(t *testing.T) {
		code := `2020y-UTC`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)

		assert.EqualValues(t, core.DateTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)), res)
	})

	t.Run("date literal", func(t *testing.T) {
		code := `2020y-1mt-1d-UTC`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)

		assert.EqualValues(t, core.DateTime(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)), res)
	})

	t.Run("datetime literal", func(t *testing.T) {
		code := `2020y-1mt-1d-5h-3m-UTC`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()

		res, err := Eval(code, state, false)
		assert.NoError(t, err)

		assert.EqualValues(t, core.DateTime(time.Date(2020, 1, 1, 5, 3, 0, 0, time.UTC)), res)
	})

	t.Run("rate literal : byte rate", func(t *testing.T) {
		t.Run("byte rate", func(t *testing.T) {
			code := `10kB/s`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			assert.EqualValues(t, core.ByteRate(10_000), res)
		})

		t.Run("frequency", func(t *testing.T) {
			code := `10x/s`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			assert.EqualValues(t, core.Frequency(10), res)
		})

	})

	t.Run("global constants", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			code := `
				const ()
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			_, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, map[string]core.Value{}, state.Globals.Entries())
		})

		t.Run("single", func(t *testing.T) {
			code := `
				const (
					a = 1
				)
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			_, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, map[string]core.Value{"a": core.Int(1)}, state.Globals.Entries())
		})
	})

	t.Run("object literal", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("empty object", func(t *testing.T) {
			code := `{}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, &core.Object{}, res)
		})

		t.Run("single property", func(t *testing.T) {
			code := `{a:1}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewObjectFromMapNoInit(core.ValMap{"a": core.Int(1)}), res)
		})

		t.Run("several properties", func(t *testing.T) {
			code := `{a:1,b:2}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewObjectFromMapNoInit(core.ValMap{"a": core.Int(1), "b": core.Int(2)}), res)
		})

		t.Run("one element", func(t *testing.T) {
			code := `{1}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewObjectFromMapNoInit(core.ValMap{
				"": core.NewWrappedValueList(core.Int(1)),
			}), res)
		})

		t.Run("two elements", func(t *testing.T) {
			code := `{1, 2}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewObjectFromMapNoInit(core.ValMap{
				"": core.NewWrappedValueList(core.Int(1), core.Int(2)),
			}), res)
		})

		t.Run("spread element", func(t *testing.T) {
			code := `
				o = {name: "foo"}
				return { ...$o.{name} }
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewObjectFromMapNoInit(core.ValMap{"name": core.String("foo")}), res)
		})

		t.Run("empty lifetime job", func(t *testing.T) {
			code := `{ lifetimejob #job {  } }`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			obj := res.(*core.Object)
			if !assert.Len(t, obj.JobInstances(), 1) {
				return
			}
			jobInstance := obj.JobInstances()[0]
			expected := obj.Prop(state.Ctx, "").(*core.List).At(state.Ctx, 0)
			assert.Equal(t, expected, jobInstance.Job())

			//assert.Equal(t, bytecodeEval, jobInstance.Thread().useBytecode)
		})

		t.Run("lifetimejob with ungranted permissions", func(t *testing.T) {
			code := `{ 
				lifetimejob "name" { 
					manifest { permissions: { read: https://example.com/index.html } }
				}
			}`

			state := core.NewGlobalState(core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{core.LThreadPermission{Kind_: permkind.Create}},
			}))
			res, err := Eval(code, state, false)

			assert.Error(t, err)
			assert.ErrorIs(t, err, core.NewNotAllowedError(core.HttpPermission{
				Kind_:  permkind.Read,
				Entity: core.URL("https://example.com/index.html"),
			}))
			assert.Nil(t, res)
		})

		t.Run("lifetime job accessing self", func(t *testing.T) {
			code := `{ 
				a: 1
				lifetimejob #job { self.a = 2 } 
			}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			obj := res.(*core.Object)
			if !assert.Len(t, obj.JobInstances(), 1) {
				return
			}

			jobInstance := obj.JobInstances()[0]
			expected := obj.Prop(state.Ctx, "").(*core.List).At(state.Ctx, 0)

			assert.Equal(t, expected, jobInstance.Job())
			//assert.Equal(t, bytecodeEval, jobInstance.thread.useBytecode)

			time.Sleep(time.Millisecond)
		})

		t.Run("lifetime job accessing patterns defined in parent state", func(t *testing.T) {
			code := `
				pattern p = 1
				return { 
					a: []
					lifetimejob #job { self.a = [%p, %int] } 
				}
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Ctx.AddNamedPattern("int", core.INT_PATTERN)
			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			obj := res.(*core.Object)

			time.Sleep(100 * time.Millisecond) //wait for job to finish

			assert.Equal(t, core.NewWrappedValueList(
				state.Ctx.ResolveNamedPattern("p"),
				state.Ctx.ResolveNamedPattern("int"),
			), obj.Prop(state.Ctx, "a"))
		})
	})

	t.Run("record literal", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("empty", func(t *testing.T) {
			code := `#{}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewEmptyRecord(), res)
		})

		t.Run("single property", func(t *testing.T) {
			code := `#{a:1}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewRecordFromMap(core.ValMap{"a": core.Int(1)}), res)
		})

		t.Run("several properties", func(t *testing.T) {
			code := `#{a:1,b:2}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewRecordFromMap(core.ValMap{"a": core.Int(1), "b": core.Int(2)}), res)
		})

		t.Run("one element", func(t *testing.T) {
			code := `#{1}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewRecordFromMap(core.ValMap{
				"": core.NewTuple([]core.Serializable{core.Int(1)}),
			}), res)
		})

		t.Run("two elements", func(t *testing.T) {
			code := `#{1, 2}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewRecordFromMap(core.ValMap{
				"": core.NewTuple([]core.Serializable{core.Int(1), core.Int(2)}),
			}), res)
		})

		t.Run("one element and a property", func(t *testing.T) {
			code := `#{1, a: 1}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewRecordFromMap(core.ValMap{
				"":  core.NewTuple([]core.Serializable{core.Int(1)}),
				"a": core.Int(1),
			}), res)
		})
	})

	t.Run("dictionary literal", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("literal only keys", func(t *testing.T) {
			code := `:{"name": "foo", ./path: "bar"}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewDictionary(map[string]core.Serializable{
				`"name"`:                   core.String(`foo`),
				`{"path__value":"./path"}`: core.String(`bar`),
			}), res)
		})

		t.Run("variable key", func(t *testing.T) {
			code := `
				k1 = "name"
				k2 = 1
				return :{k1: "foo", k2: "bar"}
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewDictionary(map[string]core.Serializable{
				`"name"`:           core.String(`foo`),
				`{"int__value":1}`: core.String(`bar`),
			}), res)
		})

	})

	t.Run("list literal", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("empty list literal", func(t *testing.T) {
			code := `[]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewWrappedValueList(), res)
		})

		t.Run("[integer]", func(t *testing.T) {
			code := `[1]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewWrappedValueList(core.Int(1)), res)
		})

		t.Run("[integer,integer]", func(t *testing.T) {
			code := `[1,2]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewWrappedValueList(core.Int(1), core.Int(2)), res)
		})

		t.Run("[...[integer]]", func(t *testing.T) {
			code := `[...[1]]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewWrappedValueList(core.Int(1)), res)
		})

		t.Run("[integer, ...[integer]]", func(t *testing.T) {
			code := `[0, ...[1]]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewWrappedValueList(core.Int(0), core.Int(1)), res)
		})
	})

	t.Run("tuple literal", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("empty", func(t *testing.T) {
			code := `#[]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewTuple([]core.Serializable{}), res)
		})

		t.Run("[integer]", func(t *testing.T) {
			code := `#[1]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewTuple([]core.Serializable{core.Int(1)}), res)
		})

		t.Run("[integer,integer]", func(t *testing.T) {
			code := `#[1,2]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewTuple([]core.Serializable{core.Int(1), core.Int(2)}), res)
		})

		t.Run("[...#[integer]]", func(t *testing.T) {
			code := `#[...#[1]]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewTuple([]core.Serializable{core.Int(1)}), res)
		})

		t.Run("[integer, ...#[integer]]", func(t *testing.T) {
			code := `#[0, ...#[1]]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewTuple([]core.Serializable{core.Int(0), core.Int(1)}), res)
		})
	})

	t.Run("multi assignement", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("variable count == length", func(t *testing.T) {
			code := `
				assign a b = [1, 2]
				return [$a, $b]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewWrappedValueList(core.Int(1), core.Int(2)), res)
		})

		t.Run("variable count > length", func(t *testing.T) {
			code := `assign a b = [1]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.Error(t, err)
			assert.Nil(t, res)
		})

		t.Run("nillable: variable count > length", func(t *testing.T) {
			code := `
				assign? a b = [1]
				return [$a, $b]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.EqualValues(t, core.NewWrappedValueList(core.Int(1), core.Nil), res)
		})
	})

	t.Run("if statement", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("condition is true", func(t *testing.T) {
			code := `if true { return 1 }`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.Int(1), res)
		})

		t.Run("condition is false", func(t *testing.T) {
			code := `
				a = 0
				if false { 
					$a = 1 
				}
				return $a
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.Int(0), res)
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
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewWrappedValueList(core.Int(0), core.Int(1)), res)
		})

		t.Run("if-else-if, condition is false, condition of inner if is true", func(t *testing.T) {
			code := `
				a = 0
				b = 0
				if false { 
					$a = 1 
				} else if true { 
					$b = 1 
				}
				return [$a, $b]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewWrappedValueList(core.Int(0), core.Int(1)), res)
		})

		t.Run("if-else-if-else, condition is false, condition of inner if is false", func(t *testing.T) {
			code := `
				a = 0
				b = 0
				if false { 
					$a = 1 
				} else if false { 
					$b = -1
				} else {
					$b = 1
				}
				return [$a, $b]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewWrappedValueList(core.Int(0), core.Int(1)), res)
		})
	})

	t.Run("if expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("true condition", func(t *testing.T) {
			code := `(if true 1)`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.Int(1), res)
		})

		t.Run("false condition", func(t *testing.T) {
			code := `(if false 1)`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.Nil, res)
		})

		t.Run("if-else, false condition", func(t *testing.T) {
			code := `(if false 1 else 2)`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.Int(2), res)
		})
	})

	t.Run("for statement", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		testCases := []struct {
			input           string
			result          core.Value
			globals         func(ctx *core.Context) map[string]core.Value
			doSymbolicCheck bool
		}{
			{
				input: `
					c = 0
					for i, e in [] {
						$c = 1
					}
					return $c
				`,
				result:          core.Int(0),
				doSymbolicCheck: true,
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
				result:          core.NewWrappedValueList(core.Int(0), core.Int(5)),
				doSymbolicCheck: true,
			},
			{
				input: `
				c = 0
				for e in [5] {
					c = $e
				}
				return $c
			`,
				result:          core.Int(5),
				doSymbolicCheck: true,
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
				result:          core.NewWrappedValueList(core.Int(1), core.Int(11)),
				doSymbolicCheck: true,
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
				result:          core.NewWrappedValueList(core.Int(1), core.Int(11)),
				doSymbolicCheck: true,
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
				result:          core.NewWrappedValueList(core.Int(1), core.Int(5)),
				doSymbolicCheck: true,
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
				result:          core.NewWrappedValueList(core.Int(1), core.Int(6)),
				doSymbolicCheck: true,
			},
			{
				input: `
				c = 0
				for (1 .. 2) {
					c = ($c + 1)
				}
				return $c
			`,
				result:          core.Int(2),
				doSymbolicCheck: true,
			},
			{
				input: `
				c = 0
				pattern p = %| 1 | 3
				for %p n in [0 1 2 3] {
					c = ($c + $n)
				}
				return $c
			`,
				result:          core.Int(4),
				doSymbolicCheck: true,
			},
			{
				input: `
				c = 0
				indexSum = 0

				pattern i = 3
				pattern p = %| 1 | 3
				for %i i, %p n in [0 1 2 3] {
					c = (c + n)
					indexSum = (indexSum + i)
				}
				return [c, indexSum]
			`,
				result:          core.NewWrappedValueList(core.Int(3), core.Int(3)),
				doSymbolicCheck: true,
			},

			{
				input: `
				for (1 .. 2) {
					1
				}
			`,
				result: core.Nil,
			},
			{
				input: `
				for e in (1 .. 2) {
					1
				}
			`,
				result: core.Nil,
			},

			{
				input: `
				for i, e in (1 .. 2) {
					1
				}
			`,
				result: core.Nil,
			},
			{
				input: `
					elements = []
					for e in streamable {
						elements.append(e)
					}
					return elements
				`,
				globals: func(ctx *core.Context) map[string]core.Value {
					watcher := core.NewGenericWatcher(core.WatcherConfiguration{Filter: core.ANYVAL_PATTERN})
					watcher.InformAboutAsync(ctx, core.String("a"))
					watcher.InformAboutAsync(ctx, core.String("b"))

					go func() {
						time.Sleep(10 * time.Millisecond)
						watcher.Stop()
					}()
					return map[string]core.Value{
						"streamable": core.StreamSource(watcher),
					}
				},
				result: core.NewWrappedValueList(core.String("a"), core.String("b")),
			},
			{
				input: `
					elements = []
					for e in streamable {
						elements.append(e)
						break
					}
					return elements
				`,
				globals: func(ctx *core.Context) map[string]core.Value {
					watcher := core.NewGenericWatcher(core.WatcherConfiguration{Filter: core.ANYVAL_PATTERN})
					watcher.InformAboutAsync(ctx, core.String("a"))
					watcher.InformAboutAsync(ctx, core.String("b"))

					go func() {
						time.Sleep(10 * time.Millisecond)
						watcher.Stop()
					}()
					return map[string]core.Value{
						"streamable": core.StreamSource(watcher),
					}
				},
				result: core.NewWrappedValueList(core.String("a")),
			},
			{
				input: `
					for chunked chunk in streamable {
						return chunk.data
					}
				`,
				globals: func(ctx *core.Context) map[string]core.Value {
					watcher := core.NewGenericWatcher(core.WatcherConfiguration{Filter: core.ANYVAL_PATTERN})
					watcher.InformAboutAsync(ctx, core.String("a"))
					watcher.InformAboutAsync(ctx, core.String("b"))

					go func() {
						time.Sleep(10 * time.Millisecond)
						watcher.Stop()
					}()
					return map[string]core.Value{
						"streamable": core.StreamSource(watcher),
					}
				},
				result: core.NewWrappedValueList(core.String("a"), core.String("b")),
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
				globals: func(ctx *core.Context) map[string]core.Value {
					watcher := core.NewGenericWatcher(core.WatcherConfiguration{Filter: core.ANYVAL_PATTERN})
					watcher.InformAboutAsync(ctx, core.String("a"))
					watcher.InformAboutAsync(ctx, core.String("b"))

					go func() {
						time.Sleep(10 * time.Millisecond)
						watcher.Stop()
					}()
					return map[string]core.Value{
						"streamable": core.StreamSource(watcher),
					}
				},
				result: core.NewWrappedValueList(core.String("a"), core.String("b")),
			},
			{
				input: `
					data = []
					for chunked chunk in streamable {
						data.append(chunk.data)
					}
					return data
				`,
				globals: func(ctx *core.Context) map[string]core.Value {
					return map[string]core.Value{
						"streamable": core.NewElementsStream(
							[]core.Value{core.String("a"), core.String("b"), core.String("c"), core.String("d")},
							nil,
						),
					}
				},
				result: core.NewWrappedValueList(
					core.NewWrappedValueList(core.String("a"), core.String("b")),
					core.NewWrappedValueList(core.String("c"), core.String("d")),
				),
			},
			//TODO: add more tests with EOS error
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				if testCase.globals != nil {
					for k, v := range testCase.globals(state.Ctx) {
						state.Globals.Set(k, v)
					}
				}

				res, err := Eval(testCase.input, state, testCase.doSymbolicCheck)
				assert.NoError(t, err)
				assert.EqualValues(t, testCase.result, res)
			})
		}

	})

	t.Run("for expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		testCases := []struct {
			input           string
			result          core.Value
			globals         func(ctx *core.Context) map[string]core.Value
			doSymbolicCheck bool
		}{
			{
				input:           `(for i, e in []: i)`,
				result:          core.NewWrappedValueList(),
				doSymbolicCheck: true,
			},
			{
				input: `(for i, e in [5]: [i, e])`,
				result: core.NewWrappedValueList(
					core.NewWrappedValueList(core.Int(0), core.Int(5)),
				),
				doSymbolicCheck: true,
			},
			{
				input:           `(for e in [5]: e)`,
				result:          core.NewWrappedValueList(core.Int(5)),
				doSymbolicCheck: true,
			},
			{
				input: `(for i, e in [5, 6]: [i, e])`,
				result: core.NewWrappedValueList(
					core.NewWrappedValueList(core.Int(0), core.Int(5)),
					core.NewWrappedValueList(core.Int(1), core.Int(6)),
				),
				doSymbolicCheck: true,
			},
			{
				input: `(for i, e in 5..6: [i, e])`,
				result: core.NewWrappedValueList(
					core.NewWrappedValueList(core.Int(0), core.Int(5)),
					core.NewWrappedValueList(core.Int(1), core.Int(6)),
				),
				doSymbolicCheck: true,
			},
			{
				input: `
					pattern p = %| 1 | 3
					return (for %p n in [0, 1, 2, 3]: n)
				`,
				result:          core.NewWrappedValueList(core.Int(1), core.Int(3)),
				doSymbolicCheck: true,
			},
			{
				input: `
					pattern i = 3
					pattern p = %| 1 | 3
					return (for %i i, %p n in [0, 1, 2, 3]: [i, n])
				`,
				result: core.NewWrappedValueList(
					core.NewWrappedValueList(core.Int(3), core.Int(3)),
				),
				doSymbolicCheck: true,
			},
			{
				input: `(for e in streamable: e)`,
				globals: func(ctx *core.Context) map[string]core.Value {
					watcher := core.NewGenericWatcher(core.WatcherConfiguration{Filter: core.ANYVAL_PATTERN})
					watcher.InformAboutAsync(ctx, core.String("a"))
					watcher.InformAboutAsync(ctx, core.String("b"))

					go func() {
						time.Sleep(5 * time.Millisecond)
						watcher.Stop()
					}()
					return map[string]core.Value{
						"streamable": core.StreamSource(watcher),
					}
				},
				result: core.NewWrappedValueList(core.String("a"), core.String("b")),
			},
			{
				input: `(for chunked chunk in streamable: chunk.data)`,
				globals: func(ctx *core.Context) map[string]core.Value {
					watcher := core.NewGenericWatcher(core.WatcherConfiguration{Filter: core.ANYVAL_PATTERN})
					watcher.InformAboutAsync(ctx, core.String("a"))
					watcher.InformAboutAsync(ctx, core.String("b"))

					go func() {
						time.Sleep(5 * time.Millisecond)
						watcher.Stop()
					}()
					return map[string]core.Value{
						"streamable": core.StreamSource(watcher),
					}
				},
				result: core.NewWrappedValueList(
					core.NewWrappedValueList(core.String("a"), core.String("b")),
				),
			},
			{
				input: `(for chunked chunk in streamable: chunk.data)`,
				globals: func(ctx *core.Context) map[string]core.Value {
					return map[string]core.Value{
						"streamable": core.NewElementsStream(
							[]core.Value{core.String("a"), core.String("b"), core.String("c"), core.String("d")},
							nil,
						),
					}
				},
				result: core.NewWrappedValueList(
					core.NewWrappedValueList(core.String("a"), core.String("b")),
					core.NewWrappedValueList(core.String("c"), core.String("d")),
				),
			},
			//TODO: add more tests with EOS error
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				if testCase.globals != nil {
					for k, v := range testCase.globals(state.Ctx) {
						state.Globals.Set(k, v)
					}
				}

				res, err := Eval(testCase.input, state, testCase.doSymbolicCheck)
				assert.NoError(t, err)
				assert.EqualValues(t, testCase.result, res)
			})
		}

	})

	t.Run("walk statement", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		GET_ENTRIES_CODE := `
			entries = []
			walk $dir entry {
				$entries.append($entry)
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
			result func(tempDir string, tempDirPath core.Path) core.Value
			before func(tempDir string, tempDirPath core.Path)
		}{
			{
				//empty dir
				input: GET_ENTRIES_CODE,
				result: func(tempDir string, tempDirPath core.Path) core.Value {
					return core.NewWrappedValueList(
						core.NewObjectFromMapNoInit(core.ValMap{
							"name":          core.String(filepath.Base(tempDir)),
							"path":          tempDirPath,
							"is-dir":        core.True,
							"is-regular":    core.False,
							"is-walk-start": core.True,
						}),
					)
				},
			},
			{
				name:  "dir with single regular file",
				input: GET_ENTRIES_CODE,
				before: func(tempDir string, tempDirPath core.Path) {
					regularFilePath := filepath.Join(tempDir, regularFilename)
					os.WriteFile(regularFilePath, nil, 0o400)
				},
				result: func(tempDir string, tempDirPath core.Path) core.Value {
					regularFilePath := filepath.Join(tempDir, regularFilename)

					return core.NewWrappedValueList(
						core.NewObjectFromMapNoInit(core.ValMap{
							"name":          core.String(filepath.Base(tempDir)),
							"path":          tempDirPath,
							"is-dir":        core.True,
							"is-regular":    core.Bool(false),
							"is-walk-start": core.True,
						}),
						core.NewObjectFromMapNoInit(core.ValMap{
							"name":          core.String(regularFilename),
							"path":          core.Path(regularFilePath),
							"is-dir":        core.Bool(false),
							"is-regular":    core.True,
							"is-walk-start": core.Bool(false),
						}),
					)
				},
			},
			{

				name:  "dir with a regular file and an empty subdirectory",
				input: GET_ENTRIES_CODE,
				before: func(tempDir string, tempDirPath core.Path) {
					regularFilePath := filepath.Join(tempDir, regularFilename)
					subdirPath := filepath.Join(tempDir, subdirName)
					os.WriteFile(regularFilePath, nil, 0o400)
					os.Mkdir(subdirPath, 0x500)
				},
				result: func(tempDir string, tempDirPath core.Path) core.Value {
					regularFilePath := filepath.Join(tempDir, regularFilename)
					subdirPath := filepath.Join(tempDir, subdirName)

					return core.NewWrappedValueList(
						core.NewObjectFromMapNoInit(core.ValMap{
							"name":          core.String(filepath.Base(tempDir)),
							"path":          core.Path(tempDir + "/"),
							"is-dir":        core.True,
							"is-regular":    core.Bool(false),
							"is-walk-start": core.True,
						}),
						core.NewObjectFromMapNoInit(core.ValMap{
							"name":          core.String(regularFilename),
							"path":          core.Path(regularFilePath),
							"is-dir":        core.Bool(false),
							"is-regular":    core.True,
							"is-walk-start": core.Bool(false),
						}),
						core.NewObjectFromMapNoInit(core.ValMap{
							"name":          core.String(subdirName),
							"path":          core.Path(subdirPath + "/"),
							"is-dir":        core.True,
							"is-regular":    core.Bool(false),
							"is-walk-start": core.Bool(false),
						}),
					)
				},
			},
			{
				name: "dir with a regular file and an empty subdirectory : prune when regular file is encountered",
				input: `
					entries = []
					walk dir entry {
						$entries.append($entry)
						if $entry.is-regular {
							prune
						}
					}
					return $entries
				`,
				before: func(tempDir string, tempDirPath core.Path) {
					regularFilePath := filepath.Join(tempDir, regularFilename)
					subdirPath := filepath.Join(tempDir, subdirName)
					os.WriteFile(regularFilePath, nil, 0o400)
					os.Mkdir(subdirPath, 0x500)
				},
				result: func(tempDir string, tempDirPath core.Path) core.Value {
					regularFilePath := filepath.Join(tempDir, regularFilename)

					return core.NewWrappedValueList(
						core.NewObjectFromMapNoInit(core.ValMap{
							"name":          core.String(filepath.Base(tempDir)),
							"path":          core.Path(tempDir + "/"),
							"is-dir":        core.True,
							"is-regular":    core.Bool(false),
							"is-walk-start": core.True,
						}),
						core.NewObjectFromMapNoInit(core.ValMap{
							"name":          core.String(regularFilename),
							"path":          core.Path(regularFilePath),
							"is-dir":        core.Bool(false),
							"is-regular":    core.True,
							"is-walk-start": core.Bool(false),
						}),
					)
				},
			},
			{
				name: "dir with to subdirectories : prune when one of the dir is encountered",
				input: `
					entries = []
					walk dir entry {
						$entries.append($entry)
						if $entry.is-walk-start {
							continue
						}
						if $entry.is-dir {
							prune
						}
					}
					return $entries
				`,
				before: func(tempDir string, tempDirPath core.Path) {
					subdir1Path := filepath.Join(tempDir, subdir1Name)
					subdir2Path := filepath.Join(tempDir, subdir2Name)

					os.Mkdir(subdir1Path, 0x500)
					os.Mkdir(subdir2Path, 0x500)
				},
				result: func(tempDir string, tempDirPath core.Path) core.Value {
					subdir1Path := filepath.Join(tempDir, subdir1Name)
					subdir2Path := filepath.Join(tempDir, subdir2Name)

					return core.NewWrappedValueList(
						core.NewObjectFromMapNoInit(core.ValMap{
							"name":          core.String(filepath.Base(tempDir)),
							"path":          core.Path(tempDir + "/"),
							"is-dir":        core.True,
							"is-regular":    core.Bool(false),
							"is-walk-start": core.True,
						}),
						core.NewObjectFromMapNoInit(core.ValMap{
							"name":          core.String(subdir1Name),
							"path":          core.Path(subdir1Path + "/"),
							"is-dir":        core.True,
							"is-regular":    core.Bool(false),
							"is-walk-start": core.Bool(false),
						}),
						core.NewObjectFromMapNoInit(core.ValMap{
							"name":          core.String(subdir2Name),
							"path":          core.Path(subdir2Path + "/"),
							"is-dir":        core.True,
							"is-regular":    core.Bool(false),
							"is-walk-start": core.Bool(false),
						}),
					)
				},
			},
			{
				name: "dir with to subdirectories : break when one of the dir is encountered",
				input: `
					entries = []
					walk dir entry {
						$entries.append($entry)
						if $entry.is-walk-start {
							continue
						}
						if $entry.is-dir {
							break
						}
					}
					return $entries
				`,
				before: func(tempDir string, tempDirPath core.Path) {
					subdir1Path := filepath.Join(tempDir, subdir1Name)
					subdir2Path := filepath.Join(tempDir, subdir2Name)

					os.Mkdir(subdir1Path, 0x500)
					os.Mkdir(subdir2Path, 0x500)
				},
				result: func(tempDir string, tempDirPath core.Path) core.Value {
					subdir1Path := filepath.Join(tempDir, subdir1Name)

					return core.NewWrappedValueList(
						core.NewObjectFromMapNoInit(core.ValMap{
							"name":          core.String(filepath.Base(tempDir)),
							"path":          core.Path(tempDir + "/"),
							"is-dir":        core.True,
							"is-regular":    core.Bool(false),
							"is-walk-start": core.True,
						}),
						core.NewObjectFromMapNoInit(core.ValMap{
							"name":          core.String(subdir1Name),
							"path":          core.Path(subdir1Path + "/"),
							"is-dir":        core.True,
							"is-regular":    core.Bool(false),
							"is-walk-start": core.Bool(false),
						}),
					)
				},
			},
			{
				name: "for statement in body",
				input: `
					a = 0
					walk dir entry {
						for i in 1..10 {
							a = (a + 1)
							break
						}
						a = (a + 2)
						break
					}
					return a
				`,
				before: func(tempDir string, tempDirPath core.Path) {},
				result: func(tempDir string, tempDirPath core.Path) core.Value {
					return core.Int(3)
				},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				tempDir := t.TempDir()
				tempDirPath := core.Path(tempDir + "/")

				if testCase.before != nil {
					testCase.before(tempDir, tempDirPath)
				}

				ctx := core.NewContext(core.ContextConfig{
					Permissions: []core.Permission{
						core.GlobalVarPermission{permkind.Read, "*"},
						core.GlobalVarPermission{permkind.Update, "*"},
						core.GlobalVarPermission{permkind.Create, "*"},
						core.GlobalVarPermission{permkind.Use, "*"},
						core.FilesystemPermission{permkind.Read, core.PathPattern(tempDirPath + "...")},
					},
					Filesystem: newOsFilesystem(),
				})
				defer ctx.CancelGracefully()

				state := core.NewGlobalState(ctx, map[string]core.Value{
					"dir": tempDirPath,
				})
				res, err := Eval(testCase.input, state, true)
				assert.NoError(t, err)

				expectedResult := testCase.result(tempDir, tempDirPath)
				assert.Equal(t, expectedResult, res)
			})
		}

	})

	t.Run("switch statement", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		testCases := []struct {
			name   string
			input  string
			result core.Value
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
				result: core.Int(1),
			},
			{
				name: "single case (that matches) and defaultcase",
				input: `
				a = 0
				switch 0 { 
					0 { a = 1 } 
					defaultcase { a = 2 }
				}
				return a
			`,
				result: core.Int(1),
			},
			{
				name: "single case (that does not match) and defaultcase",
				input: `
				a = 0
				switch 0 { 
					1 { a = 1 } 
					defaultcase { a = 2 }
				}
				return a
			`,
				result: core.Int(2),
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
				result: core.NewWrappedValueList(core.Int(1), core.Int(0)),
			},
			{
				name: "two cases and defaultcase: first matches",
				input: `
				a = 0; 
				b = 0; 
				switch 0 { 
					0 { a = 1 } 
					1 { b = 1} 
					defaultcase { a = 2; b = 2 }
				}; 
				return [$a,$b]
			`,
				result: core.NewWrappedValueList(core.Int(1), core.Int(0)),
			},
			{
				name: "two cases and defaultcase: no match",
				input: `
				a = 0; 
				b = 0; 
				switch 0 { 
					1 { a = 1 } 
					2 { b = 1} 
					defaultcase { a = 2; b = 2 }
				}; 
				return [$a,$b]
			`,
				result: core.NewWrappedValueList(core.Int(2), core.Int(2)),
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
				result: core.NewWrappedValueList(core.Int(0), core.Int(1)),
			},
			{
				name: "two cases and defaultcase: second matches",
				input: `
				a = 0
				b = 0 
				switch 1 { 
					0 { a = 1 } 
					1 { b = 1 } 
					defaultcase { a = 2; b = 2 }
				}; 
				return [$a,$b]
			`,
				result: core.NewWrappedValueList(core.Int(0), core.Int(1)),
			},
			{
				name: "stack check: 2 cases",
				input: `
					switch 1 { 
						1 {}
						2 {}
					}; 
				`,
				result: core.Nil,
			},
			{
				name: "stack check: 2 cases + default case",
				input: `
				switch 1 { 
						1 {}
						2 {}
						defaultcase {}
					}; 
				`,
				result: core.Nil,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				res, err := Eval(testCase.input, state, false)
				assert.NoError(t, err)
				assert.Equal(t, testCase.result, res)
			})
		}
	})

	t.Run("switch expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		testCases := []struct {
			name   string
			input  string
			result core.Value
		}{
			{
				name:   "no cases",
				input:  `return switch 1 { }`,
				result: core.Nil,
			},
			{
				name: "single case (that matches)",
				input: `
					return switch 0 { 
						0 => 1
					}
				`,
				result: core.Int(1),
			},
			{
				name: "single case (that matches) and defaultcase",
				input: `
					return switch 0 { 
						0 => 1
						defaultcase => 2
					}
				`,
				result: core.Int(1),
			},
			{
				name: "single case (that does not match) and defaultcase",
				input: `
					return switch 0 { 
						1 => 0
						defaultcase => 2
					}
				`,
				result: core.Int(2),
			},
			{
				name: "two cases: first matches",
				input: `
					return switch 0 { 
						0 => 0
						1 => 1
					}; 
				`,
				result: core.Int(0),
			},
			{
				name: "two cases and defaultcase: first matches",
				input: `
					return switch 0 { 
						0 => 0
						1 => 1
						defaultcase => 2
					}; 
				`,
				result: core.Int(0),
			},
			{
				name: "two cases and defaultcase: no match",
				input: `
					return switch 2 { 
						0 => 0
						1 => 1
						defaultcase => 2
					}; 
				`,
				result: core.Int(2),
			},
			{
				name: "two cases: second matches",
				input: `
					return switch 1 { 
						0 => 0
						1 => 1
					}; 
			`,
				result: core.Int(1),
			},
			{
				name: "two cases and defaultcase: second matches",
				input: `
					return switch 1 { 
						0 => 0
						1 => 1
						defaultcase => 2
					}; 
				`,
				result: core.Int(1),
			},
			{
				name: "stack check: 2 cases",
				input: `
					if (switch 1 { 
						1 => true
						2 => true
					}) {}
				`,
				result: core.Nil,
			},
			{
				name: "stack check: 2 cases + default case",
				input: `
					if (switch 1 { 
						1 => true
						2 => true
						defaultcase => true
					}) {}
				`,
				result: core.Nil,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				res, err := Eval(testCase.input, state, false)
				assert.NoError(t, err)
				assert.Equal(t, testCase.result, res)
			})
		}
	})

	t.Run("match statement", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		testCases := []struct {
			name   string
			input  string
			result core.Value
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
				result: core.NewWrappedValueList(core.Int(1), core.Int(0)),
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
				result: core.NewWrappedValueList(core.String("user"), core.Int(0)),
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
				result: core.NewWrappedValueList(core.Int(0), core.Int(1)),
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
				result: core.NewWrappedValueList(core.Int(0), core.Int(1)),
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
				result: core.NewWrappedValueList(core.Int(0), core.Int(1)),
			},
			{
				name: "stack check: 2 cases",
				input: `
					match 1 { 
						1 {}
						2 {}
					}; 
				`,
				result: core.Nil,
			},
			{
				name: "stack check: 2 cases + default case",
				input: `
					match 1 { 
						1 {}
						2 {}
						defaultcase {}
					}; 
				`,
				result: core.Nil,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				res, err := Eval(testCase.input, state, false)
				assert.NoError(t, err)
				assert.Equal(t, testCase.result, res)
			})
		}
	})

	t.Run("match expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		testCases := []struct {
			name   string
			input  string
			result core.Value
		}{
			{
				name:   "no cases",
				input:  `return match 1 { }`,
				result: core.Nil,
			},
			{
				name: "patterns : two cases (first matches)",
				input: `
					return match / { 
						%/* => 1
						%/e* => 2
					}
				`,
				result: core.Int(1),
			},
			{
				name: "group patterns : two cases (first one matches)",
				input: `
					return match /home/user { 
						%/home/{:username} m => m.username
						%/hom/{:username} => 1
					}; 
				`,
				result: core.String("user"),
			},
			{
				name: "group patterns : two cases (second one matches)",
				input: `
					return match /e { 
						%/f* => 1
						%/e* => 2
					} 
				`,
				result: core.Int(2),
			},
			{
				name: "equality : two cases (second one matches)",
				input: `
					return match /e { 
						%/f* => 1
						/e => 2
					}
				`,
				result: core.Int(2),
			},
			{
				name: "seconde case is not a simple value but is statically known",
				input: `
					return match {a:1} { 
						%/f* => 1
						({a:1}) => 2
					}; 
				`,
				result: core.Int(2),
			},
			{
				name: "stack check: 2 cases",
				input: `
					if (match 1 { 
						1 => true
						2 => true
					}) {}
				`,
				result: core.Nil,
			},
			{
				name: "stack check: 2 cases + default case",
				input: `
					if (match 1 { 
						1 => true
						2 => true
						defaultcase => true
					}) {}
				`,
				result: core.Nil,
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				res, err := Eval(testCase.input, state, false)
				assert.NoError(t, err)
				assert.Equal(t, testCase.result, res)
			})
		}
	})

	t.Run("integer range literal ", func(t *testing.T) {
		t.Run("with upper bound", func(t *testing.T) {
			code := `return 1..2`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewIntRange(1, 2), res)
		})

		t.Run("without upper bound", func(t *testing.T) {
			code := `return 1..`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewIntRange(1, math.MaxInt64), res)
		})
	})

	t.Run("float range literal ", func(t *testing.T) {
		t.Run("with upper bound", func(t *testing.T) {
			code := `return 1.0..2.0`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewFloatRange(1.0, 2.0, true), res)
		})

		t.Run("without upper bound", func(t *testing.T) {
			code := `return 1.0..`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewFloatRange(1, math.MaxFloat64, true), res)
		})
	})

	t.Run("quantity range literal ", func(t *testing.T) {
		t.Run("with upper bound", func(t *testing.T) {
			code := `return 1B..2B`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewQuantityRange(core.ByteCount(1), core.ByteCount(2), true), res)
		})

		t.Run("without upper bound", func(t *testing.T) {
			code := `return 1B..`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewQuantityRange(
				core.ByteCount(1),
				core.GetQuantityTypeMaxValue(core.ByteCount(0)),
				true,
			), res)
		})
	})

	t.Run("upper bound range expression ", func(t *testing.T) {
		t.Run("integer ", func(t *testing.T) {
			code := `return ..10`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.NewUnknownStartIntRange(10), res)
		})

		t.Run("quantity", func(t *testing.T) {
			code := `return ..10s`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)

			assert.Equal(t, core.NewUnknownStartQuantityRange(
				core.Duration(10*time.Second),
				true,
			), res)
		})
	})

	t.Run("rune range expression", func(t *testing.T) {
		code := `'a'..'z'`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()
		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.RuneRange{'a', 'z'}, res)
	})

	t.Run("sequence string pattern", func(t *testing.T) {

		t.Run("single element", func(t *testing.T) {
			code := `
				pattern s = str( 'a'..'z' )
				return %s
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.IsType(t, (*core.SequenceStringPattern)(nil), res)
			patt := res.(*core.SequenceStringPattern)

			elements := patt.Elements()
			assert.Len(t, elements, 1)
		})

		t.Run("single element: integer range with no upper bound", func(t *testing.T) {
			code := `
				pattern s = str( 1.. )
				return %s
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.IsType(t, (*core.SequenceStringPattern)(nil), res)
			patt := res.(*core.SequenceStringPattern)

			elements := patt.Elements()
			assert.Len(t, elements, 1)

			expectedPattern := core.NewIntRangeStringPattern(
				1,
				math.MaxInt64,
				parse.FindNode(state.Module.MainChunk.Node, (*parse.IntegerRangeLiteral)(nil), nil),
			)
			assert.Equal(t, expectedPattern, elements[0])
		})

		t.Run("single element: multiline string literal", func(t *testing.T) {
			code := "pattern s = str( `a` )\n" +
				"return %s"

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.IsType(t, (*core.SequenceStringPattern)(nil), res)
			patt := res.(*core.SequenceStringPattern)
			elements := patt.Elements()

			assert.Len(t, elements, 1)
		})
	})

	t.Run("function expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("empty", func(t *testing.T) {
			code := `fn(){}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			assert.IsType(t, &core.InoxFunction{}, res)
		})

		t.Run("captured locals", func(t *testing.T) {
			code := `
				a = 1
				b = 2
				return fn[a,b](){}
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			assert.IsType(t, &core.InoxFunction{}, res)

			assert.ElementsMatch(t, []any{core.Int(1), core.Int(2)}, res.(*core.InoxFunction).CapturedLocals())
		})

		t.Run("captured locals should be thread safe", func(t *testing.T) {
			code := `
				obj = {}
				return fn[obj](){}
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			assert.IsType(t, &core.InoxFunction{}, res)
			captured := res.(*core.InoxFunction).CapturedLocals()[0]
			assert.IsType(t, captured, &core.Object{})
			assert.True(t, captured.(*core.Object).IsShared())
		})
	})

	t.Run("function declaration", func(t *testing.T) {
		t.Run("base case", func(t *testing.T) {
			code := `fn f(){}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			_, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Contains(t, state.Globals.Entries(), "f")
			assert.IsType(t, &core.InoxFunction{}, state.Globals.Get("f"))
		})

		t.Run("function that do not capture locals should be available before the declaration statement", func(t *testing.T) {
			code := `
				return f
				fn f(){}
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, true)
			if !assert.NoError(t, err) {
				return
			}
			assert.IsType(t, &core.InoxFunction{}, res)
		})

		t.Run("function that do not capture locals should be available before the declaration statement: inside and outside included files", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix

				return [g, f]
				fn g(){}
			`, map[string]string{
				"./dep.ix": `
					includable-file

					pattern _true = %(f == 0)
					fn f(){}
				`,
			})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := core.NewGlobalState(NewDefaultTestContext())
			state.Module = mod
			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, true)
			if !assert.NoError(t, err) {
				return
			}

			list := res.(*core.List)

			assert.IsType(t, &core.InoxFunction{}, list.At(state.Ctx, 0))
			assert.IsType(t, &core.InoxFunction{}, list.At(state.Ctx, 1))
		})

		t.Run("function that do not capture locals should be available before the declaration statement: in embedded module of a spawn expr", func(t *testing.T) {
			code := `
				thread = go do {
					return f
					fn f(){}
				}
				return thread.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, true)
			if !assert.NoError(t, err) {
				return
			}

			assert.IsType(t, &core.InoxFunction{}, res)
		})
	})

	t.Run("Inox function call", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		noargs := func() []core.Value { return nil }

		anError := core.NewError(errors.New("an error"), core.Nil)

		testCases := []struct {
			name                  string
			error                 bool
			input                 string
			result                core.Value
			checkResult           func(t *testing.T, result core.Value, state *core.GlobalState)
			isShared              bool
			isolatedCaseArguments func() []core.Value
			doAnalysis            bool
		}{
			{

				name: "call before function declaration: no captured locals",
				input: `
					return f()
					fn f(){
						return 1
					}
				`,
				result:     core.Int(1),
				doAnalysis: true,
			},
			{
				name:  "must call of a function returning an array of length 2 whose last element is an error",
				error: true,
				input: `
					fn f(){
						return Array(1, an-error)
					}
					return f!()
				`,
			},
			{
				name: "must call of a function returning an array of length 2 whose last element is nil",
				input: `
					fn f(){
						return Array(1, nil)
					}
					return f!()
				`,
				result: core.Int(1),
			},
			{
				name: "must call of a function returning nil",
				input: `
					fn f(){
						return nil
					}
					return f!()
				`,
				result: core.Nil,
			},
			{
				name:  "must call of a function returning an error",
				error: true,
				input: `
					fn f(){
						return an-error
					}
					return f!()
				`,
			},
			{
				name: "normal call of a function returning an array of length 2 whose last element is an error",
				input: `
					fn f(){
						return Array(1, an-error)
					}
					return f()
				`,
				result: core.NewArray(nil, core.Int(1), anError),
			},
			{
				name: "normal call of a function returning an array of length 2 whose last element is nil",
				input: `
					fn f(){
						return Array(1, nil)
					}
					return f()
				`,
				result: core.NewArray(nil, core.Int(1), core.Nil),
			},
			{
				name: "normal call of a function returning nil",
				input: `
					fn f(){
						return nil
					}
					return f()
				`,
				result: core.Nil,
			},
			{
				name: "normal of a function returning an error",
				input: `
					fn f(){
						return an-error
					}
					return f()
				`,
				result: anError,
			},
			{
				name: "declared void function",
				input: `
					fn f(){  }
					return f()
				`,
				result:                core.Nil,
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
				result:                core.Int(1),
			},
			{
				name: "declared function returning its sole argument",
				input: `
					fn f(a){
						return a
					}
					return f(1)
					`,
				isolatedCaseArguments: func() []core.Value { return []core.Value{core.Int(1)} },
				result:                core.Int(1),
			},
			{
				name: "declared function with a captured value",
				input: `
					a = 1
					fn[a] f(){ return a }
					return f()
				`,
				isolatedCaseArguments: noargs,
				result:                core.Int(1),
			},
			{
				name: "declared function with a captured value and a local",
				input: `
					a = 1
					fn[a] f(b){ return [a, b] }
					return f(2)
				`,
				isolatedCaseArguments: func() []core.Value { return []core.Value{core.Int(2)} },
				result:                core.NewWrappedValueList(core.Int(1), core.Int(2)),
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
				result:                core.NewWrappedValueList(core.Int(1), core.Int(2)),
			},
			{
				name: "declared function returning a function expression",
				input: `
					fn f() { return fn() => 1 }
					return f()
				`,
				isolatedCaseArguments: noargs,
				doAnalysis:            true,
				checkResult: func(t *testing.T, result core.Value, state *core.GlobalState) {
					assert.IsType(t, (*core.InoxFunction)(nil), result)
				},
			},
			{
				name: "declared arrow function",
				input: `
					fn f() => 1
					return f()
				`,
				isolatedCaseArguments: noargs,
				result:                core.Int(1),
			},
			{
				name: "declared arrow function returning its sole argument",
				input: `
					fn f(a) => a
					return f(1)
				`,
				isolatedCaseArguments: func() []core.Value { return []core.Value{core.Int(1)} },
				result:                core.Int(1),
			},
			{
				name: "declared arrow function with a captured value",
				input: `
					a = 1
					fn[a] f() => a
					return f()
				`,
				isolatedCaseArguments: noargs,
				result:                core.Int(1),
			},
			{
				name: "declared arrow function with a captured value and a local",
				input: `
					a = 1
					fn[a] f(b) => [a, b]
					return f(2)
				`,
				isolatedCaseArguments: func() []core.Value { return []core.Value{core.Int(2)} },
				result:                core.NewWrappedValueList(core.Int(1), core.Int(2)),
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
				result:                core.NewWrappedValueList(core.Int(1), core.Int(2)),
			},
			{
				name: "declared arrow function returning a function expression",
				input: `
					fn f() => fn() => 1
					return f()
				`,
				isolatedCaseArguments: noargs,
				doAnalysis:            true,
				checkResult: func(t *testing.T, result core.Value, state *core.GlobalState) {
					assert.IsType(t, (*core.InoxFunction)(nil), result)
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
						return Array($x, $y)
					}
					return f(1)
				`,
				result: core.NewArrayFrom(core.Int(1), core.NewArrayFrom()),
			},
			{
				name: "variadic function with many arguments",
				input: `
					fn f(x, ...y){
						return Array($x, $y)
					}
					return f(1, 2, 3)
				`,
				result: core.NewArrayFrom(core.Int(1), core.NewArrayFrom(core.Int(2), core.Int(3))),
			},
			{
				name: "variadic function with many arguments from a list spread argument",
				input: `
					fn f(x, ...y){
						return Array($x, $y)
					}
					return f(1, ...[2, 3])
				`,
				result: core.NewArrayFrom(core.Int(1), core.NewArrayFrom(core.Int(2), core.Int(3))),
			},
			{
				name: "variadic function with many arguments from an array spread argument",
				input: `
					fn f(x, ...y){
						return Array($x, $y)
					}
					return f(1, ...Array(2, 3))
				`,
				result: core.NewArrayFrom(core.Int(1), core.NewArrayFrom(core.Int(2), core.Int(3))),
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
				result:                core.NewWrappedValueList(core.Int(1), core.Int(2)),
			},
			{
				name: "recursive function",
				input: `
					fn factorial(i int) int {
						if (i == 0) {
							return 1
						}
						return (i * factorial( (i - 1) ))
					}
					return factorial(3)
				`,
				isolatedCaseArguments: func() []core.Value { return []core.Value{core.Int(3)} },
				result:                core.Int(6),
				doAnalysis:            true,
			},
			{
				name: "recursive function accessing a global",
				input: `
					globalvar a = 3
					fn rec(i %int) int {
						if (i == 0) {
							return 0
						}
						return (a + rec((i - 1)))
					}
					result = rec(2)
					return [result, a] # we also check that a is still accessible
				`,
				result:     core.NewWrappedValueList(core.Int(6), core.Int(3)),
				doAnalysis: true,
			},
			{
				name: "function calling a recursive function accessing a global",
				input: `
					globalvar a = 3
					fn rec(i %int) int {
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
				result:                core.NewWrappedValueList(core.Int(6), core.Int(3)),
				doAnalysis:            true,
			},
			{
				name: "extension method calling a recursive function accessing a global",
				input: `
					globalvar a = 3
					pattern o = {}

					fn rec(i %int) int {
						if (i == 0) {
							return 0
						}
						return (a + rec((i - 1)))
					}

					obj = {}


					extend o {
						f: fn(){
							return [rec(2), a] # we also check that a is still accessible
						}
					}
					
					return obj::f()
				`,
				result:     core.NewWrappedValueList(core.Int(6), core.Int(3)),
				doAnalysis: true,
			},
			{
				name: "return is in if statement",
				input: `
					fn f(){
						if true { return 1 }
					}
					return f()
				`,
				result: core.Int(1),
			},
			{
				name: "many calls of a void function with no parameters",
				input: strings.ReplaceAll(`
					fn f(){}
					many_calls
				`, "many_calls", strings.Repeat("f()\n", 100)),
				result: core.Nil,
			},
			{
				name: "external func : no parameters, no return value",
				input: `
					rt = go do { return fn(){} }

					f = rt.wait_result!()
					return f()
				`,
				result: core.Nil,
			},
			{
				name: "external func returning an integer",
				input: `
					lthread = go do {
						return fn(){ return 1 }
					}

					f = lthread.wait_result!()
					return f()
				`,
				isolatedCaseArguments: noargs,
				result:                core.Int(1),
			},
			{
				name: "external func returning an object",
				input: `
					lthread = go do { 
						return fn(){ return {} } 
					}

					f = lthread.wait_result!()
					return f()
				`,
				isShared:              true,
				isolatedCaseArguments: noargs,
				result:                core.NewObject(),
			},
			{
				name: "external func returning its argument, argument should be shared",
				input: `
					shared_value = fn(){}

					lthread = go do { 
						return fn(arg){ return arg } 
					}

					f = lthread.wait_result!()
					return f(shared_value)
				`,
				checkResult: func(t *testing.T, result core.Value, state *core.GlobalState) {
					if !result.(*core.InoxFunction).OriginStateEqual(state) {
						assert.Fail(t, "origin state of shared value is invalid")
					}
				},
			},
			{
				name: "external func : many calls of a void function with no parameters",
				input: strings.ReplaceAll(`
					lthread = go do { 
						return fn(){ } 
					}

					f = lthread.wait_result!()
					many_calls			
				`, "many_calls", strings.Repeat("f()\n", 100)),
				result: core.Nil,
			},
			// TODO
			// {
			// 	name: "extension method call within extension method call",
			// 	input: `
			// 		obj = {a: {b: 1}}

			// 		pattern B = {b: int}

			// 		pattern A = {a: B}

			// 		extend A {
			// 			getA: fn() => self.b::getB()
			// 		}

			// 		extend B {
			// 			getB: fn() => self.b
			// 		}

			// 		return obj::getA()
			// 	`,
			// 	result:          core.Int(1),
			// 	doSymbolicCheck: true,
			// },
			// {
			// 	name: "several extension method calls within extension method call",
			// 	input: `
			// 		obj = {a: {b: 1}}

			// 		pattern B = {b: int}

			// 		pattern A = {a: B}

			// 		extend A {
			// 			getA: fn() => [self.a, self.b::getB(), self.a, self.b::getB()]
			// 		}

			// 		extend B {
			// 			getB: fn() => self.b
			// 		}

			// 		return obj::getA()
			// 	`,
			// 	result:          core.NewWrappedValueList(core.Int(2), core.Int(1), core.Int(2), core.Int(1)),
			// 	doSymbolicCheck: true,
			//},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()

				state.Globals.Set("Array", core.WrapGoFunction(core.NewArray))
				state.Globals.Set("an-error", anError)

				state.Ctx.AddNamedPattern("int", core.INT_PATTERN)
				state.Ctx.AddNamedPattern("any", core.ANYVAL_PATTERN)

				res, err := Eval(testCase.input, state, testCase.doAnalysis)
				if testCase.error {
					if !assert.Error(t, err) {
						return
					}
					assert.Nil(t, res)
				} else {
					if !assert.NoError(t, err) {
						return
					}

					if testCase.checkResult != nil {
						testCase.checkResult(t, res, state)
					} else {
						expected := testCase.result
						if testCase.isShared && utils.Ret0(core.IsSharable(expected, state)) {
							expected = core.Share(expected.(core.PotentiallySharable), state)
						}
						assert.Equal(t, expected, res)
					}
				}
			})

			if bytecodeEval && testCase.isolatedCaseArguments != nil {
				t.Run("isolated_call_"+testCase.name, func(t *testing.T) {
					state := core.NewGlobalState(NewDefaultTestContext())
					defer state.Ctx.CancelGracefully()

					state.Globals.Set("Array", core.WrapGoFunction(core.NewArray))
					state.Globals.Set("an-error", anError)

					state.Ctx.AddNamedPattern("int", core.INT_PATTERN)
					state.Ctx.AddNamedPattern("any", core.ANYVAL_PATTERN)

					lastOpeningParenIndex := strings.LastIndexByte(testCase.input, '(')
					input := testCase.input[:lastOpeningParenIndex]

					val, err := Eval(input, state, testCase.doAnalysis)
					if !assert.NoError(t, err) {
						return
					}

					fn := val.(*core.InoxFunction)
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
							if testCase.isShared && utils.Ret0(core.IsSharable(expected, state)) {
								expected = core.Share(expected.(core.PotentiallySharable), state)
							}
							assert.Equal(t, expected, res)
						}
					}
				})
			}
		}
	})

	t.Run("Go function call", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		var symbolicRegistrationLock sync.Mutex

		testCases := []struct {
			name            string
			error           bool
			input           string
			globalVariables map[string]core.Value
			makeGlobals     func(t *testing.T) map[string]core.Value
			result          core.Value
		}{
			{
				name:  "variadic: arg count < non-variadic-param-count",
				input: "gofunc()",
				error: true,
				globalVariables: map[string]core.Value{
					"gofunc": core.WrapGoFunction(func(ctx *core.Context, x core.Int, xs ...core.Int) {}),
				},
			},
			{
				name:  "variadic: arg count == non-variadic-param-count",
				input: "gofunc(1)",
				globalVariables: map[string]core.Value{
					"gofunc": core.WrapGoFunction(func(ctx *core.Context, x core.Int, xs ...core.Int) core.Int {
						return x
					}),
				},
				result: core.Int(1),
			},
			{
				name:  "variadic: arg count == 1 + non-variadic-param-count",
				input: "gofunc(1, 2)",
				globalVariables: map[string]core.Value{
					"gofunc": core.WrapGoFunction(func(ctx *core.Context, x core.Int, xs ...core.Int) core.Int {
						return core.Int(x + xs[0])
					}),
				},
				result: core.Int(3),
			},
			{
				name:  "variadic: many arguments",
				input: "gofunc(1, 2, 3,   1, 2, 3,   1, 2, 3,   1, 2, 3,   1, 2, 3,   1, 2, 3,   1, 2, 3)",
				globalVariables: map[string]core.Value{
					"gofunc": core.WrapGoFunction(func(ctx *core.Context, xs ...core.Int) core.Int {

						sum := core.Int(0)
						for _, e := range xs {
							sum += e
						}

						return core.Int(core.Int(len(xs)) /*21*/ + sum /* 7 * (1 + 2 + 3) = 42 */)
					}),
				},
				result: core.Int(63),
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
				makeGlobals: func(t *testing.T) map[string]core.Value {
					return map[string]core.Value{
						"gofunc": core.WrapGoFunction(func(ctx *core.Context, obj *core.Object) {
							assert.Equal(t, map[string]core.Serializable{"a": core.Int(1)}, obj.EntryMap(nil))
						}),
					}
				},
				result: core.Nil,
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
				makeGlobals: func(t *testing.T) map[string]core.Value {
					return map[string]core.Value{
						"gofunc": core.WrapGoFunction(func(ctx *core.Context, sharedValue *core.InoxFunction) {
							assert.False(t, sharedValue.IsShared())

						}),
					}
				},
				result: core.Nil,
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
				makeGlobals: func(t *testing.T) map[string]core.Value {
					return map[string]core.Value{
						"gofunc": core.WrapGoFunction(func(ctx *core.Context, sharedValue *core.InoxFunction) {
							assert.False(t, sharedValue.IsShared())

						}),
					}
				},
				result: core.Nil,
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
			// 	makeGlobals: func(t *testing.T) map[string]core.Value {
			// 		return map[string]core.Value{
			// 			"gofunc": core.WrapGoFunction(func(ctx *core.Context, sharedValue *core.InoxFunction) {
			// 				assert.True(t, sharedValue.IsShared())
			// 			}),
			// 		}
			// 	},
			// 	result: core.Nil,
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
			// 	makeGlobals: func(t *testing.T) map[string]core.Value {
			// 		return map[string]core.Value{
			// 			"gofunc": WrapGoClosure(func(ctx *core.Context, sharedValue *core.InoxFunction) {
			// 				assert.True(t, sharedValue.IsShared())
			// 			}),
			// 		}
			// 	},
			// 	result: core.Nil,
			// },
			{
				name:  "(must) call with two results, error is nil",
				input: "return gofunc!()",
				globalVariables: map[string]core.Value{
					"gofunc": core.WrapGoFunction(func(ctx *core.Context) (core.Int, error) {
						return 3, nil
					}),
				},
				result: core.Int(3),
			}, {
				name:  "(must) call with two results, error is not nil",
				input: "return gofunc!()",
				globalVariables: map[string]core.Value{
					"gofunc": core.WrapGoFunction(func(ctx *core.Context) (core.Int, error) {
						return -1, errors.New("error !")
					}),
				},
				error: true,
			},
			{
				name:  "core.GoValue returned",
				input: "return getuser()",
				globalVariables: map[string]core.Value{
					"getuser": core.WrapGoFunction(func(ctx *core.Context) core.GoValue {
						return core.TestMutableGoValue{Name: "Foo"}
					}),
				},
				result: core.TestMutableGoValue{Name: "Foo"},
			},
			{
				name:  "[]string returned, should be converted to a list",
				input: "return getNames()",
				globalVariables: map[string]core.Value{
					"getNames": core.WrapGoFunction(func(ctx *core.Context) []core.String {
						return []core.String{"string"}
					}),
				},
				result: core.NewWrappedValueList(core.String("string")),
			},
			{
				name:  "method",
				input: "return user.getName()",
				globalVariables: map[string]core.Value{
					"user": core.TestMutableGoValue{"Foo", ""},
				},
				result: core.String("Foo"),
			},
			{
				name: "optional parameter: no arguments",
				input: `
					return gofunc()
				`,
				makeGlobals: func(t *testing.T) map[string]core.Value {
					goFunc := func(ctx *core.Context, i *core.OptionalParam[core.Int]) core.Int {
						if !assert.Nil(t, i) {
							//assertion failed
							return -1
						}
						return 1
					}

					symbolicRegistrationLock.Lock()
					defer symbolicRegistrationLock.Unlock()

					if !core.IsSymbolicEquivalentOfGoFunctionRegistered(goFunc) {
						core.RegisterSymbolicGoFunction(goFunc, func(*symbolic.Context, *symbolic.OptionalParam[*symbolic.Int]) *symbolic.Int {
							return symbolic.ANY_INT
						})
					}

					return map[string]core.Value{
						"gofunc": core.WrapGoFunction(goFunc),
					}
				},
				result: core.Int(1),
			},
			{
				name: "optional parameter: argument provided",
				input: `
					return gofunc(2)
				`,
				makeGlobals: func(t *testing.T) map[string]core.Value {
					goFunc := func(ctx *core.Context, i *core.OptionalParam[core.Int]) core.Int {
						if !assert.NotNil(t, i) {
							//assertion failed
							return -1
						}
						return i.Value
					}

					symbolicRegistrationLock.Lock()
					defer symbolicRegistrationLock.Unlock()

					if !core.IsSymbolicEquivalentOfGoFunctionRegistered(goFunc) {
						core.RegisterSymbolicGoFunction(goFunc, func(*symbolic.Context, *symbolic.OptionalParam[*symbolic.Int]) *symbolic.Int {
							return symbolic.ANY_INT
						})
					}

					return map[string]core.Value{
						"gofunc": core.WrapGoFunction(goFunc),
					}
				},
				result: core.Int(2),
			},
			{
				name: "two optional parameters: no arguments",
				input: `
					return gofunc()
				`,
				makeGlobals: func(t *testing.T) map[string]core.Value {
					goFunc := func(ctx *core.Context, a, b *core.OptionalParam[core.Int]) core.Int {
						if !assert.Nil(t, a) {
							//assertion failed
							return -1
						}
						if !assert.Nil(t, b) {
							//assertion failed
							return -1
						}
						return 1
					}

					symbolicRegistrationLock.Lock()
					defer symbolicRegistrationLock.Unlock()

					if !core.IsSymbolicEquivalentOfGoFunctionRegistered(goFunc) {
						core.RegisterSymbolicGoFunction(goFunc, func(_ *symbolic.Context, a, b *symbolic.OptionalParam[*symbolic.Int]) *symbolic.Int {
							return symbolic.ANY_INT
						})
					}

					return map[string]core.Value{
						"gofunc": core.WrapGoFunction(goFunc),
					}
				},
				result: core.Int(1),
			},
			{
				name: "two optional parameters: single argument",
				input: `
					return gofunc(2)
				`,
				makeGlobals: func(t *testing.T) map[string]core.Value {
					goFunc := func(ctx *core.Context, a, b *core.OptionalParam[core.Int]) core.Int {
						if !assert.NotNil(t, a) {
							//assertion failed
							return -1
						}
						if !assert.Nil(t, b) {
							//assertion failed
							return -1
						}
						return a.Value
					}

					symbolicRegistrationLock.Lock()
					defer symbolicRegistrationLock.Unlock()

					if !core.IsSymbolicEquivalentOfGoFunctionRegistered(goFunc) {
						core.RegisterSymbolicGoFunction(goFunc, func(_ *symbolic.Context, a, b *symbolic.OptionalParam[*symbolic.Int]) *symbolic.Int {
							return symbolic.ANY_INT
						})
					}

					return map[string]core.Value{
						"gofunc": core.WrapGoFunction(goFunc),
					}
				},
				result: core.Int(2),
			},
			{
				name: "two optional parameters: two arguments are provided",
				input: `
					return gofunc(2, 3)
				`,
				makeGlobals: func(t *testing.T) map[string]core.Value {
					goFunc := func(ctx *core.Context, a, b *core.OptionalParam[core.Int]) core.Int {
						if !assert.NotNil(t, a) {
							//assertion failed
							return -1
						}
						if !assert.NotNil(t, b) {
							//assertion failed
							return -1
						}
						return a.Value + b.Value
					}

					symbolicRegistrationLock.Lock()
					defer symbolicRegistrationLock.Unlock()

					if !core.IsSymbolicEquivalentOfGoFunctionRegistered(goFunc) {
						core.RegisterSymbolicGoFunction(goFunc, func(_ *symbolic.Context, a, b *symbolic.OptionalParam[*symbolic.Int]) *symbolic.Int {
							return symbolic.ANY_INT
						})
					}

					return map[string]core.Value{
						"gofunc": core.WrapGoFunction(goFunc),
					}
				},
				result: core.Int(5),
			},
			{
				name: "mandatory + optional parameter: single argument",
				input: `
					return gofunc(2)
				`,
				makeGlobals: func(t *testing.T) map[string]core.Value {
					goFunc := func(ctx *core.Context, a core.Int, b *core.OptionalParam[core.Int]) core.Int {
						if !assert.Nil(t, b) {
							return -1
						}
						return a
					}

					symbolicGoFunc := func(*symbolic.Context, *symbolic.Int, *symbolic.OptionalParam[*symbolic.Int]) *symbolic.Int {
						return symbolic.ANY_INT
					}

					symbolicRegistrationLock.Lock()
					defer symbolicRegistrationLock.Unlock()

					if !core.IsSymbolicEquivalentOfGoFunctionRegistered(goFunc) {
						core.RegisterSymbolicGoFunction(goFunc, symbolicGoFunc)
					}

					return map[string]core.Value{
						"gofunc": core.WrapGoFunction(goFunc),
					}
				},
				result: core.Int(2),
			},
			{
				name: "mandatory + optional parameter: all arguments are provided",
				input: `
					return gofunc(2, 3)
				`,
				makeGlobals: func(t *testing.T) map[string]core.Value {
					goFunc := func(ctx *core.Context, a core.Int, b *core.OptionalParam[core.Int]) core.Int {
						if !assert.NotNil(t, b) {
							return -1
						}
						return a + b.Value
					}

					symbolicGoFunc := func(*symbolic.Context, *symbolic.Int, *symbolic.OptionalParam[*symbolic.Int]) *symbolic.Int {
						return symbolic.ANY_INT
					}

					symbolicRegistrationLock.Lock()
					defer symbolicRegistrationLock.Unlock()

					if !core.IsSymbolicEquivalentOfGoFunctionRegistered(goFunc) {
						core.RegisterSymbolicGoFunction(goFunc, symbolicGoFunc)
					}

					return map[string]core.Value{
						"gofunc": core.WrapGoFunction(goFunc),
					}
				},
				result: core.Int(5),
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				globals := testCase.globalVariables
				if testCase.makeGlobals != nil {
					globals = testCase.makeGlobals(t)
				}

				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
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

	t.Run("mutation of shared object", func(t *testing.T) {

		t.Run("calling a mutating method of a shared object's property", func(t *testing.T) {
			t.SkipNow()
			code := `
				start_tx()
				obj = {
					list: []
				}

				# share the object
				go {globals: {obj: obj}} do {

				}

				obj::list.append(1)
				commit_tx()
				return obj
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Globals.Set("start_tx", core.ValOf(core.StartNewTransaction))
			state.Globals.Set("commit_tx", core.ValOf(func(ctx *core.Context) {
				ctx.GetTx().Commit(ctx)
			}))

			res, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			expectedList := core.NewWrappedValueList(core.Int(1))
			list := res.(*core.Object).Prop(state.Ctx, "list").(*core.List)

			assert.Equal(t, expectedList, list)
		})

		t.Run("calling a mutating method of a shared object's property should be thread safe", func(t *testing.T) {
			t.SkipNow()
			code := `
				start_tx()
				obj = {
					list: []
				}
				group = LThreadGroup()
	
				for 1..5 {
					go {globals: {obj: obj, start_tx: start_tx, commit_tx: commit_tx}, group: group} do {
						start_tx()
						obj::list.append(1)
						commit_tx()
					}
				}
	
				group.wait_results!()
				return obj
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Globals.Set("start_tx", core.ValOf(core.StartNewTransaction))
			state.Globals.Set("commit_tx", core.ValOf(func(ctx *core.Context) {
				ctx.GetTx().Commit(ctx)
			}))
			state.Globals.Set("LThreadGroup", core.ValOf(core.NewLThreadGroup))

			res, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			var elements []core.Serializable
			for i := 0; i < 5; i++ {
				elements = append(elements, core.Int(1))
			}

			expectedList := core.NewWrappedValueList(elements...)
			list := res.(*core.Object).Prop(state.Ctx, "list").(*core.List)

			assert.Equal(t, expectedList, list)
		})

		t.Run("calling a mutating method of a shared object's property while getting the property in another goroutine should be thread safe", func(t *testing.T) {
			code := `
				start_tx()
				obj = {
					list: []
				}
				group = LThreadGroup()
	
				for 1..2 {
					go {globals: {obj: obj, start_tx: start_tx, commit_tx: commit_tx}, group: group} do {
						start_tx()
						obj::list.append(1)
						commit_tx()
					}
					go {globals: {obj: obj, start_tx: start_tx, commit_tx: commit_tx}, group: group} do {
						start_tx()
						list = obj.list
						commit_tx()
					}
				}
	
				group.wait_results!()
				return obj
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Globals.Set("LThreadGroup", core.ValOf(core.NewLThreadGroup))
			state.Globals.Set("start_tx", core.ValOf(func(ctx *core.Context) *core.Transaction {
				//fmt.Printf("start tx, context %p\n", ctx)
				return core.StartNewTransaction(ctx)
			}))
			state.Globals.Set("commit_tx", core.ValOf(func(ctx *core.Context) {
				//fmt.Printf("commited, context %p\n", ctx)
				ctx.GetTx().Commit(ctx)
			}))

			res, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			var elements []core.Serializable
			for i := 0; i < 5; i++ {
				elements = append(elements, core.Int(1))
			}

			_ = res
			//assert.Equal(t, core.NewWrappedValueList(elements...), res.(*core.Object).values[0])
		})
	})

	allExtendStmtTestsPassed := t.Run("extend statement", func(t *testing.T) {
		t.Run("computed property", func(t *testing.T) {
			code := `
				pattern p = {
					a: 1
				}

				extend p {
					b: 2
				}
			`

			state := core.NewGlobalState(NewDefaultTestContext())

			defer state.Ctx.CancelGracefully()

			_, err := Eval(code, state, true)
			if !assert.NoError(t, err) {
				return
			}

			typeExtensions := state.Ctx.GetAllTypeExtensions()
			if !assert.Len(t, typeExtensions, 1) {
				return
			}

			extension := typeExtensions[0]
			extendStmt, ancestors := parse.FindNodeAndChain(state.Module.MainChunk.Node, (*parse.ExtendStatement)(nil), nil)

			ctxData, ok := state.SymbolicData.GetContextData(extendStmt, ancestors)
			if !assert.True(t, ok) {
				return
			}

			symbolicExt := ctxData.Extensions[0]
			assert.Equal(t, symbolicExt, extension.Symbolic())
		})

		t.Run("method", func(t *testing.T) {
			code := `
				pattern p = {
					a: 1
				}

				extend p {
					f: fn() => self.a
				}
			`

			state := core.NewGlobalState(NewDefaultTestContext())

			defer state.Ctx.CancelGracefully()

			_, err := Eval(code, state, true)
			if !assert.NoError(t, err) {
				return
			}

			typeExtensions := state.Ctx.GetAllTypeExtensions()
			if !assert.Len(t, typeExtensions, 1) {
				return
			}

			extension := typeExtensions[0]
			extendStmt, ancestors := parse.FindNodeAndChain(state.Module.MainChunk.Node, (*parse.ExtendStatement)(nil), nil)

			ctxData, ok := state.SymbolicData.GetContextData(extendStmt, ancestors)
			if !assert.True(t, ok) {
				return
			}

			symbolicExt := ctxData.Extensions[0]
			assert.Equal(t, symbolicExt, extension.Symbolic())
		})
	})

	if allExtendStmtTestsPassed {
		t.Run("extension properties & methods", func(t *testing.T) {
			t.Run("computed property", func(t *testing.T) {
				code := `
					pattern p = {
						a: 1
					}
	
					extend p {
						b: 2
					}
	
					var object p = {
						a: 1
					}
	
					return object::b
				`

				state := core.NewGlobalState(NewDefaultTestContext())

				defer state.Ctx.CancelGracefully()

				res, err := Eval(code, state, true)
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, core.Int(2), res)
			})

			t.Run("method call", func(t *testing.T) {
				code := `
					pattern p = {
						a: 1
					}
	
					extend p {
						f: fn() => (1 + self.a)
					}
	
					var object p = {
						a: 1
					}
	
					return object::f()
				`

				state := core.NewGlobalState(NewDefaultTestContext())

				defer state.Ctx.CancelGracefully()

				res, err := Eval(code, state, true)
				if !assert.NoError(t, err) {
					return
				}
				assert.Equal(t, core.Int(2), res)
			})
		})
	}

	t.Run("pattern call", func(t *testing.T) {
		code := `%mypattern(1..10)`

		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()
		state.Ctx.AddNamedPattern("mypattern", core.NewRegexPattern(".*"))

		res, err := Eval(code, state, false)
		assert.NoError(t, err)

		expectedPattern, _ := core.NewRegexPattern(".*").Call(state.Ctx, []core.Serializable{core.NewIntRange(1, 10)})

		assert.Equal(t, expectedPattern, res)
	})

	t.Run("function pattern definition,", func(t *testing.T) {
		code := `
			pattern intfn = %fn() %int
			return %intfn
		`

		ctx := NewDefaultTestContext()
		defer ctx.CancelGracefully()

		ctx.AddNamedPattern("int", core.DEFAULT_NAMED_PATTERNS["int"])
		state := core.NewGlobalState(ctx)
		res, err := Eval(code, state, true)
		assert.NoError(t, err)

		assert.IsType(t, &core.FunctionPattern{}, res)
	})

	t.Run("function pattern matching,", func(t *testing.T) {
		code := `
			pattern intfn = %fn() %int
			fn f() %int { 
				return 1
			}
			return (f match %intfn)
		`

		ctx := NewDefaultTestContext()
		defer ctx.CancelGracefully()
		ctx.AddNamedPattern("int", core.DEFAULT_NAMED_PATTERNS["int"])
		state := core.NewGlobalState(ctx)

		res, err := Eval(code, state, true)
		assert.NoError(t, err)

		assert.Equal(t, core.True, res)
	})

	t.Run("pipeline statement", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("pipeline statement", func(t *testing.T) {
			code := `get-data | split-lines $`
			var dollarVarValue core.String
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"get-data": core.ValOf(func(ctx *core.Context) core.String {
					return "aaa\nbbb"
				}),
				"split-lines": core.ValOf(func(ctx *core.Context, s core.String) []core.String {
					dollarVarValue = s
					return splitLines(ctx, s)
				}),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)

			if bytecodeEval {
				assert.Equal(t, core.Nil, res)
			} else {
				assert.Equal(t, core.NewWrappedValueList(core.String("aaa"), core.String("bbb")), res)
			}

			assert.Equal(t, core.String("aaa\nbbb"), dollarVarValue)
		})

		t.Run("original value of anonymous variable is restored", func(t *testing.T) {
			code := `
				$ = 1
				get-data | split-lines $;
				return $
			`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"get-data": core.ValOf(func(ctx *core.Context) core.String {
					return "aaa\nbbb"
				}),
				"split-lines": core.ValOf(splitLines),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(1), res)
		})

	})

	t.Run("pipeline expression", func(t *testing.T) {
		code := `
			result = | idt [1, "a", 2] | filter $ %int
			return result
		`
		state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
			"idt": core.ValOf(func(ctx *core.Context, v core.Value) core.Value {
				return v
			}),
			"filter": core.ValOf(core.Filter),
		})
		state.Ctx.AddNamedPattern("int", core.INT_PATTERN)

		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.NewWrappedValueList(core.Int(1), core.Int(2)), res)
	})

	t.Run("member expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		testCases := []struct {
			error           bool
			input           string
			globalVariables map[string]core.Value
			result          core.Value
			pre             func(expected core.Value, actual core.Value, state *core.GlobalState)
		}{
			{
				input:  "$a = {v: 1}; return $a.v",
				result: core.Int(1),
			},
			{
				input:  "return ({a: 1}).a",
				result: core.Int(1),
			},
			{
				input: "return $goval.secret",
				error: true,
				globalVariables: map[string]core.Value{
					"goval": core.ValOf(core.TestMutableGoValue{Name: "Foo", Secret: "secret"}),
				},
				result: core.Nil,
			},
			{
				input:  "return ({}).?a",
				result: core.Nil,
			},
			{
				input:  "$a = {v: 1}; return $a.(\"v\")",
				result: core.Int(1),
			},
			{
				input: `
					rt = go do {
						return {x: 1}
					}
		
					res = rt.wait_result!()
					return res.x
				`,
				result: core.Int(1),
			},
			{
				input: `
					rt = go do {
						return {x: {}}
					}
		
					res = rt.wait_result!()
					return res.x
				`,
				result: core.NewObject(),
				pre: func(expected, actual core.Value, state *core.GlobalState) {
					expected = core.Share(expected.(core.PotentiallySharable), state)
				},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.input, func(t *testing.T) {
				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
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
		// testconfig.AllowParallelization(t)

		// testCases := []struct {
		// 	error       bool
		// 	input       string
		// 	globals     map[string]core.Value
		// 	result      core.Value
		// 	makeResult  func(state *core.GlobalState) core.Value
		// 	checkResult func(t *testing.T, state *core.GlobalState, actual core.Value)
		// }{
		// 	{
		// 		input: "$a = {v: 1}; return $a.<v",
		// 		checkResult: func(t *testing.T, state *core.GlobalState, actual core.Value) {
		// 			if !assert.IsType(t, &core.DynamicValue{}, actual) {
		// 				return
		// 			}

		// 			dyn := actual.(*core.DynamicValue)

		// 			assert.Equal(t, map[string]core.Serializable{"v": core.Int(1)}, dyn.value.(*core.Object).EntryMap(state.Ctx))
		// 			assert.Equal(t, core.String("v"), dyn.opData0)
		// 		},
		// 	},
		// 	{
		// 		input: "$a = {obj: {a: 1}}; return $a.<obj.a",
		// 		checkResult: func(t *testing.T, state *core.GlobalState, actual core.Value) {
		// 			if !assert.IsType(t, &core.DynamicValue{}, actual) {
		// 				return
		// 			}

		// 			dyn := actual.(*core.DynamicValue)

		// 			assert.Equal(t, map[string]core.Serializable{"a": core.Int(1)}, dyn.value.(*core.Object).EntryMap(state.Ctx))
		// 			assert.Equal(t, core.String("a"), dyn.opData0)
		// 		},
		// 	},
		// 	{
		// 		input: "return ({a: 1}).<a",
		// 		checkResult: func(t *testing.T, state *core.GlobalState, actual core.Value) {
		// 			if !assert.IsType(t, &core.DynamicValue{}, actual) {
		// 				return
		// 			}

		// 			dyn := actual.(*core.DynamicValue)

		// 			assert.Equal(t, map[string]core.Serializable{"a": core.Int(1)}, dyn.value.(*core.Object).EntryMap(state.Ctx))
		// 			assert.Equal(t, core.String("a"), dyn.opData0)
		// 		},
		// 	},
		// 	{
		// 		input: "return $goval.<secret",
		// 		error: true,
		// 		globals: map[string]core.Value{
		// 			"goval": core.ValOf(core.TestMutableGoValue{Name: "Foo", Secret: "secret"}),
		// 		},
		// 		result: core.Nil,
		// 	},
		// 	{
		// 		input: `
		// 			rt = go do {
		// 				return {x: 1}
		// 			}

		// 			res = rt.wait_result!()
		// 			return res.<x
		// 		`,
		// 		checkResult: func(t *testing.T, state *core.GlobalState, actual core.Value) {
		// 			if !assert.IsType(t, &core.DynamicValue{}, actual) {
		// 				return
		// 			}

		// 			dyn := actual.(*core.DynamicValue)

		// 			assert.Equal(t, map[string]core.Serializable{"x": core.Int(1)}, dyn.value.(*core.Object).EntryMap(state.Ctx))
		// 			assert.Equal(t, core.String("x"), dyn.opData0)
		// 		},
		// 	},
		// 	{
		// 		input: `
		// 			rt = go do {
		// 				return {x: {}}
		// 			}

		// 			res = rt.wait_result!()
		// 			return res.<x
		// 		`,
		// 		checkResult: func(t *testing.T, state *core.GlobalState, actual core.Value) {
		// 			if !assert.IsType(t, &core.DynamicValue{}, actual) {
		// 				return
		// 			}

		// 			dyn := actual.(*core.DynamicValue)

		// 			innerObj := core.NewObjectFromMap(nil, state.Ctx)
		// 			innerObj.Share(state)

		// 			assert.Equal(t, map[string]core.Serializable{"x": innerObj}, dyn.value.(*core.Object).EntryMap(state.Ctx))
		// 			assert.Equal(t, core.String("x"), dyn.opData0)
		// 		},
		// 	},
		// }

		// for _, testCase := range testCases {
		// 	t.Run(testCase.input, func(t *testing.T) {
		// 		state := core.NewGlobalState(NewDefaultTestContext())
		// 		defer state.Ctx.CancelGracefully()
		// 		for k, v := range testCase.globals {
		// 			state.Globals.Set(k, v)
		// 		}

		// 		res, err := Eval(testCase.input, state, false)
		// 		if testCase.error {
		// 			assert.Error(t, err)
		// 			assert.Nil(t, res)
		// 		} else {
		// 			assert.NoError(t, err)
		// 			expected := testCase.result
		// 			if testCase.checkResult != nil {
		// 				testCase.checkResult(t, state, res)
		// 			} else {
		// 				if testCase.makeResult != nil {
		// 					expected = testCase.makeResult(state)
		// 				}
		// 				assert.Equal(t, expected, res)
		// 			}
		// 		}
		// 	})
		// }
	})

	t.Run("extraction expression", func(t *testing.T) {
		code := `return ({a:1}).{a}`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()
		res, err := Eval(code, state, false)
		assert.NoError(t, err)
		assert.Equal(t, core.NewObjectFromMapNoInit(core.ValMap{"a": core.Int(1)}), res)
	})

	t.Run("key list expression", func(t *testing.T) {
		t.Run("empty", func(t *testing.T) {
			code := `return .{}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.KeyList{}, res)
		})

		t.Run("single element", func(t *testing.T) {
			code := `return .{name}`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.KeyList{"name"}, res)
		})

	})

	t.Run("lazy expression : @ <integer>", func(t *testing.T) {
		code := `@(1)`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()
		res, err := Eval(code, state, false)
		assert.NoError(t, err)

		assert.EqualValues(t, core.AstNode{
			Node: &parse.IntLiteral{
				NodeBase: parse.NodeBase{
					Span:            parse.NodeSpan{Start: 2, End: 3},
					IsParenthesized: true,
					// Tokens: []parse.Token{
					// 	{Type: parse.OPENING_PARENTHESIS, Span: parse.NodeSpan{Start: 1, End: 2}},
					// 	{Type: parse.CLOSING_PARENTHESIS, Span: parse.NodeSpan{Start: 3, End: 4}},
					// },
				},
				Raw:   "1",
				Value: 1,
			},
			Chunk_: state.Module.MainChunk,
		}, res)
	})

	t.Run("inclusion import statement", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("single included file with no dependecies", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return a
			`, map[string]string{"./dep.ix": "includable-file \n a = 1"})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Module = mod
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(1), res)
		})

		t.Run("single included file which itself includes a file", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep2.ix
				return a
			`, map[string]string{
				"./dep2.ix": `
					includable-file
					import ./dep1.ix
				`,
				"./dep1.ix": `
					includable-file
					a = 1
				`,
			})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Module = mod
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(1), res)
		})

		t.Run("two included files with no dependecies", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep1.ix
				import ./dep2.ix
				return (a + b)
			`, map[string]string{
				"./dep1.ix": "includable-file\n const (a = 1)",
				"./dep2.ix": "includable-file\n const (b = 2)",
			})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Module = mod
			res, err := Eval(mod, state, true)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(3), res)
		})

		t.Run("single included file accessing a global", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return a
			`, map[string]string{"./dep.ix": "includable-file\n a = myglobal"})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"myglobal": core.Int(1),
			})
			state.Module = mod
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(1), res)
		})

		t.Run("single included file accessing a global in a function", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return f()
			`, map[string]string{"./dep.ix": `
				includable-file
				fn f(){
					return myglobal
				}
			`})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"myglobal": core.Int(1),
			})
			state.Module = mod
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(1), res)
		})

		t.Run("single included file accessing a global in a lifetime job", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				sleep 10ms
				return obj.a
			`, map[string]string{"./dep.ix": `
				includable-file
				obj = {
					a: 0
					lifetimejob #job {
						self.a = myglobal
					}
				}
			`})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"myglobal": core.Int(1),
				"sleep":    core.WrapGoFunction(core.Sleep),
			})
			state.Module = mod
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(1), res)
		})

		t.Run("included file defining a pattern", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return %p
			`, map[string]string{"./dep.ix": `
				includable-file
				pattern p = str
			`})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Ctx.AddNamedPattern("str", core.STR_PATTERN)
			state.Module = mod
			res, err := Eval(mod, state, false)
			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, core.STR_PATTERN, res)
		})
	})

	t.Run("import statement", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		getModule := func(code string) (*core.Module, error) {
			fls := newMemFilesystem()
			err := util.WriteFile(fls, "/mod.ix", []byte(code), 0600)
			if err != nil {
				return nil, err
			}

			ctx := core.NewContextWithEmptyState(core.ContextConfig{
				Permissions: []core.Permission{
					core.CreateFsReadPerm(core.PathPattern("/...")),
					core.CreateHttpReadPerm(core.ANY_HTTPS_HOST_PATTERN),
				},
				Filesystem: fls,
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := core.ParseLocalModule("/mod.ix", core.ModuleParsingConfig{
				Context: ctx,
			})

			return mod, err
		}

		t.Run("no globals, no required permissions", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			mod, err := getModule(strings.ReplaceAll(`
				manifest {}
				import importname https://modules.com/return_1.ix {
					validation: "<hash>"
				}
				return $importname
			`, "<hash>", RETURN_1_MODULE_HASH))

			if !assert.NoError(t, err) {
				return
			}

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.Int(1), res)
		})

		t.Run("imported module returns the positional 'a' argument", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			mod, err := getModule(strings.ReplaceAll(`
				manifest {}
				import importname https://modules.com/return_global_a.ix {
					validation: "<hash>"
					arguments: {1}
				}
				return $importname
			`, "<hash>", RETURN_POS_ARG_A_MODULE_HASH))

			if !assert.NoError(t, err) {
				return
			}

			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()
			ctx.AddNamedPattern("int", core.INT_PATTERN)

			state := core.NewGlobalState(ctx)
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.Int(1), res)
		})

		t.Run("imported module returns the non-positional 'a' argument", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			mod, err := getModule(strings.ReplaceAll(`
				manifest {}
				import importname https://modules.com/return_global_a.ix {
					validation: "<hash>"
					arguments: {a: 1}
				}
				return $importname
			`, "<hash>", RETURN_NON_POS_ARG_A_MODULE_HASH))

			if !assert.NoError(t, err) {
				return
			}

			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()
			ctx.AddNamedPattern("int", core.INT_PATTERN)

			state := core.NewGlobalState(ctx)
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.Int(1), res)
		})

		t.Run("imported module returns the %two pattern (same pattern is defined in module)", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			mod, err := getModule(strings.ReplaceAll(`
				manifest {}
				pattern two = 1

				import two_patt https://modules.com/return_patt_two.ix {
					validation: "<hash>"
					arguments: {}
				}
				return $two_patt
			`, "<hash>", RETURN_PATTERN_INT_TWO_MODULE_HASH))

			if !assert.NoError(t, err) {
				return
			}

			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()
			ctx.AddNamedPattern("int", core.INT_PATTERN)

			state := core.NewGlobalState(ctx)
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewExactValuePattern(core.Int(2)), res)
		})

		t.Run("imported module returns the %two pattern", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			mod, err := getModule(strings.ReplaceAll(`
				manifest {}
				import two_patt https://modules.com/return_patt_two.ix {
					validation: "<hash>"
					arguments: {}
				}
				return $two_patt
			`, "<hash>", RETURN_PATTERN_INT_TWO_MODULE_HASH))

			if !assert.NoError(t, err) {
				return
			}

			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()
			ctx.AddNamedPattern("int", core.INT_PATTERN)

			state := core.NewGlobalState(ctx)
			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewExactValuePattern(core.Int(2)), res)
		})

		t.Run("imported module returns the %int pattern (base pattern)", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			mod, err := getModule(strings.ReplaceAll(`
				manifest {}
				import int_pattern https://modules.com/return_patt_int.ix {
					validation: "<hash>"
					arguments: {}
				}
				return int_pattern
			`, "<hash>", RETURN_INT_PATTERN_MODULE_HASH))

			if !assert.NoError(t, err) {
				return
			}

			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()
			ctx.AddNamedPattern("int", core.INT_PATTERN)

			state := core.NewGlobalState(ctx)
			//we copy the pattern in order to later check that the importer's pattern is not passed to the imported module.
			intPatternCopy := *core.INT_PATTERN

			state.GetBasePatternsForImportedModule = func() (map[string]core.Pattern, map[string]*core.PatternNamespace) {
				return map[string]core.Pattern{"int": &intPatternCopy}, nil
			}

			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.Same(t, &intPatternCopy, res)
		})

		t.Run("local module that includes a file", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			mod, err := getModule(strings.ReplaceAll(`
				manifest {}
				import importname ./return_a.ix  {
					validation: "<hash>"
					arguments: {a: 1}
				}
				return $importname
			`, "<hash>", RETURN_NON_POS_ARG_A_MODULE_HASH))

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(
					core.GetDefaultGlobalVarPermissions(),
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
					core.LThreadPermission{permkind.Create},
				),
				Filesystem: newOsFilesystem(),
				Limits:     []core.Limit{permissiveLthreadLimit},
			})
			ctx.AddNamedPattern("int", core.INT_PATTERN)
			defer ctx.CancelGracefully()

			state := core.NewGlobalState(ctx)
			state.Module = mod
			state.GetBasePatternsForImportedModule = func() (map[string]core.Pattern, map[string]*core.PatternNamespace) {
				return core.DEFAULT_NAMED_PATTERNS, core.DEFAULT_PATTERN_NAMESPACES
			}

			res, err := Eval(mod, state, false)
			assert.NoError(t, err)
			assert.EqualValues(t, core.Int(1), res)
		})

		t.Run("logs from an imported module should have the correct source and respect the default log level", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			fls := newMemFilesystem()
			err := util.WriteFile(fls, "/mod.ix", []byte(`
				manifest {}
				import mod /imported_mod.ix {}
			`), 0600)

			if !assert.NoError(t, err) {
				return
			}

			err = util.WriteFile(fls, "/imported_mod.ix", []byte(`
				manifest {}
				log("hello")
			`), 0600)

			if !assert.NoError(t, err) {
				return
			}

			parsingCtx := core.NewContextWithEmptyState(core.ContextConfig{
				Permissions: []core.Permission{
					core.CreateFsReadPerm(core.PathPattern("/...")),
					core.CreateHttpReadPerm(core.ANY_HTTPS_HOST_PATTERN),
				},
				Filesystem: fls,
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := core.ParseLocalModule("/mod.ix", core.ModuleParsingConfig{
				Context: parsingCtx,
			})

			if !assert.NoError(t, err) {
				return
			}

			logBuf := bytes.NewBuffer(nil)

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(
					core.GetDefaultGlobalVarPermissions(),
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
					core.LThreadPermission{permkind.Create},
				),
				Filesystem: newOsFilesystem(),
				Limits:     []core.Limit{permissiveLthreadLimit},
			})
			defer ctx.CancelGracefully()

			state := core.NewGlobalState(ctx)
			state.Out = io.Discard
			state.Logger = zerolog.New(logBuf)
			state.LogLevels = core.NewLogLevels(core.LogLevelsInitialization{
				DefaultLevel: zerolog.DebugLevel,
			})
			state.OutputFieldsInitialized.Store(true)

			state.Globals.Set("log", core.WrapGoFunction(func(ctx *core.Context, msg core.String) {
				ctx.DebugLogEvent().Msg(string(msg))
			}))

			state.Module = mod
			state.GetBaseGlobalsForImportedModule = func(ctx *core.Context, manifest *core.Manifest) (core.GlobalVariables, error) {
				return state.Globals, nil
			}
			state.GetBasePatternsForImportedModule = func() (map[string]core.Pattern, map[string]*core.PatternNamespace) {
				return nil, nil
			}

			_, err = Eval(mod, state, false)
			assert.NoError(t, err)
			assert.Contains(t, logBuf.String(), `"src":"/imported_mod.ix","msg":"hello"`)
		})

		t.Run("logs from an imported module should respect the log level configured for its path", func(t *testing.T) {
			testconfig.AllowParallelization(t)

			fls := newMemFilesystem()
			err := util.WriteFile(fls, "/mod.ix", []byte(`
				manifest {}
				import mod /imported_mod.ix {}
			`), 0600)

			if !assert.NoError(t, err) {
				return
			}

			err = util.WriteFile(fls, "/imported_mod.ix", []byte(`
				manifest {}
				log_debug("debug")
				log_info("info")
			`), 0600)

			if !assert.NoError(t, err) {
				return
			}

			parsingCtx := core.NewContextWithEmptyState(core.ContextConfig{
				Permissions: []core.Permission{
					core.CreateFsReadPerm(core.PathPattern("/...")),
					core.CreateHttpReadPerm(core.ANY_HTTPS_HOST_PATTERN),
				},
				Filesystem: fls,
			}, nil)
			defer parsingCtx.CancelGracefully()

			mod, err := core.ParseLocalModule("/mod.ix", core.ModuleParsingConfig{
				Context: parsingCtx,
			})

			if !assert.NoError(t, err) {
				return
			}

			logBuf := bytes.NewBuffer(nil)

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(
					core.GetDefaultGlobalVarPermissions(),
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
					core.LThreadPermission{permkind.Create},
				),
				Filesystem: newOsFilesystem(),
				Limits:     []core.Limit{permissiveLthreadLimit},
			})
			defer ctx.CancelGracefully()

			state := core.NewGlobalState(ctx)
			state.Out = io.Discard
			state.Logger = zerolog.New(logBuf)
			state.LogLevels = core.NewLogLevels(core.LogLevelsInitialization{
				DefaultLevel: zerolog.DebugLevel,
				ByPath: map[core.Path]zerolog.Level{
					"/imported_mod.ix": zerolog.InfoLevel,
				},
			})
			state.OutputFieldsInitialized.Store(true)

			state.Globals.Set("log_debug", core.WrapGoFunction(func(ctx *core.Context, msg core.String) {
				ctx.DebugLogEvent().Msg(string(msg))
			}))
			state.Globals.Set("log_info", core.WrapGoFunction(func(ctx *core.Context, msg core.String) {
				ctx.InfoLogEvent().Msg(string(msg))
			}))

			state.Module = mod
			state.GetBaseGlobalsForImportedModule = func(ctx *core.Context, manifest *core.Manifest) (core.GlobalVariables, error) {
				return state.Globals, nil
			}
			state.GetBasePatternsForImportedModule = func() (map[string]core.Pattern, map[string]*core.PatternNamespace) {
				return nil, nil
			}

			_, err = Eval(mod, state, false)
			assert.NoError(t, err)
			assert.Contains(t, logBuf.String(), `"src":"/imported_mod.ix","msg":"info"`)
			assert.NotContains(t, logBuf.String(), `"src":"/imported_mod.ix","msg":"debug"`)
		})

	})

	t.Run("spawn expression", func(t *testing.T) {
		t.Run("call expression: user declared function", func(t *testing.T) {
			code := `
				fn f(){
					return 1
				}
				lthread = go do f()
				return lthread.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Logger = zerolog.New(state.Out)
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(1), res)
		})

		t.Run("call expression: namespace's function", func(t *testing.T) {
			code := `
				lthread = go do http.read(https://example.com/)
				return lthread.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"http": core.NewNamespace("http", map[string]core.Value{
					"read": core.WrapGoFunction(func(*core.Context, core.URL) core.String {
						return core.String("result")
					}),
				}),
			})
			defer state.Ctx.CancelGracefully()
			state.Logger = zerolog.New(state.Out)

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.String("result"), res)
		})

		t.Run("call expression: embedded module should inherit global start constants", func(t *testing.T) {
			code := `
				fn f(arg){
					return arg
				}
				lthread = go do f(myconst)
				return lthread.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"myconst": core.Int(1),
			})
			defer state.Ctx.CancelGracefully()
			state.Logger = zerolog.New(state.Out)

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(1), res)
		})

		t.Run("call expression: embedded module should not inherit explicitly defined global constants", func(t *testing.T) {
			code := `
				const (
					myconst = 1
				)
				fn f(arg){
					return arg
				}

				lthread = go do f(myconst)
				return lthread.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Logger = zerolog.New(state.Out)

			_, err := Eval(code, state, false)
			assert.Error(t, err)
		})

		t.Run("call expression: embedded module should not inherit global variables", func(t *testing.T) {
			code := `
				fn f(arg){
					return arg
				}

				globalvar myglobal = 1
				lthread = go do f(myglobal)
				return lthread.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Logger = zerolog.New(state.Out)

			_, err := Eval(code, state, false)
			assert.Error(t, err)
		})

		t.Run("empty embedded module", func(t *testing.T) {
			code := `
				go do { }
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
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
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(1), res)
		})

		t.Run("embedded module returns a data structure", func(t *testing.T) {
			code := `
				rt = go do { 
					return { }
				}
	
				return rt.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.IsType(t, &core.Object{}, res)
			assert.True(t, res.(*core.Object).IsShared())
		})

		t.Run("allow <runtime manifest>", func(t *testing.T) {
			code := `
				globalvar URL = https://example.com/
				rt = go {allow: {read: URL}} do { 
	
				}
	
				rt.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			_, err := Eval(code, state, false)
			assert.NoError(t, err)
		})

		t.Run("pass an additional global to a single expression embedded module", func(t *testing.T) {
			code := `
				rt = go {globals: {b: 2}} do idt(b)
				return rt.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": core.WrapGoFunction(func(ctx *core.Context, arg core.Value) core.Value {
					return arg
				}),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(2), res)
		})

		t.Run("pass an additional global to an embedded module (block)", func(t *testing.T) {
			code := `
				rt = go {globals: {b: 2}} do { 
					return b
				}
	
				return rt.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(2), res)
		})

		t.Run("group: used once", func(t *testing.T) {
			code := `
				group = LThreadGroup()
				go {group: group} do { }
	
				return group
			`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"LThreadGroup": core.WrapGoFunction(core.NewLThreadGroup),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.IsType(t, &core.LThreadGroup{}, res)
			assert.Len(t, res.(*core.LThreadGroup).CurrentThreads(), 1)
		})

		t.Run("group: used twice", func(t *testing.T) {
			code := `
				group = LThreadGroup()
				go {group: group} do { }
				go {group: group} do { }
	
				return group
			`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"LThreadGroup": core.WrapGoFunction(core.NewLThreadGroup),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.IsType(t, &core.LThreadGroup{}, res.(core.GoValue))

			assert.Len(t, res.(*core.LThreadGroup).CurrentThreads(), 2)
		})

		t.Run("call a passed Inox function", func(t *testing.T) {
			code := `
				fn f(){
					return 2
				}
				rt = go {globals: {f: f}} do {
					return f() # f is external for the embedded module
				}
	
				return rt.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(2), res)
		})

		t.Run("call a passed Inox function that access a captured global", func(t *testing.T) {
			code := `
				globalvar a = 1
				fn f(){
					return a
				}
				rt = go {globals: {f: f}} do {
					globalvar a = 2

					return f()
				}
	
				return rt.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, true)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(1), res)
		})

		t.Run("call a function accessing a global variable within a passed Inox function that captured this global", func(t *testing.T) {
			code := `
				globalvar a = 1
				fn f(){
					b = a
					func = fn(){
						return a
					}
					return func()
				}
				rt = go {globals: {f: f}} do {
					globalvar a = 2

					return f()
				}
	
				return rt.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, true)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(1), res)
		})

		t.Run("compute a Mapping entry that access a captured global", func(t *testing.T) {
			code := `
				globalvar a = 1
				mapping = Mapping {
					%/... => a
				}
				rt = go {globals: {m: mapping}} do {
					globalvar a = 2

					return m.compute(/)
				}
	
				return rt.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, true)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(1), res)
		})

		//TODO: add more tests on global capture

		t.Run("call passed Go func", func(t *testing.T) {
			called := false
			code := `
				group = LThreadGroup()
				rt = go {group: group} do gofunc()
	
				return rt.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"gofunc": core.ValOf(func(ctx *core.Context) core.Int {
					called = true
					return 2
				}),
				"LThreadGroup": core.WrapGoFunction(core.NewLThreadGroup),
			})
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.True(t, called)
			assert.Equal(t, core.Int(2), res)
		})

		t.Run("spawner & lthread access a shared value in a synchronized block", func(t *testing.T) {
			{
				runtime.GC()
				startMemStats := new(runtime.MemStats)
				runtime.ReadMemStats(startMemStats)

				defer utils.AssertNoMemoryLeak(t, startMemStats, 100_000, utils.AssertNoMemoryLeakOptions{
					PreSleepDurationMillis: 100,
				})
			}

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

			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			state := core.NewGlobalState(ctx, map[string]core.Value{
				"sleep": core.ValOf(func(ctx *core.Context) {
					time.Sleep(time.Millisecond)
				}),
			})
			state.Logger = zerolog.New(state.Out)
			state.Out = os.Stdout

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(2*goroutineIncCount), res)
		})

		t.Run("spawner & lthread access a shared value without synchronization", func(t *testing.T) {
			{
				runtime.GC()
				startMemStats := new(runtime.MemStats)
				runtime.ReadMemStats(startMemStats)

				defer utils.AssertNoMemoryLeak(t, startMemStats, 100_000, utils.AssertNoMemoryLeakOptions{
					PreSleepDurationMillis: 100,
				})
			}

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

			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			state := core.NewGlobalState(ctx, map[string]core.Value{
				"sleep": core.ValOf(func(ctx *core.Context) {
					time.Sleep(time.Millisecond)
				}),
			})
			state.Logger = zerolog.New(state.Out)
			state.Out = os.Stdout

			timedOut := atomic.Bool{}

			// timeout
			go func() {
				<-time.After(10 * time.Second)
				state.Ctx.CancelGracefully()
				timedOut.Store(true)
			}()

			_, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.False(t, timedOut.Load())
		})

		t.Run("embedded module yields once and has no return statement", func(t *testing.T) {
			code := `
				rt = go do { 
					coyield 0
				}
	
				result = rt.wait_result!()

				step_results = []
				for step in rt.steps {
					step_results.append(step.result)
				}
				return [result, step_results]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.NewWrappedValueList(
				core.Nil,
				core.NewWrappedValueList(core.Int(0)),
			), res)
		})

		t.Run("embedded module yields twice and has no return statement", func(t *testing.T) {
			code := `
				rt = go do { 
					coyield 0

					coyield 1
				}
	
				result = rt.wait_result!()

				step_results = []
				for step in rt.steps {
					step_results.append(step.result)
				}
				return [result, step_results]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.NewWrappedValueList(
				core.Nil,
				core.NewWrappedValueList(core.Int(0), core.Int(1)),
			), res)
		})

		t.Run("embedded module yields three times and has no return statement", func(t *testing.T) {
			code := `
				rt = go do { 
					coyield 0

					coyield 1

					coyield 2
				}
	
				result = rt.wait_result!()

				step_results = []
				for step in rt.steps {
					step_results.append(step.result)
				}
				return [result, step_results]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.NewWrappedValueList(
				core.Nil,
				core.NewWrappedValueList(core.Int(0), core.Int(1), core.Int(2)),
			), res)
		})

		t.Run("embedded module yields once and has a return statement", func(t *testing.T) {
			code := `
				rt = go do { 
					coyield 0
					return "final result"
				}
	
				result = rt.wait_result!()

				step_results = []
				for step in rt.steps {
					step_results.append(step.result)
				}
				return [result, step_results]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.NewWrappedValueList(
				core.String("final result"),
				core.NewWrappedValueList(core.Int(0)),
			), res)
		})

		t.Run("patterns declared by an embedded module should not be declared in top level module's context", func(t *testing.T) {
			code := `
				pattern p1 = 1
				rt = go {} do {
					pattern p2 = 2
				}
	
				rt.wait_result!()
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			_, err := Eval(code, state, false)
			assert.NoError(t, err)

			assert.NotNil(t, state.Ctx.ResolveNamedPattern("p1"))
			assert.Nil(t, state.Ctx.ResolveNamedPattern("p2"))
		})

		t.Run("the source of a lthread's main chunk should be the source of the main module: call expression", func(t *testing.T) {
			code := `
				fn f(){
					return 1
				}
				lthread = go do f()
				return lthread
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			state.Logger = zerolog.New(state.Out)
			res, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}
			lthread := res.(*core.LThread)
			assert.Equal(t, state.Module.MainChunk.Source, lthread.Module().MainChunk.Source)
		})

		t.Run("the source of a lthread's main chunk should be the source of the main module: block", func(t *testing.T) {
			code := `
				fn f(){
					return 1
				}
				lthread = go do {}
				return lthread
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			state.Logger = zerolog.New(state.Out)
			res, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}
			lthread := res.(*core.LThread)
			assert.Equal(t, state.Module.MainChunk.Source, lthread.Module().MainChunk.Source)
		})

		t.Run("the source of the main chunk of a lthread spawned at the top level of an included file "+
			"should be the source of the included chunk", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return lthread
			`, map[string]string{"./dep.ix": `
				includable-file
				lthread = go do {}
			`})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			state.Logger = zerolog.New(state.Out)
			res, err := Eval(mod, state, false)
			if !assert.NoError(t, err) {
				return
			}
			lthread := res.(*core.LThread)
			assert.Equal(t, state.Module.IncludedChunkForest[0].Source, lthread.Module().MainChunk.Source)
		})

		t.Run("the source of the main chunk of a lthread spawned in a function that is defined in an included file "+
			"but called by the module should be the source of the included chunk", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return f()
			`, map[string]string{"./dep.ix": `
				includable-file
				fn f(){
					return go do {}
				}
			`})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			state.Logger = zerolog.New(state.Out)
			res, err := Eval(mod, state, false)
			if !assert.NoError(t, err) {
				return
			}
			lthread := res.(*core.LThread)
			assert.Equal(t, state.Module.IncludedChunkForest[0].Source, lthread.Module().MainChunk.Source)
		})
	})

	t.Run("mapping expression", func(t *testing.T) {
		// 	testconfig.AllowParallelization(t)

		// 	t.Run("empty", func(t *testing.T) {
		// 		code := `Mapping{}`
		// 		state := core.NewGlobalState(NewDefaultTestContext())
		// 		defer state.Ctx.CancelGracefully()

		// 		res, err := Eval(code, state, false)
		// 		assert.NoError(t, err)
		// 		assert.Equal(t, &Mapping{
		// 			keys:                         map[string]core.Serializable{},
		// 			preComputedStaticEntryValues: map[string]core.Serializable{},
		// 			dynamicEntries:               map[string]*parse.DynamicMappingEntry{},
		// 			patterns: []struct {
		// 				string
		// 				core.Pattern
		// 			}{},
		// 		}, res)
		// 	})

		// 	t.Run("not empty", func(t *testing.T) {
		// 		code := `Mapping{
		// 			0 => 1
		// 			1 => f()
		// 			n 2 => n
		// 		}`
		// 		state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
		// 			"f": core.WrapGoFunction(func(ctx *core.Context) core.Int {
		// 				return -1
		// 			}),
		// 		})
		// 		defer state.Ctx.CancelGracefully()

		// 		res, err := Eval(code, state, false)
		// 		mod := parse.MustParseChunk(code)

		// 		assert.NoError(t, err)
		// 		assert.Equal(t, &Mapping{
		// 			keys: map[string]core.Serializable{
		// 				`{"int__value":0}`: core.Int(0),
		// 				`{"int__value":1}`: core.Int(1),
		// 				`{"int__value":2}`: core.Int(2),
		// 			},
		// 			preComputedStaticEntryValues: map[string]core.Serializable{
		// 				`{"int__value":0}`: core.Int(1),
		// 			},
		// 			staticEntries: map[string]*parse.StaticMappingEntry{
		// 				`{"int__value":1}`: parse.FindNode(mod, &parse.StaticMappingEntry{}, nil),
		// 			},
		// 			dynamicEntries: map[string]*parse.DynamicMappingEntry{
		// 				`{"int__value":2}`: parse.FindNode(mod, &parse.DynamicMappingEntry{}, nil),
		// 			},
		// 			patterns: []struct {
		// 				string
		// 				core.Pattern
		// 			}{},
		// 		}, res)
		// 	})

		// 	t.Run("pattern identifier keys", func(t *testing.T) {
		// 		code := `Mapping{ %str => 1  n %int => n }`
		// 		state := core.NewGlobalState(NewDefaultTestContext())
		// 		defer state.Ctx.CancelGracefully()

		// 		state.Ctx.AddNamedPattern("str", core.STR_PATTERN)
		// 		state.Ctx.AddNamedPattern("int", core.INT_PATTERN)

		// 		res, err := Eval(code, state, false)
		// 		mod := parse.MustParseChunk(code)

		// 		strTypePatternRepr := core.GetJSONRepresentation(core.STR_PATTERN, state.Ctx, nil)
		// 		intTypePatternRepr := core.GetJSONRepresentation(core.INT_PATTERN, state.Ctx, nil)

		// 		assert.NoError(t, err)
		// 		assert.Equal(t, &Mapping{
		// 			keys: map[string]core.Serializable{
		// 				strTypePatternRepr: core.STR_PATTERN,
		// 				intTypePatternRepr: core.INT_PATTERN,
		// 			},
		// 			preComputedStaticEntryValues: map[string]core.Serializable{
		// 				strTypePatternRepr: core.Int(1),
		// 			},
		// 			dynamicEntries: map[string]*parse.DynamicMappingEntry{
		// 				intTypePatternRepr: parse.FindNode(mod, &parse.DynamicMappingEntry{}, nil),
		// 			},
		// 			patterns: []struct {
		// 				string
		// 				core.Pattern
		// 			}{
		// 				{strTypePatternRepr, core.STR_PATTERN},
		// 				{intTypePatternRepr, core.INT_PATTERN},
		// 			},
		// 		}, res)
		// 	})

		// 	t.Run("should not be sharable if one of the captured globals is not sharable", func(t *testing.T) {
		// 		code := `
		// 			globalvar a = 1
		// 			return Mapping{
		// 				0 => notsharable
		// 				1 => a
		// 			}
		// 		`
		// 		state := core.NewGlobalState(NewDefaultTestContext())
		// 		defer state.Ctx.CancelGracefully()
		// 		state.Globals.Set("notsharable", core.TestMutableGoValue{})

		// 		res, err := Eval(code, state, true)

		// 		if !assert.NoError(t, err) {
		// 			return
		// 		}
		// 		assert.False(t, utils.Ret0(res.(*core.Mapping).IsSharable(state)))
		// 	})

		// 	t.Run("should be sharable if all of the captured globals are sharable", func(t *testing.T) {
		// 		code := `
		// 			globalvar a = 1
		// 			b = 2
		// 			return Mapping{
		// 				0 => a
		// 				1 => b
		// 			}
		// 		`
		// 		state := core.NewGlobalState(NewDefaultTestContext())
		// 		defer state.Ctx.CancelGracefully()
		// 		state.Globals.Set("notsharable", core.TestMutableGoValue{})

		// 		res, err := Eval(code, state, true)

		// 		if !assert.NoError(t, err) {
		// 			return
		// 		}
		// 		assert.True(t, utils.Ret0(res.(*core.Mapping).IsSharable(state)))
		// 	})

		// })
	})

	t.Run("treedata literal", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("not empty", func(t *testing.T) {
			code := `treedata 0 { 1 {2} 3 }`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &core.Treedata{
				Root: core.Int(0),
				HiearchyEntries: []core.TreedataHiearchyEntry{
					{
						Value: core.Int(1),
						Children: []core.TreedataHiearchyEntry{
							{Value: core.Int(2)},
						},
					},
					{
						Value: core.Int(3),
					},
				},
			}, res)
		})

		t.Run("pair", func(t *testing.T) {
			code := `treedata 0 { 1: 2 }`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, &core.Treedata{
				Root: core.Int(0),
				HiearchyEntries: []core.TreedataHiearchyEntry{
					{
						Value: core.NewOrderedPair(core.Int(1), core.Int(2)),
					},
				},
			}, res)
		})
	})

	t.Run("Mapping", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("compute() with inexisting key", func(t *testing.T) {
			code := `
				m = Mapping{}
				return m.compute(0)
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
		})

		t.Run("compute() with existing static entry", func(t *testing.T) {
			code := `
				m = Mapping{0 => 1}
				return m.compute(0)
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(1), res)
		})

		t.Run("compute() with existing dynamic value static key entry", func(t *testing.T) {
			code := `
				globalvar v = "a"
				m = Mapping{0 => v}
				return m.compute(0)
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.String("a"), res)
		})

		t.Run("compute() with existing dynamic entry key", func(t *testing.T) {
			code := `
				m = Mapping{ n 0 => n}
				return m.compute(0)
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(0), res)
		})

		t.Run("compute() with existing dynamic entry key & group matching var", func(t *testing.T) {
			code := `
				m = Mapping{ p %/{:name} m => [p, m] }
				return m.compute(/a)
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.NewWrappedValueList(
				core.Path("/a"),
				core.NewObjectFromMap(core.ValMap{
					"0":    core.Path("/a"),
					"name": core.String("a"),
				}, state.Ctx),
			), res)
		})

		t.Run("compute() with existing dynamic entry key in many goroutines", func(t *testing.T) {
			if false {
				runtime.GC()
				startMemStats := new(runtime.MemStats)
				runtime.ReadMemStats(startMemStats)

				defer utils.AssertNoMemoryLeak(t, startMemStats, 100_000, utils.AssertNoMemoryLeakOptions{
					PreSleepDurationMillis: 100,
				})
			}

			code := `
				globalvar m = Mapping{ n 0 => n }

				group = LThreadGroup()

				for 1..10_000 {
					go {globals: .{m}, group: group} do {
						return m.compute(0)
					}
				}

				return group.wait_results!()
			`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			state := core.NewGlobalState(ctx, map[string]core.Value{
				"LThreadGroup": core.WrapGoFunction(core.NewLThreadGroup),
			})
			state.Out = os.Stdout
			state.Logger = zerolog.New(state.Out)

			res, err := Eval(code, state, true)
			if !assert.NoError(t, err) {
				return
			}

			assert.IsType(t, &core.Array{}, res)
			for _, e := range *res.(*core.Array) {
				if !assert.Equal(t, core.Int(0), e) {
					return
				}
			}
		})

		t.Run("compute() with existing dynamic entry key (accessing a global variable) in many goroutines", func(t *testing.T) {
			if false {
				runtime.GC()
				startMemStats := new(runtime.MemStats)
				runtime.ReadMemStats(startMemStats)
				goroutineCount := runtime.NumGoroutine()

				defer utils.AssertNoMemoryLeak(t, startMemStats, 100_000, utils.AssertNoMemoryLeakOptions{
					PreSleepDurationMillis: 100,
					CheckGoroutines:        true,
					GoroutineCount:         goroutineCount,
					MaxGoroutineCountDelta: 1,
				})
			}

			code := `
				globalvar a = 1
				globalvar m = Mapping{ n 0 => a }

				group = LThreadGroup()

				for 1..10_000 {
					go {globals: .{m}, group: group} do {
						return m.compute(0)
					}
				}

				return group.wait_results!()
			`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()

			state := core.NewGlobalState(ctx, map[string]core.Value{
				"LThreadGroup": core.WrapGoFunction(core.NewLThreadGroup),
			})

			state.Out = io.Discard
			state.Logger = zerolog.New(state.Out)

			res, err := Eval(code, state, true)
			if !assert.NoError(t, err) {
				return
			}

			assert.IsType(t, &core.Array{}, res)
			for _, e := range *res.(*core.Array) {
				if !assert.Equal(t, core.Int(1), e) {
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
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, false)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(0), res)
		})

	})

	t.Run("concatenation expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("single string element", func(t *testing.T) {
			code := `concat "a"`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, true)

			assert.NoError(t, err)
			assert.Equal(t, core.String("a"), res)
		})

		t.Run("two short string-like elements", func(t *testing.T) {
			code := `concat "a" "b"`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, true)

			assert.NoError(t, err)
			assert.Equal(t, core.String("ab"), res)
		})

		t.Run("two long string-like elements", func(t *testing.T) {
			oneString := core.String(strings.Repeat("b", 100))
			code := strings.ReplaceAll(`concat "b" "b"`, `b`, string(oneString))
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, true)

			assert.NoError(t, err)
			assert.Equal(t, core.NewStringConcatenation(oneString, oneString), res)
		})

		t.Run("single byteslice element", func(t *testing.T) {
			code := `concat 0d[12]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, true)

			assert.NoError(t, err)
			assert.Equal(t, core.NewByteSlice([]byte{12}, false, ""), res)
		})

		t.Run("two bytes-like elements", func(t *testing.T) {
			code := `concat 0d[12] 0d[34]`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{})
			res, err := Eval(code, state, true)

			assert.NoError(t, err)
			assert.Equal(t, core.NewBytesConcatenation(
				core.NewByteSlice([]byte{12}, false, ""),
				core.NewByteSlice([]byte{34}, false, ""),
			), res)
		})

		t.Run("bytes-like element followed by a spread element with a single item", func(t *testing.T) {
			code := `concat 0d[12] ...[0d[34]]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, true)

			assert.NoError(t, err)
			assert.Equal(t, core.NewBytesConcatenation(
				core.NewByteSlice([]byte{12}, false, ""),
				core.NewByteSlice([]byte{34}, false, ""),
			), res)
		})

		t.Run("bytes-like element followed by a spread element with two items", func(t *testing.T) {
			code := `concat 0d[12] ...[0d[34], 0d[56]]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, true)

			assert.NoError(t, err)
			assert.Equal(t, core.NewBytesConcatenation(
				core.NewByteSlice([]byte{12}, false, ""),
				core.NewByteSlice([]byte{34}, false, ""),
				core.NewByteSlice([]byte{56}, false, ""),
			), res)
		})

		t.Run("alternation of normal & spread bytes-like elements", func(t *testing.T) {
			code := `concat 0d[12] ...[0d[34], 0d[56]] 0d[78] ...[0d[91], 0d[23]]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, true)

			assert.NoError(t, err)
			assert.Equal(t, core.NewBytesConcatenation(
				core.NewByteSlice([]byte{12}, false, ""),
				core.NewByteSlice([]byte{34}, false, ""),
				core.NewByteSlice([]byte{56}, false, ""),
				core.NewByteSlice([]byte{78}, false, ""),
				core.NewByteSlice([]byte{91}, false, ""),
				core.NewByteSlice([]byte{23}, false, ""),
			), res)
		})

		t.Run("modifying an element of the concatenation should not change the concatenation value", func(t *testing.T) {
			code := `
				bytes = 0d[12]
				c = concat bytes 0d[34]
				bytes[0] = tobyte(24)
				return c
			`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"tobyte": core.WrapGoFunction(toByte),
			})
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, true)

			assert.NoError(t, err)
			assert.Equal(t, core.NewBytesConcatenation(
				core.NewByteSlice([]byte{12}, false, ""),
				core.NewByteSlice([]byte{34}, false, ""),
			), res)
		})

		t.Run("two tuples", func(t *testing.T) {
			code := `concat #[1] #["a"]`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{})
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, true)

			assert.NoError(t, err)
			assert.Equal(t, core.NewTuple([]core.Serializable{core.Int(1), core.String("a")}), res)
		})

		t.Run("string element followed by a spread element with a single item", func(t *testing.T) {
			code := `concat "a" ...["b"]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, true)

			assert.NoError(t, err)
			assert.Equal(t, "ab", res.(core.StringLike).GetOrBuildString())
		})

		t.Run("string element followed by a spread element with two items", func(t *testing.T) {
			code := `concat "a" ...["b", "c"]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, true)

			assert.NoError(t, err)
			assert.Equal(t, "abc", res.(core.StringLike).GetOrBuildString())
		})

		t.Run("alternation of normal & spread string elements", func(t *testing.T) {
			code := `concat "a" ...["b", "c"] "d" ...["e", "f"]`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(code, state, true)

			assert.NoError(t, err)
			assert.Equal(t, "abcdef", res.(core.StringLike).GetOrBuildString())
		})
	})

	t.Run("a value passed to a lthread and then returned by it should not be wrapped", func(t *testing.T) {
		called := false

		code := `
			rt = go {globals: {gofunc: $gofunc}} do {
				return $gofunc
			}

			f = rt.wait_result!()
			return f()
		`

		_ctx := NewDefaultTestContext()
		defer _ctx.CancelGracefully()

		state := core.NewGlobalState(_ctx, map[string]core.Value{
			"gofunc": core.ValOf(func(ctx *core.Context) core.Int {
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

		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()
		_, err := Eval(code, state, false)

		assert.True(t, state.Ctx.HasPermission(core.GlobalVarPermission{Kind_: permkind.Use, Name: "*"}))
		assert.False(t, state.Ctx.HasPermission(core.GlobalVarPermission{Kind_: permkind.Read, Name: "*"}))

		assert.NoError(t, err)
	})

	t.Run("pattern conversion expression,", func(t *testing.T) {
		t.Run("int literal", func(t *testing.T) {
			code := `%(1)`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()
			state := core.NewGlobalState(ctx)

			res, err := Eval(code, state, true)
			assert.NoError(t, err)
			assert.Equal(t, core.NewExactValuePattern(core.Int(1)), res)
		})

		t.Run("string literal", func(t *testing.T) {
			code := `%("s")`
			ctx := NewDefaultTestContext()
			defer ctx.CancelGracefully()
			state := core.NewGlobalState(ctx)

			res, err := Eval(code, state, true)
			assert.NoError(t, err)
			assert.Equal(t, core.NewExactStringPattern(core.String("s")), res)
		})

		t.Run("variable with an int value", func(t *testing.T) {
			code := `
				one = 1; 
				return %(one)
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewExactValuePattern(core.Int(1)), res)
		})

		t.Run("variable with a string value", func(t *testing.T) {
			code := `
				s = "s"; 
				return %(s)
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewExactStringPattern(core.String("s")), res)
		})
	})

	t.Run("pattern definition", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("RHS is an int literal", func(t *testing.T) {
			code := `
				pattern one = 1; 
				return %one
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewExactValuePattern(core.Int(1)), res)
		})

		t.Run("RHS is a string literal", func(t *testing.T) {
			code := `
				pattern s = "s"
				return %s
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewExactStringPattern(core.String("s")), res)
		})

		t.Run("RHS is a variable with an int value", func(t *testing.T) {
			code := `
				one = 1; 
				pattern one = $one
				return %one
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewExactValuePattern(core.Int(1)), res)
		})

		t.Run("RHS is a variable with a string value", func(t *testing.T) {
			code := `
				s = "s"; 
				pattern s = $s
				return %s
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewExactStringPattern(core.String("s")), res)
		})

		t.Run("RHS is an unprefixed object pattern", func(t *testing.T) {
			code := `pattern o = {}; return %o`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewInexactObjectPattern(nil), res)
		})

		t.Run("RHS is a prefixed object pattern", func(t *testing.T) {
			code := `pattern o = %{}; return %o`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewInexactObjectPattern(nil), res)
		})

		t.Run("RHS is an unprefixed object pattern with a unprefixed property pattern", func(t *testing.T) {
			code := `pattern o = {a: int}; return %o`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Ctx.AddNamedPattern("int", core.INT_PATTERN)
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewInexactObjectPattern([]core.ObjectPatternEntry{{Name: "a", Pattern: core.INT_PATTERN}}), res)
		})

		t.Run("pattern definition & identifiers : RHS is another pattern identifier", func(t *testing.T) {
			code := `pattern p = "p"; 
			pattern s = %p; 
			return %s`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewExactStringPattern(core.String("p")), res)
		})

		t.Run("pattern definition & identifiers : minimal lazy", func(t *testing.T) {
			code := `
				pattern s = @ %p
				prev = %s
				pattern p = "p"
				return [$prev, %s]
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			prev := res.(*core.List).At(state.Ctx, 0)
			pattern := res.(*core.List).At(state.Ctx, 1)
			assert.IsType(t, (*core.DynamicStringPatternElement)(nil), prev)
			assert.Equal(t, core.NewDynamicStringPatternElement("p", state.Ctx), pattern)
		})

		t.Run("pattern definition & identifiers : lazy", func(t *testing.T) {
			code := `
				pattern s = @ str( p "a" )
				prev = %s
				pattern p = "p"
				return [$prev, %s]
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			prev := res.(*core.List).At(state.Ctx, 0)
			pattern := res.(*core.List).At(state.Ctx, 1)
			assert.IsType(t, (*core.SequenceStringPattern)(nil), prev)
			assert.Same(t, prev, pattern)
		})

		t.Run("pattern definition: sequence string pattern: single element", func(t *testing.T) {
			code := `
				pattern s = str( 'a'..'z' )
				return %s
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.IsType(t, (*core.SequenceStringPattern)(nil), res)
			patt := res.(*core.SequenceStringPattern)
			assert.Len(t, patt.Elements(), 1)
		})

	})

	t.Run("pattern namespace definition", func(t *testing.T) {
		t.Run("RHS is an empty object literal", func(t *testing.T) {
			code := `pnamespace namespace. = {}`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)

			assert.Equal(t, map[string]*core.PatternNamespace{
				"namespace": {
					Patterns: map[string]core.Pattern{},
				},
			}, state.Ctx.GetPatternNamespaces())
		})

		t.Run("RHS is an object literal with patterns & non-pattern values", func(t *testing.T) {
			code := `pnamespace namespace. = {one: 1, empty_obj: %{}}`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)

			assert.Equal(t, map[string]*core.PatternNamespace{
				"namespace": {
					Patterns: map[string]core.Pattern{
						"one":       core.NewExactValuePattern(core.Int(1)),
						"empty_obj": core.NewInexactObjectPattern(nil),
					},
				},
			}, state.Ctx.GetPatternNamespaces())
		})
	})

	t.Run("pattern namespace member", func(t *testing.T) {
		code := `
			pnamespace namespace. = {one: 1}
			return %namespace.one
		`
		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()
		res, err := Eval(code, state, false)

		assert.NoError(t, err)
		assert.Equal(t, core.NewExactValuePattern(core.Int(1)), res)
	})

	t.Run("object pattern literal", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("empty", func(t *testing.T) {
			code := `%{}`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewInexactObjectPattern(nil), res)
		})

		t.Run("not empty", func(t *testing.T) {
			code := `pattern s = "s"; return %{name: %s, count: 2}`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewInexactObjectPattern(
				[]core.ObjectPatternEntry{
					{
						Name:    "count",
						Pattern: core.NewExactValuePattern(core.Int(2)),
					},
					{
						Name:    "name",
						Pattern: core.NewExactStringPattern(core.String("s")),
					},
				},
			), res)
		})

		t.Run("unprefixed named pattern", func(t *testing.T) {
			code := `pattern s = "s"; return %{name: s}`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewInexactObjectPattern(
				[]core.ObjectPatternEntry{
					{
						Name:    "name",
						Pattern: core.NewExactStringPattern(core.String("s")),
					},
				},
			), res)
		})

		t.Run("spread", func(t *testing.T) {

			t.Run("single-property object pattern before properties", func(t *testing.T) {
				code := `
					pattern s = "s"
					pattern user = %{name: "foo"}
					return %{...%user, s: %s}
				`

				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, core.NewInexactObjectPattern(
					[]core.ObjectPatternEntry{
						{
							Name:    "name",
							Pattern: core.NewExactStringPattern(core.String("foo")),
						},
						{
							Name:    "s",
							Pattern: core.NewExactStringPattern(core.String("s")),
						},
					},
				), res)
			})

			t.Run("single-property object pattern before same property with different type", func(t *testing.T) {
				code := `
					pattern s = "s"
					pattern user = %{name: "foo"}
					return %{...%user, name: "bar"}
				`

				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, core.NewInexactObjectPattern(
					[]core.ObjectPatternEntry{
						{
							Name:    "name",
							Pattern: core.NewExactStringPattern(core.String("bar")),
						},
					},
				), res)
			})

			t.Run("two-property object pattern before properties", func(t *testing.T) {
				code := `
					pattern s = "s"
					pattern user = %{name: "foo", age: 30}
					return %{...%user, s: %s}
				`

				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, core.NewInexactObjectPattern(
					[]core.ObjectPatternEntry{
						{
							Name:    "age",
							Pattern: core.NewExactValuePattern(core.Int(30)),
						},
						{
							Name:    "name",
							Pattern: core.NewExactStringPattern(core.String("foo")),
						},
						{
							Name:    "s",
							Pattern: core.NewExactStringPattern(core.String("s")),
						},
					},
				), res)
			})

			t.Run("complex", func(t *testing.T) {
				code := `
					pattern user = %{name: "foo"}
					return %{...%user, friends: %[]%user}
				`

				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, core.NewInexactObjectPattern(
					[]core.ObjectPatternEntry{
						{
							Name: "friends",
							Pattern: core.NewListPatternOf(core.NewInexactObjectPattern(
								[]core.ObjectPatternEntry{
									{
										Name:    "name",
										Pattern: core.NewExactStringPattern(core.String("foo")),
									},
								},
							)),
						},
						{
							Name:    "name",
							Pattern: core.NewExactStringPattern(core.String("foo")),
						},
					},
				), res)
			})

			t.Run("spread element is not an object pattern", func(t *testing.T) {
				code := `pattern s = "s"; return %{...%s}`

				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				res, err := Eval(code, state, false)

				assert.Error(t, err)
				assert.Nil(t, res)
			})
		})

	})

	t.Run("record pattern literal", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("empty", func(t *testing.T) {
			code := `pattern p = #{}; return %p`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewInexactRecordPattern(nil), res)
		})

		t.Run("not empty", func(t *testing.T) {
			code := `pattern s = "s"; pattern p = #{name: %s, count: 2}; return %p`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewInexactRecordPattern(
				[]core.RecordPatternEntry{
					{
						Name:    "count",
						Pattern: core.NewExactValuePattern(core.Int(2)),
					},
					{
						Name:    "name",
						Pattern: core.NewExactStringPattern(core.String("s")),
					},
				},
			), res)
		})

		t.Run("unprefixed named pattern", func(t *testing.T) {
			code := `pattern s = "s"; pattern p = #{name: s}; return %p`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewInexactRecordPattern(
				[]core.RecordPatternEntry{
					{
						Name:    "name",
						Pattern: core.NewExactStringPattern(core.String("s")),
					},
				},
			), res)
		})

		t.Run("spread", func(t *testing.T) {

			t.Run("single-property object pattern before properties", func(t *testing.T) {
				code := `
					pattern s = "s"
					pattern user = #{name: "foo"}
					pattern p = #{...%user, s: %s}
					return %p
				`

				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, core.NewInexactRecordPattern(
					[]core.RecordPatternEntry{
						{
							Name:    "name",
							Pattern: core.NewExactStringPattern(core.String("foo")),
						},
						{
							Name:    "s",
							Pattern: core.NewExactStringPattern(core.String("s")),
						},
					},
				), res)
			})

			t.Run("single-property record pattern before same property with different type", func(t *testing.T) {
				code := `
					pattern user = #{name: "foo"}
					pattern p = #{...%user, name: "bar"}
					return %p
				`

				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, core.NewInexactRecordPattern(
					[]core.RecordPatternEntry{
						{
							Name:    "name",
							Pattern: core.NewExactStringPattern(core.String("bar")),
						},
					},
				), res)
			})

			t.Run("two-property object pattern before properties", func(t *testing.T) {
				code := `
					pattern s = "s"
					pattern user = #{name: "foo", age: 30}
					pattern p = #{...%user, s: %s}
					return %p
				`

				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, core.NewInexactRecordPattern(
					[]core.RecordPatternEntry{
						{
							Name:    "age",
							Pattern: core.NewExactValuePattern(core.Int(30)),
						},
						{
							Name:    "name",
							Pattern: core.NewExactStringPattern(core.String("foo")),
						},
						{
							Name:    "s",
							Pattern: core.NewExactStringPattern(core.String("s")),
						},
					},
				), res)
			})

			t.Run("complex", func(t *testing.T) {
				code := `
					pattern s = "s"
					pattern user = #{name: "foo"}
					pattern p = #{...%user, friends: %[]%user}
					return %p
				`

				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, core.NewInexactRecordPattern(
					[]core.RecordPatternEntry{
						{
							Name: "friends",
							Pattern: core.NewListPatternOf(core.NewInexactRecordPattern(
								[]core.RecordPatternEntry{
									{
										Name:    "name",
										Pattern: core.NewExactStringPattern(core.String("foo")),
									},
								},
							)),
						},
						{
							Name:    "name",
							Pattern: core.NewExactStringPattern(core.String("foo")),
						},
					},
				), res)
			})

			t.Run("spread element is not an record pattern", func(t *testing.T) {
				code := `pattern s = "s"; pattern p = #{...%s}; return %p`

				state := core.NewGlobalState(NewDefaultTestContext())
				defer state.Ctx.CancelGracefully()
				res, err := Eval(code, state, false)

				assert.Error(t, err)
				assert.Nil(t, res)
			})
		})

	})

	t.Run("list pattern literal", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("empty", func(t *testing.T) {
			code := `%[]`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewListPattern(make([]core.Pattern, 0)), res)
		})

		t.Run("single element: integer literal", func(t *testing.T) {
			code := `%[ 2 ]`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewListPattern(
				[]core.Pattern{
					core.NewExactValuePattern(core.Int(2)),
				},
			), res)
		})

		t.Run("single element: empty object pattern", func(t *testing.T) {
			code := `%[ {} ]`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewListPattern(
				[]core.Pattern{
					core.NewInexactObjectPattern(nil),
				},
			), res)
		})

		t.Run("single element: empty object", func(t *testing.T) {
			code := `%[ %(#{}) ]`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewListPattern([]core.Pattern{core.NewExactValuePattern(core.NewEmptyRecord())}), res)
		})

		t.Run("single element: an object pattern literal", func(t *testing.T) {
			code := `%[ %{} ]`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewListPattern([]core.Pattern{core.NewInexactObjectPattern(nil)}), res)
		})

		t.Run("general element is an object pattern literal", func(t *testing.T) {
			code := `%[]%{}`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewListPatternOf(core.NewInexactObjectPattern(nil)), res)
		})

		t.Run("general element is an unprefixed object pattern literal", func(t *testing.T) {
			code := `%[]%{}`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewListPatternOf(core.NewInexactObjectPattern(nil)), res)
		})

		t.Run("general element is an unprefixed named pattern", func(t *testing.T) {
			code := `%[]int`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Ctx.AddNamedPattern("int", core.INT_PATTERN)
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewListPatternOf(core.INT_PATTERN), res)
		})
	})

	t.Run("tuple pattern literal", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("empty", func(t *testing.T) {
			code := `pattern p = #[]; return %p`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewTuplePattern(make([]core.Pattern, 0)), res)
		})

		t.Run("single element: integer literal", func(t *testing.T) {
			code := `pattern p = #[ 2 ]; return %p`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewTuplePattern([]core.Pattern{core.NewExactValuePattern(core.Int(2))}), res)
		})

		t.Run("single element: empty record pattern", func(t *testing.T) {
			code := `pattern p = #[ #{} ]; return %p`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewTuplePattern([]core.Pattern{core.NewInexactRecordPattern(nil)}), res)
		})

		t.Run("single element: empty record", func(t *testing.T) {
			code := `pattern p = #[ %(#{}) ]; return %p`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewTuplePattern([]core.Pattern{core.NewExactValuePattern(core.NewEmptyRecord())}), res)
		})

		t.Run("single element: an object pattern literal", func(t *testing.T) {
			code := `pattern p = #[ #{} ]; return %p`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewTuplePattern([]core.Pattern{core.NewInexactRecordPattern(nil)}), res)
		})

		t.Run("general element is an record pattern literal", func(t *testing.T) {
			code := `pattern p = #[]#{}; return %p`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewTuplePatternOf(core.NewInexactRecordPattern(nil)), res)
		})

		t.Run("general element is an unprefixed object pattern literal", func(t *testing.T) {
			code := `pattern p = #[]#{}; return %p`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewTuplePatternOf(core.NewInexactRecordPattern(nil)), res)
		})

		t.Run("general element is an unprefixed named pattern", func(t *testing.T) {
			code := `pattern p = #[]int; return %p`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Ctx.AddNamedPattern("int", core.INT_PATTERN)
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewTuplePatternOf(core.INT_PATTERN), res)
		})
	})

	t.Run("union pattern", func(t *testing.T) {
		code := `%| 1 | 2`

		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()
		res, err := Eval(code, state, false)

		assert.NoError(t, err)
		assert.Equal(t, []core.Pattern{
			core.NewExactValuePattern(core.Int(1)),
			core.NewExactValuePattern(core.Int(2)),
		}, res.(*core.UnionPattern).Cases())
	})

	t.Run("regex literal : empty", func(t *testing.T) {
		code := "%`a`"

		state := core.NewGlobalState(NewDefaultTestContext())
		defer state.Ctx.CancelGracefully()
		res, err := Eval(code, state, false)

		assert.NoError(t, err)
		assert.IsType(t, (*core.RegexPattern)(nil), res)
	})

	t.Run("assertion statement: true", func(t *testing.T) {

		t.Run("true", func(t *testing.T) {
			code := "assert true"

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
		})

		t.Run("false", func(t *testing.T) {
			code := "assert false"

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.Error(t, err)
			assert.Nil(t, res)
		})

	})

	t.Run("testsuite expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("no manifest", func(t *testing.T) {
			code := `return testsuite "name" {}`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			if !assert.IsType(t, &core.TestSuite{}, res) {
				return
			}
			assert.Equal(t, core.String("name"), res.(*core.TestSuite).Meta())
			assert.Equal(t, state.Module.MainChunk.Source, res.(*core.TestSuite).Module().MainChunk.Source)
		})

		t.Run("no meta", func(t *testing.T) {
			code := `return testsuite {}`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			if !assert.IsType(t, &core.TestSuite{}, res) {
				return
			}
			assert.Equal(t, core.Nil, res.(*core.TestSuite).Meta())
			assert.Equal(t, state.Module.MainChunk.Source, res.(*core.TestSuite).Module().MainChunk.Source)
		})

		t.Run("meta; name", func(t *testing.T) {
			code := `
				fn f(){
					return "my test suite"
				}
				return testsuite({name: f()}) {}
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			if !assert.IsType(t, &core.TestSuite{}, res) {
				return
			}
			if !assert.NotNil(t, core.Nil, res.(*core.TestSuite).Meta()) {
				return
			}
			assert.Equal(t, "my test suite", res.(*core.TestSuite).NameFromMeta())
		})

		t.Run("meta: name + fs", func(t *testing.T) {
			code := `
				fn f(){
					return "my test suite"
				}
				return testsuite({name: f(), fs: snapshot}) {}
			`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			fls := newMemFilesystem()
			snapshot := &memFilesystemSnapshot{fls: fls}
			state.Globals.Set("snapshot", core.WrapFsSnapshot(snapshot))

			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			if !assert.IsType(t, &core.TestSuite{}, res) {
				return
			}
			if !assert.NotNil(t, core.Nil, res.(*core.TestSuite).Meta()) {
				return
			}
			assert.Equal(t, "my test suite", res.(*core.TestSuite).NameFromMeta())
			assert.Equal(t, snapshot, utils.MustGet(res.(*core.TestSuite).FilesystemSnapshot()))
		})

		t.Run("the source of the main chunk of a testsuite created at the top level of an included file "+
			"should be the source of the included chunk", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return case
			`, map[string]string{"./dep.ix": `
				includable-file
				case = testsuite "name" {}
			`})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			state.Logger = zerolog.New(state.Out)
			res, err := Eval(mod, state, false)
			if !assert.NoError(t, err) {
				return
			}
			testSuite := res.(*core.TestSuite)
			assert.Equal(t, state.Module.IncludedChunkForest[0].Source, testSuite.Module().MainChunk.Source)
		})

		t.Run("the source of the main chunk of a testsuite created in a function that is defined in an included file "+
			"but called by the module should be the source of the included chunk", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return f()
			`, map[string]string{"./dep.ix": `
				includable-file
				fn f(){
					return testsuite "name" {}
				}
			`})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			state.Logger = zerolog.New(state.Out)
			res, err := Eval(mod, state, false)
			if !assert.NoError(t, err) {
				return
			}
			testSuite := res.(*core.TestSuite)
			assert.Equal(t, state.Module.IncludedChunkForest[0].Source, testSuite.Module().MainChunk.Source)
		})

	})

	t.Run("testcase expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("no manifest", func(t *testing.T) {
			code := `return testcase "name" {}`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			if !assert.IsType(t, &core.TestCase{}, res) {
				return
			}
			assert.Equal(t, core.String("name"), res.(*core.TestCase).Meta())
			assert.Equal(t, state.Module.MainChunk.Source, res.(*core.TestCase).Module().MainChunk.Source)
		})

		t.Run("no meta", func(t *testing.T) {
			code := `return testcase {}`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			if !assert.IsType(t, &core.TestCase{}, res) {
				return
			}
			assert.Equal(t, core.Nil, res.(*core.TestCase).Meta())
			assert.Equal(t, state.Module.MainChunk.Source, res.(*core.TestCase).Module().MainChunk.Source)
		})

		t.Run("the source of the main chunk of a testcase created at the top level of an included file "+
			"should be the source of the included chunk", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return case
			`, map[string]string{"./dep.ix": `
				includable-file
				case = testcase "name" {}
			`})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			state.Logger = zerolog.New(state.Out)
			res, err := Eval(mod, state, false)
			if !assert.NoError(t, err) {
				return
			}
			testCase := res.(*core.TestCase)
			assert.Equal(t, state.Module.IncludedChunkForest[0].Source, testCase.Module().MainChunk.Source)
		})

		t.Run("the source of the main chunk of a testcase created in a function that is defined in an included file "+
			"but called by the module should be the source of the included chunk", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return f()
			`, map[string]string{"./dep.ix": `
				includable-file
				fn f(){
					return testcase "name" {}
				}
			`})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			state.Logger = zerolog.New(state.Out)
			res, err := Eval(mod, state, false)
			if !assert.NoError(t, err) {
				return
			}
			testCase := res.(*core.TestCase)
			assert.Equal(t, state.Module.IncludedChunkForest[0].Source, testCase.Module().MainChunk.Source)
		})
	})

	t.Run("lifetimejob expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("no manifest", func(t *testing.T) {
			code := `return lifetimejob "name" {}`

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			if !assert.IsType(t, &core.LifetimeJob{}, res) {
				return
			}
			assert.Equal(t, core.String("name"), res.(*core.LifetimeJob).Meta())
			assert.Equal(t, state.Module.MainChunk.Source, res.(*core.LifetimeJob).Module().MainChunk.Source)
		})

		t.Run("the source of the main chunk of a lifetimejob created at the top level of an included file "+
			"should be the source of the included chunk", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return job
			`, map[string]string{"./dep.ix": `
				includable-file
				job = lifetimejob "name" {}
			`})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			state.Logger = zerolog.New(state.Out)
			res, err := Eval(mod, state, false)
			if !assert.NoError(t, err) {
				return
			}
			job := res.(*core.LifetimeJob)
			assert.Equal(t, state.Module.IncludedChunkForest[0].Source, job.Module().MainChunk.Source)
		})

		t.Run("the source of the main chunk of a lifetimejob create in a function that is defined in an included file "+
			"but called by the module should be the source of the included chunk", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, `
				manifest {}
				import ./dep.ix
				return f()
			`, map[string]string{"./dep.ix": `
				includable-file
				fn f(){
					return lifetimejob "name" {}
				}
			`})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			state.Logger = zerolog.New(state.Out)
			res, err := Eval(mod, state, false)
			if !assert.NoError(t, err) {
				return
			}
			job := res.(*core.LifetimeJob)
			assert.Equal(t, state.Module.IncludedChunkForest[0].Source, job.Module().MainChunk.Source)
		})
	})

	t.Run("testsuite statement", func(t *testing.T) {

		allTestsFilter := core.TestFilters{
			PositiveTestFilters: []core.TestFilter{
				{
					NameRegex: ".*",
				},
			},
		}

		makeSourceFile := func(code string) parse.SourceFile {
			return parse.SourceFile{
				NameString:             "/mod.ix",
				CodeString:             code,
				UserFriendlyNameString: "/mod.ix",
				Resource:               "/mod.ix",
				ResourceDir:            "/",
				IsResourceURL:          false,
			}
		}

		createModuleAndImports := func(code string, modules map[string]string) (*core.Module, afs.Filesystem, error) {
			fls := newMemFilesystem()
			err := util.WriteFile(fls, "/mod.ix", []byte(code), 0600)
			if err != nil {
				return nil, nil, err
			}

			for k, v := range modules {
				err := util.WriteFile(fls, k, []byte(v), 0600)
				if err != nil {
					return nil, nil, err
				}
			}

			ctx := core.NewContextWithEmptyState(core.ContextConfig{
				Permissions: []core.Permission{
					core.CreateFsReadPerm(core.PathPattern("/...")),
				},
				Filesystem: fls,
			}, nil)
			defer ctx.CancelGracefully()

			mod, err := core.ParseLocalModule("/mod.ix", core.ModuleParsingConfig{
				Context: ctx,
			})

			return mod, fls, err
		}

		t.Run("empty: testing disabled", func(t *testing.T) {
			src := makeSourceFile(`testsuite "name" {}`)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			assert.Empty(t, state.TestingState.SuiteResults)
		})

		t.Run("empty: testing disabled: meta should not be evaluted", func(t *testing.T) {
			src := makeSourceFile(`
				globalvar a = {value: 0}
				fn f(){
					a.value = 1
					return "my test suite"
				}

				testsuite({name: f()}) {
					
				}

				return $a.value
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Int(0), res)
			assert.Empty(t, state.TestingState.CaseResults)
			assert.Empty(t, state.TestingState.SuiteResults)
		})

		t.Run("should inherit patterns", func(t *testing.T) {

			src := makeSourceFile(`
				pattern p = 1
				testsuite "name" {
					val = %p
				}
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			assert.Len(t, state.TestingState.SuiteResults, 1)
		})

		t.Run("should inherit pattern namespaces", func(t *testing.T) {

			src := makeSourceFile(`
				pnamespace ns. = {a: %{a: 1}}
				testsuite "name" {
					val = %ns.
				}
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			assert.Len(t, state.TestingState.SuiteResults, 1)
		})

		t.Run("should inherit global variables", func(t *testing.T) {

			src := makeSourceFile(`
				globalvar host = https://localhost
				testsuite "name" {
					assert ($host/index.html == https://localhost/index.html)
				}
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			assert.Len(t, state.TestingState.SuiteResults, 1)
		})

		t.Run("empty", func(t *testing.T) {
			src := makeSourceFile(`testsuite "name" {}`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			assert.Len(t, state.TestingState.SuiteResults, 1)
		})

		t.Run("empty: filtered out", func(t *testing.T) {
			src := makeSourceFile(`testsuite "name" {}`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = core.TestFilters{
				PositiveTestFilters: []core.TestFilter{
					{
						NameRegex: "not this test",
					},
				},
			}
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			assert.Empty(t, state.TestingState.SuiteResults)
		})

		t.Run("empty: return statement after test suite", func(t *testing.T) {
			src := makeSourceFile(`
				testsuite "name" {}
				return 0
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Int(0), res)
			assert.Empty(t, state.TestingState.CaseResults)
			assert.Len(t, state.TestingState.SuiteResults, 1)
		})

		t.Run("empty in imported module", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}
				import res /imported.ix {
					allow: {
						create: {threads: {}}
					}
				}
			`, map[string]string{
				"/imported.ix": `
					manifest {
						permissions: {create: {threads: {}}}
					}

					testsuite "name" {}
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})
			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}
			assert.Empty(t, state.TestingState.SuiteResults[0].CaseResults)
		})

		t.Run("empty in imported module: disabled import testing", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}
				import res /imported.ix {
					allow: {
						create: {threads: {}}
					}
				}
			`, map[string]string{
				"/imported.ix": `
					manifest {
						permissions: {create: {threads: {}}}
					}

					testsuite "name" {}
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})
			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = false
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			assert.Empty(t, state.TestingState.SuiteResults)
		})

		t.Run("empty in included chunk", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}
				import /included.ix
			`, map[string]string{
				"/included.ix": `
					includable-file

					testsuite "name" {}
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})
			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}
			assert.Empty(t, state.TestingState.SuiteResults[0].CaseResults)
		})

		t.Run("empty in included chunk: disabled import testing", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}
				import /included.ix
			`, map[string]string{
				"/included.ix": `
					includable-file

					testsuite "name" {}
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})
			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = false
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			assert.Empty(t, state.TestingState.SuiteResults)
		})

		t.Run("empty in included chunk (deep)", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}
				import /included1.ix
			`, map[string]string{
				"/included1.ix": `
					includable-file

					import /included2.ix
				`,
				"/included2.ix": `
					includable-file

					testsuite "name" {}
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})
			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}
			assert.Empty(t, state.TestingState.SuiteResults[0].CaseResults)
		})

		t.Run("empty in included chunk (deep): disabled import testing", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
			manifest {}
			import /included1.ix
		`, map[string]string{
				"/included1.ix": `
				includable-file

				import /included2.ix
			`,
				"/included2.ix": `
				includable-file

				testsuite "name" {}
			`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})
			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = false
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			assert.Empty(t, state.TestingState.SuiteResults)
		})

		t.Run("if a fs snapshot is specified the filesystem should be created from it", func(t *testing.T) {
			src := makeSourceFile(`
				testsuite({fs: snapshot}) {
					test_read_file(/file.txt)
					test_read_file(/not-existing.txt)
				}
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			fls := newMemFilesystem()
			util.WriteFile(fls, "/file.txt", []byte("content"), 0400)
			snapshot := &memFilesystemSnapshot{fls: fls}
			state.Globals.Set("snapshot", core.WrapFsSnapshot(snapshot))

			callCount := 0
			state.Globals.Set("test_read_file", core.WrapGoFunction(func(ctx *core.Context, path core.Path) {
				content, err := util.ReadFile(ctx.GetFileSystem(), path.UnderlyingString())
				callCount++
				if path == "/not-existing.txt" {
					assert.ErrorIs(t, err, os.ErrNotExist)
				} else {
					if !assert.NoError(t, err) {
						return
					}
					assert.Equal(t, "content", string(content))
				}
			}))

			res, err := Eval(src, state, false)
			if !assert.NoError(t, err) {
				return
			}
			if !assert.Equal(t, 2, callCount) {
				return
			}
			assert.Equal(t, core.Nil, res)
		})

		t.Run("empty test case", func(t *testing.T) {
			src := makeSourceFile(`testsuite "name" {
				testcase {}
			}`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
		})

		t.Run("empty test case: test suite in imported module", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}
				import res /imported.ix {
					allow: {
						create: {threads: {}}
					}
				}
			`, map[string]string{
				"/imported.ix": `
					manifest {
						permissions: {create: {threads: {}}}
					}

					testsuite "name" {
						testcase {}
					}
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})
			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			assert.Len(t, state.TestingState.SuiteResults[0].CaseResults, 1)
		})

		t.Run("empty test case: test suite in included chunk", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}
				import /included.ix
			`, map[string]string{
				"/included.ix": `
					includable-file

					testsuite "name" {
						testcase {}
					}
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})
			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}
			assert.Len(t, state.TestingState.SuiteResults[0].CaseResults, 1)
		})

		t.Run("test cases should inherit the patterns of the parent testsuite", func(t *testing.T) {
			src := makeSourceFile(`
				pattern p1 = 1

				testsuite "name" {
					pattern p2 = 2

					testcase {
						val = [%p1, %p2]
					}
				}
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
		})

		t.Run("test cases should inherit the pattern namespaces of the parent testsuite", func(t *testing.T) {
			src := makeSourceFile(`
				pnamespace ns1. = {a: %{a: 1}}

				testsuite "name" {
					pnamespace ns2. = {a: %{a: 2}}

					testcase {
						val = [%ns1., %ns2.]
					}
				}
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
		})

		t.Run("test cases should inherit the global variables of the parent testsuite", func(t *testing.T) {
			src := makeSourceFile(`
				globalvar host1 = https://localhost:8081

				testsuite "name" {
					globalvar host2 = https://localhost:8082

					testcase {
						assert ($host1/index.html == https://localhost:8081/index.html)
						assert ($host2/index.html == https://localhost:8082/index.html)
					}
				}
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
		})

		t.Run("test cases should have http clients configured for all provided localhost hosts ", func(t *testing.T) {

			src := makeSourceFile(`
				manifest {
					permissions: {
						provide: https://localhost:8081
					}
				}

				testsuite "name" {
					testcase {
						 ok = is_client_insecure_and_stateful(https://localhost:8081)
						 assert ok 
						 ok = is_client_insecure_and_stateful(https://localhost:8081)
						 assert !ok
					}
				}
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			state.Globals.Set("is_client_insecure_and_stateful", core.WrapGoFunction(isClientInsecureAndStateful))

			defer state.Ctx.CancelGracefully()

			//Generate a self signed certificate and spins 2 HTTP server (localhost:8081 and localhost/8082)

			res, err := Eval(src, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)

			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			testSuitResult := state.TestingState.SuiteResults[0]
			if !assert.Len(t, testSuitResult.CaseResults, 1) {
				return
			}

			caseResult := testSuitResult.CaseResults[0]
			if !assert.False(t, caseResult.Success) {
				return
			}
		})

		t.Run("manifest with ungranted permissions", func(t *testing.T) {
			src := makeSourceFile(`testsuite "name" {
				manifest {    
					permissions: { read: https://example.com/index.html }
				}
			}`)

			state := core.NewGlobalState(core.NewContext(core.ContextConfig{
				Permissions: []core.Permission{core.LThreadPermission{Kind_: permkind.Create}},
				Limits:      []core.Limit{permissiveLthreadLimit},
			}))
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			assert.Error(t, err)
			assert.ErrorIs(t, err, core.NewNotAllowedError(core.HttpPermission{
				Kind_:  permkind.Read,
				Entity: core.URL("https://example.com/index.html"),
			}))
			assert.Nil(t, res)
		})

		t.Run("test case with failing assertion", func(t *testing.T) {
			src := makeSourceFile(`testsuite "name" {
				testcase {
					assert false
				}
			}`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			state.Out = os.Stdout
			state.Logger = zerolog.New(state.Out)
			res, err := Eval(src, state, false)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Nil, res)

			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			testSuitResult := state.TestingState.SuiteResults[0]
			if !assert.Len(t, testSuitResult.CaseResults, 1) {
				return
			}

			caseResult := testSuitResult.CaseResults[0]
			if !assert.False(t, caseResult.Success) {
				return
			}

			if !assert.NotNil(t, caseResult.AssertionError) {
				return
			}

			if !assert.True(t, caseResult.AssertionError.IsTestAssertion()) {
				return
			}
		})

		t.Run("test case failing because of an unexpected error", func(t *testing.T) {
			src := makeSourceFile(`testsuite "name" {
				testcase {
					(1 / 0)
				}
			}`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			state.Out = os.Stdout
			state.Logger = zerolog.New(state.Out)
			res, err := Eval(src, state, false)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Nil, res)

			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			testSuitResult := state.TestingState.SuiteResults[0]
			if !assert.Len(t, testSuitResult.CaseResults, 1) {
				return
			}

			caseResult := testSuitResult.CaseResults[0]
			if !assert.False(t, caseResult.Success) {
				return
			}

			if !assert.Nil(t, caseResult.AssertionError) {
				return
			}

			if !assert.NotNil(t, caseResult.Error) {
				return
			}
		})

		t.Run("test case with failing assertion: testing disabled", func(t *testing.T) {
			src := makeSourceFile(`testsuite "name" {
				testcase {
					assert false
				}
			}`)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			state.Out = os.Stdout
			state.Logger = zerolog.New(state.Out)
			res, err := Eval(src, state, false)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.SuiteResults)
			assert.Empty(t, state.TestingState.CaseResults)
		})

		t.Run("test case with failing assertion followed by a passing test case", func(t *testing.T) {
			src := makeSourceFile(`testsuite "name" {
				testcase {
					assert false
				}
				testcase {
					
				}
			}`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter

			state.Out = os.Stdout
			state.Logger = zerolog.New(state.Out)
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Nil, res)

			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			testSuitResult := state.TestingState.SuiteResults[0]
			if !assert.Len(t, testSuitResult.CaseResults, 2) {
				return
			}

			caseResult1 := testSuitResult.CaseResults[0]
			if !assert.False(t, caseResult1.Success) {
				return
			}

			if !assert.NotNil(t, caseResult1.AssertionError) {
				return
			}

			if !assert.True(t, caseResult1.AssertionError.IsTestAssertion()) {
				return
			}

			caseResult2 := testSuitResult.CaseResults[1]
			if !assert.True(t, caseResult2.Success) {
				return
			}
		})

		t.Run("test case because of an unexpected error followed by a passing test case", func(t *testing.T) {
			src := makeSourceFile(`testsuite "name" {
				testcase {
					(1 / 0)
				}
				testcase {
					
				}
			}`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			state.Out = os.Stdout
			state.Logger = zerolog.New(state.Out)
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Nil, res)

			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			testSuitResult := state.TestingState.SuiteResults[0]
			if !assert.Len(t, testSuitResult.CaseResults, 2) {
				return
			}

			caseResult1 := testSuitResult.CaseResults[0]
			if !assert.False(t, caseResult1.Success) {
				return
			}

			if !assert.Nil(t, caseResult1.AssertionError) {
				return
			}

			if !assert.NotNil(t, caseResult1.Error) {
				return
			}

			caseResult2 := testSuitResult.CaseResults[1]
			if !assert.True(t, caseResult2.Success) {
				return
			}
		})

		t.Run("test case with failing assertion followed by a failing test case", func(t *testing.T) {
			src := makeSourceFile(`testsuite "name" {
				testcase {
					assert false
				}
				testcase {
					assert false
				}
			}`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter

			state.Out = os.Stdout
			state.Logger = zerolog.New(state.Out)
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Nil, res)

			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			testSuitResult := state.TestingState.SuiteResults[0]
			if !assert.Len(t, testSuitResult.CaseResults, 2) {
				return
			}

			caseResult1 := testSuitResult.CaseResults[0]
			if !assert.False(t, caseResult1.Success) {
				return
			}

			if !assert.NotNil(t, caseResult1.AssertionError) {
				return
			}

			if !assert.True(t, caseResult1.AssertionError.IsTestAssertion()) {
				return
			}

			caseResult2 := testSuitResult.CaseResults[1]
			if !assert.False(t, caseResult2.Success) {
				return
			}

			if !assert.NotNil(t, caseResult2.AssertionError) {
				return
			}

			if !assert.True(t, caseResult2.AssertionError.IsTestAssertion()) {
				return
			}
		})

		t.Run("sub test suite case with failing assertion", func(t *testing.T) {
			src := makeSourceFile(`testsuite "name" {
				testsuite "" {
					testcase {
						assert false
					}
				}
			}`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter

			state.Out = os.Stdout
			state.Logger = zerolog.New(state.Out)
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.Nil, res)

			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			testSuitResult := state.TestingState.SuiteResults[0]
			if !assert.Len(t, testSuitResult.SubSuiteResults, 1) {
				return
			}
			assert.Empty(t, testSuitResult.CaseResults)

			subSuiteResult := testSuitResult.SubSuiteResults[0]

			if !assert.Len(t, subSuiteResult.CaseResults, 1) {
				return
			}

			caseResult := subSuiteResult.CaseResults[0]
			if !assert.False(t, caseResult.Success) {
				return
			}
		})

		t.Run("if a fs snapshot is specified the filesystem of test cases should be created from it", func(t *testing.T) {
			src := makeSourceFile(`
				testsuite({fs: snapshot}) {
					# modifications done by the test suite should have no impact.
					remove_file /file.txt

					testcase {
						test_read_file(/file.txt)
						test_read_file(/not-existing.txt)
					}
				}
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			fls := newMemFilesystem()
			util.WriteFile(fls, "/file.txt", []byte("content"), 0400)
			snapshot := &memFilesystemSnapshot{fls: fls}
			state.Globals.Set("snapshot", core.WrapFsSnapshot(snapshot))

			callCount := 0
			state.Globals.Set("test_read_file", core.WrapGoFunction(func(ctx *core.Context, path core.Path) {
				content, err := util.ReadFile(ctx.GetFileSystem(), path.UnderlyingString())
				callCount++
				if path == "/not-existing.txt" {
					assert.ErrorIs(t, err, os.ErrNotExist)
				} else {
					if !assert.NoError(t, err) {
						return
					}
					assert.Equal(t, "content", string(content))
				}
			}))

			state.Globals.Set("remove_file", core.WrapGoFunction(func(ctx *core.Context, path core.Path) {
				err := ctx.GetFileSystem().Remove(path.UnderlyingString())
				assert.NoError(t, err)
			}))

			res, err := Eval(src, state, false)
			if !assert.NoError(t, err) {
				return
			}
			if !assert.Equal(t, 2, callCount) {
				return
			}
			assert.Equal(t, core.Nil, res)
		})

		t.Run("if a fs snapshot is specified and pass-live-fs-snapshot-to-subtests: true then"+
			"the filesystem of test cases should be created from the live filesystem of the suite", func(t *testing.T) {
			src := makeSourceFile(`
				testsuite({
					fs: snapshot
					pass-live-fs-copy-to-subtests: true
				}) {
					# modifications done by the test suite should have an impact.
					remove_file /file1.txt

					testcase {
						test_read_file(/file1.txt)
						test_read_file(/file2.txt)
						test_read_file(/not-existing.txt)

						# modifications done by the test case should have no impact.
						remove_file /file2.txt
					}

					testcase {
						test_read_file(/file1.txt)
						test_read_file(/file2.txt)
						test_read_file(/not-existing.txt)
					}
				}
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			fls := newMemFilesystem()
			util.WriteFile(fls, "/file1.txt", []byte("content1"), 0400)
			util.WriteFile(fls, "/file2.txt", []byte("content2"), 0400)
			snapshot := &memFilesystemSnapshot{fls: fls}
			state.Globals.Set("snapshot", core.WrapFsSnapshot(snapshot))

			callCount := 0
			state.Globals.Set("test_read_file", core.WrapGoFunction(func(ctx *core.Context, path core.Path) {
				content, err := util.ReadFile(ctx.GetFileSystem(), path.UnderlyingString())
				callCount++
				if path == "/file2.txt" {
					if !assert.NoError(t, err) {
						return
					}
					assert.Equal(t, "content2", string(content))
				} else {
					assert.ErrorIs(t, err, os.ErrNotExist)
				}
			}))

			state.Globals.Set("remove_file", core.WrapGoFunction(func(ctx *core.Context, path core.Path) {
				err := ctx.GetFileSystem().Remove(path.UnderlyingString())
				assert.NoError(t, err)
			}))

			res, err := Eval(src, state, false)
			if !assert.NoError(t, err) {
				return
			}
			if !assert.Equal(t, 6, callCount) {
				return
			}
			assert.Equal(t, core.Nil, res)
		})

		t.Run("if the filter's name only matches the top level suite, all sub tests should be executed", func(t *testing.T) {
			src := makeSourceFile(`testsuite "suite" {
				testcase "shallow" {

				}

				testsuite {
					testcase "deep" {

					}
				}
			}`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = core.TestFilters{
				PositiveTestFilters: []core.TestFilter{
					{
						NameRegex: "suite",
					},
				},
			}
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, core.Nil, res)

			testSuitResult := state.TestingState.SuiteResults[0]
			assert.Len(t, testSuitResult.CaseResults, 1)

			if !assert.Len(t, testSuitResult.SubSuiteResults, 1) {
				return
			}

			subSuiteResult := testSuitResult.SubSuiteResults[0]
			if !assert.Len(t, subSuiteResult.CaseResults, 1) {
				return
			}
			assert.Equal(t, "shallow", testSuitResult.CaseResults[0].TestCase.Name())
		})

		t.Run("if the filter specifies a path and the node span of a test case (direct child), the test case should be executed", func(t *testing.T) {
			src := makeSourceFile(`testsuite "suite" {
				testcase "shallow 1" {

				}

				testcase "shallow 2" {

				}

				testsuite {
					testcase "deep" {

					}
				}
			}`)

			chunk := parse.MustParseChunk(src.CodeString)
			testcaseNode := chunk.Statements[0].(*parse.TestSuiteExpression).Module.Statements[0].(*parse.TestCaseExpression)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = core.TestFilters{
				PositiveTestFilters: []core.TestFilter{
					{
						AbsolutePath: "/mod.ix",
						NameRegex:    ".*",
						NodeSpan:     testcaseNode.Span,
					},
				},
			}
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, core.Nil, res)

			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			testSuitResult := state.TestingState.SuiteResults[0]
			if !assert.Len(t, testSuitResult.CaseResults, 1) {
				return
			}
			assert.Len(t, testSuitResult.SubSuiteResults, 0)
			assert.Equal(t, "shallow 1", testSuitResult.CaseResults[0].TestCase.Name())
		})

		t.Run("if the filter specifies a path and the node span of a test case (not direct child), the test case should be executed", func(t *testing.T) {
			src := makeSourceFile(`testsuite "suite" {
				testcase "shallow 1" {

				}

				testcase "shallow 2" {

				}

				testsuite {
					testcase "deep 1" {

					}

					testcase "deep 2" {

					}
				}
			}`)

			chunk := parse.MustParseChunk(src.CodeString)
			testcaseNode := chunk.Statements[0].(*parse.TestSuiteExpression).
				Module.Statements[2].(*parse.TestSuiteExpression).
				Module.Statements[0].(*parse.TestCaseExpression)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = core.TestFilters{
				PositiveTestFilters: []core.TestFilter{
					{
						AbsolutePath: "/mod.ix",
						NameRegex:    ".*",
						NodeSpan:     testcaseNode.Span,
					},
				},
			}
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, core.Nil, res)

			testSuitResult := state.TestingState.SuiteResults[0]
			assert.Empty(t, testSuitResult.CaseResults)

			if !assert.Len(t, testSuitResult.SubSuiteResults, 1) {
				return
			}

			subsuiteResult := testSuitResult.SubSuiteResults[0]

			if !assert.Len(t, subsuiteResult.CaseResults, 1) {
				return
			}
			assert.Equal(t, "deep 1", subsuiteResult.CaseResults[0].TestCase.Name())
		})

		t.Run("if the filter specifies a path and the node span of a test suite (direct child), the test suite should be executed", func(t *testing.T) {
			src := makeSourceFile(`testsuite "suite" {
				testcase "shallow 1" {

				}

				testsuite {
					testcase "deep 1" {

					}

					testcase "deep 2" {

					}
				}
			}`)

			chunk := parse.MustParseChunk(src.CodeString)
			testsuiteNode := chunk.Statements[0].(*parse.TestSuiteExpression).Module.Statements[1].(*parse.TestSuiteExpression)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = core.TestFilters{
				PositiveTestFilters: []core.TestFilter{
					{
						AbsolutePath: "/mod.ix",
						NameRegex:    ".*",
						NodeSpan:     testsuiteNode.Span,
					},
				},
			}
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, core.Nil, res)

			testSuitResult := state.TestingState.SuiteResults[0]
			assert.Empty(t, testSuitResult.CaseResults)

			if !assert.Len(t, testSuitResult.SubSuiteResults, 1) {
				return
			}

			subsuiteResult := testSuitResult.SubSuiteResults[0]
			assert.Len(t, subsuiteResult.CaseResults, 2)
		})

		t.Run("if the test filter specifies the path /mod.ix, the tests in /imported.ix should not be executed", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}
				import res /imported.ix {
					allow: {
						create: {threads: {}}
					}
				}
			`, map[string]string{
				"/imported.ix": `
					manifest {
						permissions: {create: {threads: {}}}
					}

					testsuite "name" {
						testcase {}
					}
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})
			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = true
			state.TestingState.Filters = core.TestFilters{
				PositiveTestFilters: []core.TestFilter{
					{
						AbsolutePath: "/mod.ix",
						NameRegex:    ".*",
					},
				},
			}
			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			assert.Empty(t, state.TestingState.SuiteResults)
		})

		t.Run("if the test filter specifies the path /imported.ix, the tests in /mod.ix should not be executed", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}
				import res /imported.ix {
					allow: {
						create: {threads: {}}
					}
				}

				testsuite "in mod.ix" {
					testcase {}
				}

			`, map[string]string{
				"/imported.ix": `
					manifest {
						permissions: {create: {threads: {}}}
					}

					testsuite "in imported.ix" {
						testcase {}
					}
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})
			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = true
			state.TestingState.Filters = core.TestFilters{
				PositiveTestFilters: []core.TestFilter{
					{
						AbsolutePath: "/imported.ix",
						NameRegex:    ".*",
					},
				},
			}
			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			result := state.TestingState.SuiteResults[0]
			if !assert.Len(t, result.CaseResults, 1) {
				return
			}

			assert.Equal(t, "/imported.ix", result.TestSuite.ParentChunk().Source.Name())
			assert.Equal(t, "/imported.ix", result.CaseResults[0].TestCase.ParentChunk().Source.Name())
		})

		t.Run("if the filter's name matches a test case (direct child) in the top level suite, only the matching test case should be executed", func(t *testing.T) {
			src := makeSourceFile(`testsuite "suite" {
				testcase "my test" {

				}

				testcase "my other test" {

				}

				testcase {

				}

				testsuite {
					testcase {

					}

					testcase "my test" {
						
					}
				}
			}`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = core.TestFilters{
				PositiveTestFilters: []core.TestFilter{
					{
						NameRegex: "suite::my test",
					},
				},
			}
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, core.Nil, res)

			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			testSuitResult := state.TestingState.SuiteResults[0]
			if !assert.Len(t, testSuitResult.CaseResults, 1) {
				return
			}

			assert.Equal(t, "my test", testSuitResult.CaseResults[0].TestCase.Name())
			assert.Empty(t, testSuitResult.SubSuiteResults)
		})

		t.Run("if the filter's name only matches a test case (not direct child) in the top level suite, only the matching test case should be executed", func(t *testing.T) {
			src := makeSourceFile(`testsuite "suite" {
				testcase "my test" {

				}

				testcase "my other test" {

				}

				testcase {

				}

				testsuite "sub suite" {
					testcase {

					}

					testcase "my test (deep)" {
						
					}
				}
			}`)

			state := core.NewGlobalState(NewDefaultTestContext())
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = core.TestFilters{
				PositiveTestFilters: []core.TestFilter{
					{
						NameRegex: "suite::sub suite::my test",
					},
				},
			}
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			if !assert.NoError(t, err) {
				return
			}
			assert.Equal(t, core.Nil, res)

			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			testSuitResult := state.TestingState.SuiteResults[0]
			assert.Empty(t, testSuitResult.CaseResults)
			if !assert.Len(t, testSuitResult.SubSuiteResults, 1) {
				return
			}

			subsuiteResult := testSuitResult.SubSuiteResults[0]
			if !assert.Len(t, subsuiteResult.CaseResults, 1) {
				return
			}
			assert.Equal(t, "my test (deep)", subsuiteResult.CaseResults[0].TestCase.Name())
		})

		//setup for following tests

		if !core.AreDefaultScriptLimitsSet() {
			core.SetDefaultScriptLimits([]core.Limit{})
			defer core.UnsetDefaultScriptLimits()
		}

		newDefaultContext := func(config core.DefaultContextConfig) (*core.Context, error) {

			ctxConfig := core.ContextConfig{
				Permissions:          config.Permissions,
				ForbiddenPermissions: config.ForbiddenPermissions,
				Limits:               config.Limits,
				HostDefinitions:      config.HostDefinitions,
				ParentContext:        config.ParentContext,
				ParentStdLibContext:  config.ParentStdLibContext,
				Filesystem:           config.Filesystem,
				OwnedDatabases:       config.OwnedDatabases,
			}

			if ctxConfig.ParentContext != nil {
				if err, _ := ctxConfig.Check(); err != nil {
					return nil, err
				}
			}

			ctx := core.NewContext(ctxConfig)

			for k, v := range core.DEFAULT_NAMED_PATTERNS {
				ctx.AddNamedPattern(k, v)
			}

			for k, v := range core.DEFAULT_PATTERN_NAMESPACES {
				ctx.AddPatternNamespace(k, v)
			}

			return ctx, nil
		}

		newDefaultContextBackup := core.NewDefaultContext
		defer func() {
			core.NewDefaultContext = newDefaultContextBackup
		}()
		core.NewDefaultContext = newDefaultContext

		newDefaultGlobalStateBackup := core.NewDefaultGlobalState
		defer func() {
			core.NewDefaultGlobalState = newDefaultGlobalStateBackup
		}()

		//billy.memfs is not thread safe
		var flsLock sync.Mutex

		core.NewDefaultGlobalState = func(ctx *core.Context, conf core.DefaultGlobalStateConfig) (*core.GlobalState, error) {

			writeFile := func(ctx *core.Context, path core.Path) {
				flsLock.Lock()
				defer flsLock.Unlock()

				err := util.WriteFile(ctx.GetFileSystem(), path.UnderlyingString(), []byte("content"), 0600)
				assert.NoError(t, err)
			}

			symbWriteFile := func(ctx *symbolic.Context, path *symbolic.Path) {

			}

			if !core.IsSymbolicEquivalentOfGoFunctionRegistered(writeFile) {
				core.RegisterSymbolicGoFunction(writeFile, symbWriteFile)
			}

			state := core.NewGlobalState(ctx, map[string]core.Value{
				"write_file": core.WrapGoFunction(writeFile),
			})

			state.Out = conf.Out

			state.Logger = conf.Logger
			if reflect.ValueOf(state.Logger).IsZero() {
				state.Logger = zerolog.New(conf.LogOut)
			}
			if reflect.ValueOf(state.Logger).IsZero() {
				state.Logger = zerolog.New(conf.Out)
			}
			return state, nil
		}

		if _, ok := core.GetOpenDbFn("ldb"); !ok {
			core.RegisterOpenDbFn("ldb", func(ctx *core.Context, config core.DbOpenConfiguration) (core.Database, error) {
				return &core.DummyDatabase{
					Resource_:        config.Resource,
					SchemaUpdated:    false,
					TopLevelEntities: map[string]core.Serializable{},
				}, nil
			})
		}

		t.Run("program specified by top level suite", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}
				
				testsuite({
					program: /program.ix
				}) {

				}

			`, map[string]string{
				"/program.ix": `
					manifest {

					}
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})

			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = true
			state.TestingState.Filters = allTestsFilter

			projectID := core.RandomProjectID("test")
			state.Project = &TestProject{
				ID: projectID,
				Img: &testImage{
					snapshot: &memFilesystemSnapshot{
						fls: copyMemFs(fls),
					},
					projectID: projectID,
				},
			}

			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			assert.Empty(t, state.TestingState.SuiteResults[0].CaseResults, 0)
		})

		t.Run("program specified by top level suite: empty testcase", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}
				
				testsuite({
					program: /program.ix
				}) {
					testcase {}
				}

			`, map[string]string{
				"/program.ix": `
					manifest {

					}
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})

			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = true
			state.TestingState.Filters = allTestsFilter

			projectID := core.RandomProjectID("test")
			state.Project = &TestProject{
				ID: projectID,
				Img: &testImage{
					snapshot: &memFilesystemSnapshot{
						fls: copyMemFs(fls),
					},
					projectID: projectID,
				},
			}

			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			assert.Len(t, state.TestingState.SuiteResults[0].CaseResults, 1)
		})

		t.Run("program specified by top level suite: testcase should have access to the program", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}
				
				testsuite({
					program: /program.ix
				}) {
					testcase {
						check_program_not_nil(__test.program)
					}
				}

			`, map[string]string{
				"/program.ix": `
					manifest {

					}
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})

			var isNotNil atomic.Bool

			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = true
			state.TestingState.Filters = allTestsFilter

			projectID := core.RandomProjectID("test")
			state.Project = &TestProject{
				ID: projectID,
				Img: &testImage{
					snapshot: &memFilesystemSnapshot{
						fls: copyMemFs(fls),
					},
					projectID: projectID,
				},
			}
			state.Globals.Set("check_program_not_nil", core.WrapGoFunction(func(ctx *core.Context, v core.Value) {
				program, ok := v.(*core.TestedProgram)
				if !assert.True(t, ok) {
					return
				}
				if !assert.NotNil(t, program.LThread()) {
					return
				}
				isNotNil.Store(true)
			}))

			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			assert.Len(t, state.TestingState.SuiteResults[0].CaseResults, 1)
			assert.True(t, isNotNil.Load())
		})

		t.Run("program specified by top level suite: testcase should be able to cancel the program", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}
				
				testsuite({
					program: /program.ix
				}) {
					testcase {
						__test.program.cancel()
						sleep10ms()
						check_program_is_done(__test.program)
					}
				}

			`, map[string]string{
				"/program.ix": `
					manifest {

					}
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})

			var isDone atomic.Bool

			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = true
			state.TestingState.Filters = allTestsFilter

			projectID := core.RandomProjectID("test")
			state.Project = &TestProject{
				ID: projectID,
				Img: &testImage{
					snapshot: &memFilesystemSnapshot{
						fls: copyMemFs(fls),
					},
					projectID: projectID,
				},
			}
			state.Globals.Set("sleep10ms", core.WrapGoFunction(func(ctx *core.Context) {
				core.Sleep(ctx, core.Duration(10*time.Millisecond))
			}))

			state.Globals.Set("check_program_is_done", core.WrapGoFunction(func(ctx *core.Context, program *core.TestedProgram) {
				if !assert.True(t, program.LThread().IsDone()) {
					return
				}
				isDone.Store(true)
			}))

			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			assert.Len(t, state.TestingState.SuiteResults[0].CaseResults, 1)
			assert.True(t, isDone.Load())
		})

		t.Run("program specified by top level suite: testcase and program should use the same filesystem", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}

				testsuite({
					program: /program.ix
				}) {
					testcase {
						sleep10ms()
						test_read_file /file_in_shared_fs.txt
					}
				}

			`, map[string]string{
				"/program.ix": `
					manifest {
						permissions: {
							write: %/...
						}
					}

					write_file /file_in_shared_fs.txt
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
					core.FilesystemPermission{permkind.Write, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})

			var correctFile atomic.Bool

			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = true
			state.TestingState.Filters = allTestsFilter

			projectID := core.RandomProjectID("test")
			state.Project = &TestProject{
				ID: projectID,
				Img: &testImage{
					snapshot: &memFilesystemSnapshot{
						fls: copyMemFs(fls),
					},
					projectID: projectID,
				},
			}

			state.Globals.Set("sleep10ms", core.WrapGoFunction(func(ctx *core.Context) {
				core.Sleep(ctx, core.Duration(10*time.Millisecond))
			}))

			state.Globals.Set("test_read_file", core.WrapGoFunction(func(ctx *core.Context, path core.Path) {
				flsLock.Lock()
				defer flsLock.Unlock()

				content, err := util.ReadFile(ctx.GetFileSystem(), path.UnderlyingString())
				if !assert.NoError(t, err) {
					return
				}
				if !assert.Equal(t, "content", string(content)) {
					return
				}
				correctFile.Store(true)
			}))

			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			assert.Len(t, state.TestingState.SuiteResults[0].CaseResults, 1)

			assert.True(t, correctFile.Load())
		})

		t.Run("program specified by top level suite: testcase in sub testsuite and program should use the same filesystem", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}

				testsuite({
					program: /program.ix
				}) {
					testsuite {
						testcase {
							sleep10ms()
							test_read_file /file_in_shared_fs.txt
						}
					}
				}

			`, map[string]string{
				"/program.ix": `
					manifest {
						permissions: {
							write: %/...
						}
					}

					write_file /file_in_shared_fs.txt
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
					core.FilesystemPermission{permkind.Write, core.PathPattern("/...")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})

			var correctFile atomic.Bool

			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = true
			state.TestingState.Filters = allTestsFilter

			projectID := core.RandomProjectID("test")
			state.Project = &TestProject{
				ID: projectID,
				Img: &testImage{
					snapshot: &memFilesystemSnapshot{
						fls: copyMemFs(fls),
					},
					projectID: projectID,
				},
			}

			state.Globals.Set("sleep10ms", core.WrapGoFunction(func(ctx *core.Context) {
				core.Sleep(ctx, core.Duration(10*time.Millisecond))
			}))

			state.Globals.Set("test_read_file", core.WrapGoFunction(func(ctx *core.Context, path core.Path) {
				flsLock.Lock()
				defer flsLock.Unlock()

				content, err := util.ReadFile(ctx.GetFileSystem(), path.UnderlyingString())
				if !assert.NoError(t, err) {
					return
				}
				if !assert.Equal(t, "content", string(content)) {
					return
				}
				correctFile.Store(true)
			}))

			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			assert.True(t, correctFile.Load())
			if !assert.Len(t, state.TestingState.SuiteResults[0].SubSuiteResults, 1) {
				return
			}
			assert.Len(t, state.TestingState.SuiteResults[0].SubSuiteResults[0].CaseResults, 1)
		})

		t.Run("main db schema and migrations specified by top level suite: main database should be initialized in test case", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}
				
				testsuite({
					program: /program.ix
					main-db-schema: %{
						user: {
							name: "foo"
						}
					}
					main-db-migrations: {
						inclusions: :{
							%/user: {
								name: "foo"
							}
						}
					}
				}) {
					testcase {
						check_databases(__test.program.dbs)
					}
				}

			`, map[string]string{
				"/program.ix": `
					manifest {
						databases: {
							main: {
								resource: ldb://main
								resolution-data: nil
							}
						}
					}
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
					core.FilesystemPermission{permkind.Write, core.PathPattern("/...")},
					core.DatabasePermission{permkind.Read, core.Host("ldb://main")},
					core.DatabasePermission{permkind.Write, core.Host("ldb://main")},
					core.DatabasePermission{permkind.Delete, core.Host("ldb://main")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})

			var isProperlyInitialized atomic.Bool

			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = true
			state.TestingState.Filters = allTestsFilter

			projectID := core.RandomProjectID("test")
			state.Project = &TestProject{
				ID: projectID,
				Img: &testImage{
					snapshot: &memFilesystemSnapshot{
						fls: copyMemFs(fls),
					},
					projectID: projectID,
				},
			}
			state.Globals.Set("check_databases", core.WrapGoFunction(func(ctx *core.Context, ns *core.Namespace) {
				if !assert.Contains(t, ns.PropertyNames(ctx), "main") {
					return
				}

				database := ns.Prop(ctx, "main").(*core.DatabaseIL)

				if !assert.True(t, database.TopLevelEntitiesLoaded()) {
					return
				}

				user := database.Prop(ctx, "user").(*core.Object)

				if !assert.Contains(t, user.PropertyNames(ctx), "name") {
					return
				}

				assert.Equal(t, core.String("foo"), user.Prop(ctx, "name"))

				isProperlyInitialized.Store(true)
			}))
			state.Ctx.AddNamedPattern("str", core.STR_PATTERN)

			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			if !assert.Len(t, state.TestingState.SuiteResults[0].CaseResults, 1) {
				return
			}
			result := state.TestingState.SuiteResults[0].CaseResults[0]
			if !assert.NoError(t, result.Error) {
				return
			}
			assert.True(t, isProperlyInitialized.Load())
		})

		t.Run("main db schema and migrations specified by top level suite: main database should be initialized in test case located in sub suite", func(t *testing.T) {

			mod, fls, err := createModuleAndImports(`
				manifest {}
				
				testsuite({
					program: /program.ix
					main-db-schema: %{
						user: {
							name: "foo"
						}
					}
					main-db-migrations: {
						inclusions: :{
							%/user: {
								name: "foo"
							}
						}
					}
				}) {
					testsuite {
						testcase {
							check_databases(__test.program.dbs)
						}
					}
				}

			`, map[string]string{
				"/program.ix": `
					manifest {
						databases: {
							main: {
								resource: ldb://main
								resolution-data: nil
							}
						}
					}
				`,
			})

			if !assert.NoError(t, err) {
				return
			}

			ctx := core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(),
					core.LThreadPermission{permkind.Create},
					core.FilesystemPermission{permkind.Read, core.PathPattern("/...")},
					core.FilesystemPermission{permkind.Write, core.PathPattern("/...")},
					core.DatabasePermission{permkind.Read, core.Host("ldb://main")},
					core.DatabasePermission{permkind.Write, core.Host("ldb://main")},
					core.DatabasePermission{permkind.Delete, core.Host("ldb://main")},
				),
				Filesystem: fls,
				Limits:     []core.Limit{permissiveLthreadLimit},
			})
			defer ctx.CancelGracefully()

			var isProperlyInitialized atomic.Bool

			state := core.NewGlobalState(ctx)
			state.TestingState.IsTestingEnabled = true
			state.TestingState.IsImportTestingEnabled = true
			state.TestingState.Filters = allTestsFilter

			projectID := core.RandomProjectID("test")
			state.Project = &TestProject{
				ID: projectID,
				Img: &testImage{
					snapshot: &memFilesystemSnapshot{
						fls: copyMemFs(fls),
					},
					projectID: projectID,
				},
			}
			state.Globals.Set("check_databases", core.WrapGoFunction(func(ctx *core.Context, ns *core.Namespace) {
				if !assert.Contains(t, ns.PropertyNames(ctx), "main") {
					return
				}

				database := ns.Prop(ctx, "main").(*core.DatabaseIL)

				if !assert.True(t, database.TopLevelEntitiesLoaded()) {
					return
				}

				user := database.Prop(ctx, "user").(*core.Object)

				if !assert.Contains(t, user.PropertyNames(ctx), "name") {
					return
				}

				assert.Equal(t, core.String("foo"), user.Prop(ctx, "name"))

				isProperlyInitialized.Store(true)
			}))
			state.Ctx.AddNamedPattern("str", core.STR_PATTERN)

			defer state.Ctx.CancelGracefully()

			res, err := Eval(mod, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.Nil, res)
			assert.Empty(t, state.TestingState.CaseResults)
			if !assert.Len(t, state.TestingState.SuiteResults, 1) {
				return
			}

			if !assert.Len(t, state.TestingState.SuiteResults[0].SubSuiteResults, 1) {
				return
			}
			subSuiteResult := state.TestingState.SuiteResults[0].SubSuiteResults[0]

			if !assert.Len(t, subSuiteResult.CaseResults, 1) {
				return
			}
			result := subSuiteResult.CaseResults[0]
			if !assert.NoError(t, result.Error) {
				return
			}
			assert.True(t, isProperlyInitialized.Load())
		})

		t.Run("main db schema and migrations specified by top level suite: tested program should be allowed to update the data", func(t *testing.T) {
			//TODO

			//this test requires the definiting of a top-level collection or container in the schema
		})
	})

	t.Run("testcase statement", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		allTestsFilter := core.TestFilters{
			PositiveTestFilters: []core.TestFilter{
				{
					NameRegex: ".*",
				},
			},
		}

		makeSourceFile := func(code string) parse.SourceFile {
			return parse.SourceFile{
				NameString:             "/mod.ix",
				CodeString:             code,
				UserFriendlyNameString: "/mod.ix",
				Resource:               "/mod.ix",
				ResourceDir:            "/",
				IsResourceURL:          false,
			}
		}

		t.Run("manifest with ungranted permissions", func(t *testing.T) {
			src := makeSourceFile(`
				testsuite {
					testcase "name" {
						manifest {    
							permissions: { read: https://example.com/index.html }
						}
					}
				}
			`)

			state := core.NewGlobalState(core.NewContext(core.ContextConfig{
				Permissions: append(core.GetDefaultGlobalVarPermissions(), core.LThreadPermission{Kind_: permkind.Create}),
				Limits:      []core.Limit{permissiveLthreadLimit},
			}))
			state.TestingState.IsTestingEnabled = true
			state.TestingState.Filters = allTestsFilter
			defer state.Ctx.CancelGracefully()

			res, err := Eval(src, state, false)

			assert.Error(t, err)
			assert.ErrorIs(t, err, core.NewNotAllowedError(core.HttpPermission{
				Kind_:  permkind.Read,
				Entity: core.URL("https://example.com/index.html"),
			}))
			assert.Nil(t, res)
		})
	})

	t.Run("string template literal", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		replace := func(s string) string {
			return strings.ReplaceAll(s, "|", "`")
		}

		t.Run("no interpolation", func(t *testing.T) {
			code := replace(`
				pattern digit = str('0'..'9')
				return %digit|3|
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewCheckedStringNoCheck("3", "digit", state.Ctx.ResolveNamedPattern("digit")), res)
		})

		t.Run("no pattern, no interpolation", func(t *testing.T) {
			code := replace(`return |3|`)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.String("3"), res)
		})

		t.Run("valid interpolations", func(t *testing.T) {
			code := replace(`
				pnamespace sql. = {
					stmt: %str( %|.*| )
					int: %str( '0'..'9'+ )
				}
				unsanitized_id = "5"
				return %sql.stmt|SELECT * FROM users WHERE id = ${int:$unsanitized_id}|
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.NewCheckedStringNoCheck(
				"SELECT * FROM users WHERE id = 5",
				"sql.stmt",
				state.Ctx.ResolvePatternNamespace("sql").Patterns["stmt"],
			), res)
		})

		t.Run("valid interpolation with conversion", func(t *testing.T) {
			code := replace(`
				return %ns.any_str|integer = ${int_str.from:5}|
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Ctx.AddPatternNamespace("ns", &core.PatternNamespace{
				Patterns: map[string]core.Pattern{
					"any_str": core.STR_PATTERN,
					"int_str": utils.Ret0(core.INT_PATTERN.StringPattern()),
				},
			})
			res, err := Eval(code, state, false)

			assert.NoError(t, err)

			expectedCheckedString := core.NewCheckedStringNoCheck("integer = 5", "ns.any_str", state.Ctx.ResolvePatternNamespace("ns").Patterns["any_str"])
			assert.Equal(t, expectedCheckedString, res)
		})

		t.Run("invalid interpolation", func(t *testing.T) {
			code := replace(`
				pnamespace sql. = {
					stmt: %str( %|.*| )
					int: %str( '0'..'9'+ )
				}
				unsanitized_id = "e5"
				return %sql.stmt|SELECT * FROM users WHERE id = ${int:$unsanitized_id}|
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.Error(t, err)
			assert.Nil(t, res)
		})

		t.Run("final string does not match pattern", func(t *testing.T) {
			code := replace(`
				pnamespace sql. = {
					stmt: %str( %|x.*| )
					int: %str( '0'..'9'+ )
				}
				unsanitized_id = "5"
				return %sql.stmt|SELECT * FROM users WHERE id = ${int:$unsanitized_id}|
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.Error(t, err)
			assert.Nil(t, res)
		})

		t.Run("no pattern, leading interpolation", func(t *testing.T) {
			code := replace(`
				s = "1"
				return |${s}2|
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.String("12"), res)
		})

		t.Run("no pattern, trailing interpolation", func(t *testing.T) {
			code := replace(`
				s = "2"
				return |1${s}|
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.String("12"), res)
		})

		t.Run("no pattern, interpolation & escaped n (\\n)", func(t *testing.T) {
			code := replace(`
				s = "1"
				return |${s}\n2|
			`)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.String("1\n2"), res)
		})

		t.Run("no pattern, interpolation & linefeed", func(t *testing.T) {
			code := replace("s = \"1\"; return |${s}\n2|")

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			res, err := Eval(code, state, false)

			assert.NoError(t, err)
			assert.Equal(t, core.String("1\n2"), res)
		})
	})

	t.Run("sendval expression", func(t *testing.T) {

	})

	t.Run("transaction", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		newState := func() (*core.GlobalState, **core.Transaction) {
			_tx := new(*core.Transaction)

			return core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"start_tx": core.ValOf(func(ctx *core.Context) *core.Transaction {
					*_tx = core.StartNewTransaction(ctx)
					return *_tx
				}),
				"do_reversible_side_effect": core.ValOf(func(ctx *core.Context) {
					effect := &reversibleEffect{}
					if ctx.HasCurrentTx() {
						if err := ctx.GetTx().AddEffect(ctx, effect); err != nil {
							panic(err)
						}
					}
				}),
				"do_irreversible_side_effect": core.ValOf(func(ctx *core.Context) {
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
				defer state.Ctx.CancelGracefully()

				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, core.Nil, res)
				assert.NotNil(t, tx)

				expectedEffects := []core.Effect{
					&reversibleEffect{applied: false},
				}

				assert.Equal(t, expectedEffects, utils.Must((*tx).CurrentEffects()))
			})

			t.Run("single, irreversible side effect", func(t *testing.T) {
				code := `
					tx = start_tx()
					do_irreversible_side_effect()
					return "after"
				`

				state, tx := newState()
				defer state.Ctx.CancelGracefully()

				res, err := Eval(code, state, false)

				assert.ErrorIs(t, err, core.ErrCannotAddIrreversibleEffect)
				assert.Nil(t, res)
				assert.NotNil(t, tx)

				assert.Empty(t, utils.Must((*tx).CurrentEffects()))
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
				defer state.Ctx.CancelGracefully()

				res, err := Eval(code, state, false)

				assert.NoError(t, err)
				assert.Equal(t, core.Nil, res)
				assert.NotNil(t, tx)

				expectedEffects := []core.Effect{
					&reversibleEffect{applied: true},
				}

				assert.Equal(t, expectedEffects, utils.Must((*tx).CurrentEffects()))
			})
		})

	})

	t.Run("map fn", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		t.Run("recursive map calls", func(t *testing.T) {
			code := `
				fn rec(list %serializable-iterable){
				    assert (list match %[]%serializable-iterable)
					return map(list, rec)
				}

				return rec([ [ [], [] ], [ [], [] ]])
			`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"map": core.WrapGoFunction(core.MapIterable),
			})
			defer state.Ctx.CancelGracefully()

			state.Ctx.AddNamedPattern("serializable-iterable", core.SERIALIZABLE_ITERABLE_PATTERN)

			res, err := Eval(code, state, true)
			assert.NoError(t, err)
			assert.EqualValues(t, core.NewWrappedValueList(
				core.NewWrappedValueList(core.NewWrappedValueListFrom([]core.Serializable{}), core.NewWrappedValueListFrom([]core.Serializable{})),
				core.NewWrappedValueList(core.NewWrappedValueListFrom([]core.Serializable{}), core.NewWrappedValueListFrom([]core.Serializable{})),
			), res)
		})

		t.Run("recursive map calls witin a function called in isolation", func(t *testing.T) {
			code := `
				fn rec(list %serializable-iterable){
				    assert (list match %[]%serializable-iterable)
					return map(list, rec)
				}

				fn isolated(){
					return rec([ [ [], [] ], [ [], [] ]])
				}

				return isolated
			`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"map": core.WrapGoFunction(core.MapIterable),
			})
			defer state.Ctx.CancelGracefully()

			state.Ctx.AddNamedPattern("serializable-iterable", core.SERIALIZABLE_ITERABLE_PATTERN)

			val, err := Eval(code, state, true)
			if !assert.NoError(t, err) {
				return
			}

			fn := val.(*core.InoxFunction)
			res, err := fn.Call(state, nil, nil, nil)
			if !assert.NoError(t, err) {
				return
			}

			assert.EqualValues(t, core.NewWrappedValueList(
				core.NewWrappedValueList(core.NewWrappedValueListFrom([]core.Serializable{}), core.NewWrappedValueListFrom([]core.Serializable{})),
				core.NewWrappedValueList(core.NewWrappedValueListFrom([]core.Serializable{}), core.NewWrappedValueListFrom([]core.Serializable{})),
			), res)
		})

		t.Run("recursive map calls witin a method called in isolation", func(t *testing.T) {
			code := `
				fn rec(list %serializable-iterable){
				    assert (list match %[]%serializable-iterable)
					return map(list, rec)
				}

				obj = {
					isolated: fn(){
						return rec([ [ [], [] ], [ [], [] ]])
					}
				}
		
				return obj.isolated
			`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"map": core.WrapGoFunction(core.MapIterable),
			})
			defer state.Ctx.CancelGracefully()

			state.Ctx.AddNamedPattern("serializable-iterable", core.SERIALIZABLE_ITERABLE_PATTERN)

			val, err := Eval(code, state, true)
			if !assert.NoError(t, err) {
				return
			}

			fn := val.(*core.InoxFunction)
			res, err := fn.Call(state, nil, nil, nil)
			if !assert.NoError(t, err) {
				return
			}

			assert.EqualValues(t, core.NewWrappedValueList(
				core.NewWrappedValueList(core.NewWrappedValueListFrom([]core.Serializable{}), core.NewWrappedValueListFrom([]core.Serializable{})),
				core.NewWrappedValueList(core.NewWrappedValueListFrom([]core.Serializable{}), core.NewWrappedValueListFrom([]core.Serializable{})),
			), res)
		})
	})

	t.Run("XML expression", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		__idt := core.WrapGoFunction(func(ctx *core.Context, e *core.XMLElement) *core.XMLElement {
			return e
		})

		createNamespaceWithFactory := func() *core.Namespace {
			return core.NewNamespace("x", map[string]core.Value{symbolic.FROM_XML_FACTORY_NAME: __idt})
		}

		t.Run("element", func(t *testing.T) {
			code := `idt<div></div>`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div", nil, []core.Value{core.String("")}), val)
		})

		t.Run("self-closing element", func(t *testing.T) {
			code := `idt<div/>`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div", nil, nil), val)
		})

		t.Run("implicit namespace", func(t *testing.T) {
			code := `(<div></div>)`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				globalnames.HTML_NS: createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div", nil, []core.Value{core.String("")}), val)
		})

		t.Run("integer attribute", func(t *testing.T) {
			code := `idt<div a=1></div>`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement(
				"div",
				[]core.XMLAttribute{core.NewXMLAttribute("a", core.Int(1))},
				[]core.Value{core.String("")},
			), val)
		})

		t.Run("string attribute", func(t *testing.T) {
			code := `idt<div a="b"></div>`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div", []core.XMLAttribute{core.NewXMLAttribute("a", core.String("b"))}, []core.Value{core.String("")}), val)
		})

		t.Run("attribute without value", func(t *testing.T) {
			code := `idt<div a></div>`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div", []core.XMLAttribute{core.NewXMLAttribute("a", core.DEFAULT_XML_ATTR_VALUE)}, []core.Value{core.String("")}), val)
		})

		t.Run("value of attribute should be HTML escaped", func(t *testing.T) {
			code := `idt<div a="<"></div>`
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div", []core.XMLAttribute{core.NewXMLAttribute("a", core.String("<"))}, []core.Value{core.String("")}), val)
		})

		t.Run("linefeed", func(t *testing.T) {
			code := "idt<div>\n</div>"
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div", nil, []core.Value{core.String("\n")}), val)
		})

		t.Run("raw text element", func(t *testing.T) {
			code := "idt<script><a></script>"
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewRawTextXmlElement("script", nil, "<a>"), val)
		})

		t.Run("empty child", func(t *testing.T) {
			code := "idt<div><span></span></div>"
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div", nil, []core.Value{
				core.String(""),
				core.NewXmlElement("span", nil, []core.Value{core.String("")}),
				core.String(""),
			}), val)
		})

		t.Run("single attribute + empty child", func(t *testing.T) {
			code := "idt<div a=1><span></span></div>"
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div",
				[]core.XMLAttribute{core.NewXMLAttribute("a", core.Int(1))},
				[]core.Value{
					core.String(""),
					core.NewXmlElement("span", nil, []core.Value{core.String("")}),
					core.String(""),
				}), val)
		})

		t.Run("two attributes + empty child", func(t *testing.T) {
			code := "idt<div a=1 b=2><span></span></div>"
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div",
				[]core.XMLAttribute{
					core.NewXMLAttribute("a", core.Int(1)),
					core.NewXMLAttribute("b", core.Int(2)),
				},
				[]core.Value{
					core.String(""),
					core.NewXmlElement("span", nil, []core.Value{core.String("")}),
					core.String(""),
				}), val)
		})

		t.Run("linefeed followed by empty child", func(t *testing.T) {
			code := "idt<div>\n<span></span></div>"
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div", nil, []core.Value{
				core.String("\n"),
				core.NewXmlElement("span", nil, []core.Value{core.String("")}),
				core.String(""),
			}), val)
		})

		t.Run("non-empty child", func(t *testing.T) {
			code := "idt<div><span>1</span></div>"
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div", nil, []core.Value{
				core.String(""),
				core.NewXmlElement("span", nil, []core.Value{core.String("1")}),
				core.String(""),
			}), val)
		})

		t.Run("two empty children", func(t *testing.T) {
			code := "idt<div><span></span><span></span></div>"
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div", nil, []core.Value{
				core.String(""),
				core.NewXmlElement("span", nil, []core.Value{core.String("")}),
				core.String(""),
				core.NewXmlElement("span", nil, []core.Value{core.String("")}),
				core.String(""),
			}), val)
		})

		t.Run("child + grandchild", func(t *testing.T) {
			code := "idt<div><span><span></span></span></div>"
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div", nil, []core.Value{
				core.String(""),
				core.NewXmlElement("span", nil, []core.Value{
					core.String(""),
					core.NewXmlElement("span", nil, []core.Value{core.String("")}),
					core.String(""),
				}),
				core.String(""),
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
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			_, err := Eval(code, state, false)
			assert.NoError(t, err)
		})

		t.Run("interpolation: XML element", func(t *testing.T) {
			code := "idt<div>{idt<span></span>}</div>"
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div", nil, []core.Value{
				core.String(""),
				core.NewXmlElement("span", nil, []core.Value{core.String("")}),
				core.String(""),
			}), val)
		})

		t.Run("interpolation: string", func(t *testing.T) {
			code := "idt<div>{\"a\"}</div>"
			state := core.NewGlobalState(NewDefaultTestContext(), map[string]core.Value{
				"idt": createNamespaceWithFactory(),
			})
			defer state.Ctx.CancelGracefully()

			val, err := Eval(code, state, false)
			if !assert.NoError(t, err) {
				return
			}

			assert.Equal(t, core.NewXmlElement("div", nil, []core.Value{
				core.String(""),
				core.String("a"),
				core.String(""),
			}), val)
		})
	})

	t.Run("new expression", func(t *testing.T) {
		t.Run("without init", func(t *testing.T) {
			code := `
				struct MyStruct {
					a int
				}

				return new MyStruct
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			state.Ctx.AddNamedPattern("int", core.INT_PATTERN)

			res, err := Eval(code, state, true)
			assert.NoError(t, err)
			assert.IsType(t, (*core.Struct)(nil), res)
		})

		t.Run("with init of single-field struct", func(t *testing.T) {
			code := `
				struct MyStruct {
					a int
				}

				ptr = new MyStruct {a: 3}
				return ptr.a
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			state.Ctx.AddNamedPattern("int", core.INT_PATTERN)

			res, err := Eval(code, state, true)
			assert.NoError(t, err)
			assert.Equal(t, core.Int(3), res)
		})

		t.Run("with init of two-field struct", func(t *testing.T) {
			code := `
				struct MyStruct {
					a int
					b int
				}

				ptr = new MyStruct {a: 3, b: 4}
				return [ptr.a, ptr.b]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			state.Ctx.AddNamedPattern("int", core.INT_PATTERN)

			res, err := Eval(code, state, true)
			assert.NoError(t, err)
			assert.Equal(t, core.NewWrappedValueList(core.Int(3), core.Int(4)), res)
		})

		t.Run("with init of two-field struct: alternate order", func(t *testing.T) {
			code := `
				struct MyStruct {
					a int
					b int
				}

				ptr = new MyStruct {b: 4, a: 3}
				return [ptr.a, ptr.b]
			`
			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()

			state.Ctx.AddNamedPattern("int", core.INT_PATTERN)

			res, err := Eval(code, state, true)
			assert.NoError(t, err)
			assert.Equal(t, core.NewWrappedValueList(core.Int(3), core.Int(4)), res)
		})
	})

	t.Run("error position stack", func(t *testing.T) {
		testconfig.AllowParallelization(t)

		joinLines := func(lines ...string) string {
			return strings.Join(lines, "\n")
		}

		t.Run("in included file", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, "manifest {}\nimport ./dep.ix", map[string]string{
				"./dep.ix": joinLines(
					"includable-file",
					"const( a = (1 / 0) )",
				),
			})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			includedChunk := mod.IncludedChunkForest[0]
			importStmt := parse.FindNode(mod.MainChunk.Node, (*parse.InclusionImportStatement)(nil), nil)
			binExpr := parse.FindNode(includedChunk.Node, (*parse.BinaryExpression)(nil), nil)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Module = mod

			_, err = Eval(mod, state, true)

			var locatedError core.LocatedEvalError
			if !assert.ErrorAs(t, err, &locatedError) {
				return
			}

			assert.Equal(t, parse.SourcePositionStack{
				{
					SourceName:  mod.MainChunk.Source.Name(),
					StartLine:   2,
					StartColumn: 1,
					EndLine:     2,
					EndColumn:   16,
					Span:        importStmt.Span,
				},
				{
					SourceName:  includedChunk.Name(),
					StartLine:   2,
					StartColumn: 12,
					EndLine:     2,
					EndColumn:   19,
					Span:        binExpr.Span,
				},
			}, locatedError.Location)
		})

		t.Run("in an included file (deep)", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName, "manifest {}\nimport ./dep1.ix", map[string]string{
				"./dep1.ix": joinLines(
					"includable-file",
					"import ./dep2.ix",
				),
				"./dep2.ix": joinLines(
					"includable-file",
					"const (a = (1 / 0))",
				),
			})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			includedChunk1 := mod.IncludedChunkForest[0]
			includedChunk2 := includedChunk1.IncludedChunkForest[0]

			importStmt1 := parse.FindNode(mod.MainChunk.Node, (*parse.InclusionImportStatement)(nil), nil)
			importStmt2 := parse.FindNode(includedChunk1.Node, (*parse.InclusionImportStatement)(nil), nil)
			binExpr := parse.FindNode(includedChunk2.Node, (*parse.BinaryExpression)(nil), nil)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Module = mod

			_, err = Eval(mod, state, true)

			var locatedError core.LocatedEvalError
			if !assert.ErrorAs(t, err, &locatedError) {
				return
			}

			assert.Equal(t, parse.SourcePositionStack{
				{
					//import
					SourceName:  mod.MainChunk.Source.Name(),
					StartLine:   2,
					StartColumn: 1,
					EndLine:     2,
					EndColumn:   17,
					Span:        importStmt1.Span,
				},
				{
					//import
					SourceName:  includedChunk1.Name(),
					StartLine:   2,
					StartColumn: 1,
					EndLine:     2,
					EndColumn:   17,
					Span:        importStmt2.Span,
				},
				{
					//binary expression
					SourceName:  includedChunk2.Name(),
					StartLine:   2,
					StartColumn: 12,
					EndLine:     2,
					EndColumn:   19,
					Span:        binExpr.Span,
				},
			}, locatedError.Location)
		})

		t.Run("in a function defined by an included file but called by the module", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName,
				joinLines(
					"manifest {}",
					"import ./dep.ix",
					"return f()",
				), map[string]string{
					"./dep.ix": joinLines(
						"includable-file",
						"fn f(){ return (1 / 0) }",
					),
				})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			includedChunk := mod.IncludedChunkForest[0]
			callExpr := parse.FindNode(mod.MainChunk.Node, (*parse.CallExpression)(nil), nil)
			fnDecl := parse.FindNode(includedChunk.Node, (*parse.FunctionDeclaration)(nil), nil)
			binExpr := parse.FindNode(includedChunk.Node, (*parse.BinaryExpression)(nil), nil)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Module = mod

			_, err = Eval(mod, state, true)

			var locatedError core.LocatedEvalError
			if !assert.ErrorAs(t, err, &locatedError) {
				return
			}

			assert.Equal(t, parse.SourcePositionStack{
				{
					//call
					SourceName:  mod.MainChunk.Source.Name(),
					StartLine:   3,
					StartColumn: 8,
					EndLine:     3,
					EndColumn:   11,
					Span:        callExpr.Span,
				},
				{
					//function declaration
					SourceName:  includedChunk.Name(),
					StartLine:   2,
					StartColumn: 1,
					EndLine:     2,
					EndColumn:   25,
					Span:        fnDecl.Span,
				},
				{
					//binary expression
					SourceName:  includedChunk.Name(),
					StartLine:   2,
					StartColumn: 16,
					EndLine:     2,
					EndColumn:   23,
					Span:        binExpr.Span,
				},
			}, locatedError.Location)
		})

		t.Run("in a function defined by an included file (deep) but called by the module", func(t *testing.T) {
			moduleName := "mymod.ix"
			modpath := writeModuleAndIncludedFiles(t, moduleName,
				joinLines(
					"manifest {}",
					"import ./dep1.ix",
					"return f()",
				), map[string]string{
					"./dep1.ix": joinLines(
						"includable-file",
						"import ./dep2.ix",
					),
					"./dep2.ix": joinLines(
						"includable-file",
						"fn f(){ return (1 / 0) }",
					),
				})

			mod, err := core.ParseLocalModule(modpath, core.ModuleParsingConfig{Context: createParsingContext(modpath)})
			if !assert.NoError(t, err) {
				return
			}

			includedChunk1 := mod.IncludedChunkForest[0]
			includedChunk2 := includedChunk1.IncludedChunkForest[0]

			callExpr := parse.FindNode(mod.MainChunk.Node, (*parse.CallExpression)(nil), nil)
			fnDecl := parse.FindNode(includedChunk2.Node, (*parse.FunctionDeclaration)(nil), nil)
			binExpr := parse.FindNode(includedChunk2.Node, (*parse.BinaryExpression)(nil), nil)

			state := core.NewGlobalState(NewDefaultTestContext())
			defer state.Ctx.CancelGracefully()
			state.Module = mod

			_, err = Eval(mod, state, true)

			var locatedError core.LocatedEvalError
			if !assert.ErrorAs(t, err, &locatedError) {
				return
			}

			assert.Equal(t, parse.SourcePositionStack{
				{
					//call
					SourceName:  mod.MainChunk.Source.Name(),
					StartLine:   3,
					StartColumn: 8,
					EndLine:     3,
					EndColumn:   11,
					Span:        callExpr.Span,
				},
				{
					//function declaration
					SourceName:  includedChunk2.Name(),
					StartLine:   2,
					StartColumn: 1,
					EndLine:     2,
					EndColumn:   25,
					Span:        fnDecl.Span,
				},
				{
					//binary expression
					SourceName:  includedChunk2.Name(),
					StartLine:   2,
					StartColumn: 16,
					EndLine:     2,
					EndColumn:   23,
					Span:        binExpr.Span,
				},
			}, locatedError.Location)
		})

		//TODO: add tests on shared functions.
	})

}

func TestGetQuantity(t *testing.T) {
	//TODO
}

type evalFn = func(chunkStringOrModule any, state *core.GlobalState, doSymbolicCheck bool) (core.Value, error)

func splitLines(ctx *core.Context, s core.String) (slice []core.String) {
	for _, e := range strings.Split(string(s), "\n") {
		slice = append(slice, core.String(e))
	}
	return
}

type reversibleEffect struct {
	applied bool
}

func (e *reversibleEffect) Resources() []core.ResourceName {
	return nil
}

func (e *reversibleEffect) PermissionKind() core.PermissionKind {
	return permkind.Create
}
func (e *reversibleEffect) Reversability(*core.Context) core.Reversability {
	return core.Reversible
}

func (e *reversibleEffect) IsApplied() bool {
	return e.applied
}

func (e *reversibleEffect) IsApplying() bool {
	return false
}

func (e *reversibleEffect) Apply(*core.Context) error {
	if e.applied {
		return core.ErrEffectAlreadyApplied
	}
	e.applied = true
	return nil
}

func (e *reversibleEffect) Reverse(*core.Context) error {
	return nil
}

type irreversibleEffect struct {
	applied bool
}

func (e *irreversibleEffect) Resources() []core.ResourceName {
	return nil
}

func (e *irreversibleEffect) PermissionKind() core.PermissionKind {
	return permkind.Create
}
func (e *irreversibleEffect) Reversability(*core.Context) core.Reversability {
	return core.Irreversible
}

func (e *irreversibleEffect) IsApplied() bool {
	return e.applied
}

func (e *irreversibleEffect) IsApplying() bool {
	return false
}

func (e *irreversibleEffect) Apply(*core.Context) error {
	if e.applied {
		return core.ErrEffectAlreadyApplied
	}
	e.applied = true
	return nil
}

func (e *irreversibleEffect) Reverse(*core.Context) error {
	return nil
}

func makeTreeWalkEvalFunc(t *testing.T) func(c any, s *core.GlobalState, doSymbolicCheck bool) (core.Value, error) {
	return _makeTreeWalkEvalFunc(t, false)
}

func makeRecylingTreeWalkEvalFunc(t *testing.T) func(c any, s *core.GlobalState, doSymbolicCheck bool) (core.Value, error) {
	return _makeTreeWalkEvalFunc(t, true)
}

func _makeTreeWalkEvalFunc(t *testing.T, recycle bool) func(c any, s *core.GlobalState, doSymbolicCheck bool) (core.Value, error) {

	var states []*core.TreeWalkState
	var lock sync.Mutex

	return func(c any, s *core.GlobalState, doSymbolicCheck bool) (core.Value, error) {
		var mod *core.Module

		switch val := c.(type) {
		case *core.Module:
			mod = val
			s.Module = mod
		case parse.SourceFile:
			chunk := utils.Must(parse.ParseChunkSource(val))

			mod = &core.Module{MainChunk: chunk}

			//if the test case provide a module we reuse the source
			if s.Module != nil {
				chunk.Source = s.Module.MainChunk.Source
				s.Module.MainChunk = chunk
				mod = s.Module
			} else {
				s.Module = mod
			}
		case string:
			chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
				NameString: "core-test",
				CodeString: val,
			}))

			mod = &core.Module{MainChunk: chunk}

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
			staticCheckData, err := core.StaticCheck(core.StaticCheckInput{
				State:             s,
				Node:              mod.MainChunk.Node,
				Module:            mod,
				Chunk:             mod.MainChunk,
				Globals:           s.Globals,
				Patterns:          s.Ctx.GetNamedPatterns(),
				PatternNamespaces: s.Ctx.GetPatternNamespaces(),
			})
			if !assert.NoError(t, err) {
				return nil, err
			}

			s.StaticCheckData = staticCheckData

			globals := make(map[string]symbolic.ConcreteGlobalValue)
			s.Globals.Foreach(func(name string, v core.Value, isConstant bool) error {
				globals[name] = symbolic.ConcreteGlobalValue{Value: v, IsConstant: isConstant}
				return nil
			})

			symbCtx, err := s.Ctx.ToSymbolicValue()
			if !assert.NoError(t, err) {
				return nil, err
			}

			symbData, err := symbolic.EvalCheck(symbolic.EvalCheckInput{
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

		var treeWalkState *core.TreeWalkState
		if recycle {
			lock.Lock()

			//Find an unused state to recycle.
			for i := 0; i < len(states); i++ {
				if states[i].Global.Ctx.IsDone() {
					treeWalkState = states[i]
					treeWalkState.Reset(s)
					break
				}
			}

			//Create a new state if no unused state was found.
			if treeWalkState == nil {
				treeWalkState = core.NewTreeWalkStateWithGlobal(s)
				states = append(states, treeWalkState)
			}
			lock.Unlock()
		} else {
			treeWalkState = core.NewTreeWalkStateWithGlobal(s)
		}

		return core.TreeWalkEval(mod.MainChunk.Node, treeWalkState)
	}
}

type TestProject struct {
	ID     core.ProjectID
	Img    core.Image
	Config testProjectConfig
}

func (p *TestProject) Id() core.ProjectID {
	return p.ID
}

func (p *TestProject) GetSecrets(ctx *core.Context) ([]core.ProjectSecret, error) {
	return nil, nil
}

func (p *TestProject) ListSecrets(ctx *core.Context) ([]core.ProjectSecretInfo, error) {
	return nil, nil
}

func (p *TestProject) BaseImage() (core.Image, error) {
	return p.Img, nil
}

type testProjectConfig struct {
	areExposedWebServersAllowed bool
}

func (c testProjectConfig) AreExposedWebServersAllowed() bool {
	return c.areExposedWebServersAllowed
}

func (p *TestProject) Configuration() core.ProjectConfiguration {
	return p.Config
}

func (p *TestProject) DevDatabasesDirOnOsFs(*core.Context, string) (string, error) {
	panic("unimplemented")
}

func (p *TestProject) CanProvideS3Credentials(s3Provider string) (bool, error) {
	panic("unimplemented")
}

func (p *TestProject) GetS3CredentialsForBucket(ctx *core.Context, bucketName string, provider string) (accessKey string, secretKey string, s3Endpoint core.Host, _ error) {
	panic("unimplemented")
}

type testImage struct {
	snapshot  core.FilesystemSnapshot
	projectID core.ProjectID
}

func (img testImage) ProjectID() core.ProjectID {
	return img.projectID
}

func (img testImage) FilesystemSnapshot() core.FilesystemSnapshot {
	return img.snapshot
}

func (img *testImage) Zip(ctx *core.Context, w io.Writer) error {
	panic(core.ErrNotImplemented)
}

func toByte(ctx *core.Context, i core.Int) core.Byte {
	return core.Byte(i)
}

func isClientInsecureAndStateful(ctx *core.Context, host core.Host) bool {
	client := utils.Must(ctx.GetProtolClient(host.URLWithPath("/")))

	return client.MayPurposefullySkipAuthentication() && client.IsStateful()
}

func createParsingContext(modpath string) *core.Context {
	pathPattern := core.PathPattern(core.Path(modpath).DirPath() + "...")
	return core.NewContextWithEmptyState(core.ContextConfig{
		Permissions: []core.Permission{core.CreateFsReadPerm(pathPattern)},
		Filesystem:  newOsFilesystem(),
	}, nil)
}

func newOsFilesystem() afs.Filesystem {
	fs := polyfill.New(osfs.Default)

	return afs.AddAbsoluteFeature(fs, func(path string) (string, error) {
		return filepath.Abs(path)
	})
}

// writeModuleAndIncludedFiles write a module & it's included files in a temporary directory on the OS filesystem.
func writeModuleAndIncludedFiles(t *testing.T, mod string, modContent string, dependencies map[string]string) string {
	dir := t.TempDir()
	modPath := filepath.Join(dir, mod)

	assert.NoError(t, fsutils.WriteFileSync(modPath, []byte(modContent), 0o400))

	for name, content := range dependencies {
		assert.NoError(t, fsutils.WriteFileSync(filepath.Join(dir, name), []byte(content), 0o400))
	}

	return modPath
}

func newMemFilesystem() afs.Filesystem {
	fs := memfs.New()

	return afs.AddAbsoluteFeature(fs, func(path string) (string, error) {
		if path[0] == '/' {
			return path, nil
		}
		return "", core.ErrNotImplemented
	})
}

func newMemFilesystemRootWD() afs.Filesystem {
	fs := memfs.New()

	return afs.AddAbsoluteFeature(fs, func(path string) (string, error) {
		if path[0] == '/' {
			return path, nil
		}
		if len(path) > 1 && path[0] == '.' && path[1] == '/' {
			return path[1:], nil
		}
		return "", core.ErrNotImplemented
	})
}

func newSnapshotableMemFilesystem() *snapshotableMemFilesystem {
	return &snapshotableMemFilesystem{memfs.New()}
}

var _ = afs.Filesystem((*snapshotableMemFilesystem)(nil))
var _ = core.SnapshotableFilesystem((*snapshotableMemFilesystem)(nil))

func copyMemFs(fls afs.Filesystem) afs.Filesystem {
	newMemFs := newMemFilesystem()
	err := util.Walk(fls, "/", func(path string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return newMemFs.MkdirAll(path, info.Mode().Perm())
		} else {
			content, err := util.ReadFile(fls, path)
			if err != nil {
				return err
			}
			return util.WriteFile(newMemFs, path, content, info.Mode().Perm())
		}
	})
	if err != nil {
		panic(err)
	}
	return newMemFs
}

type snapshotableMemFilesystem struct {
	billy.Filesystem
}

func (*snapshotableMemFilesystem) Absolute(path string) (string, error) {
	if path[0] == '/' {
		return path, nil
	}
	return "", core.ErrNotImplemented
}

func (fls *snapshotableMemFilesystem) TakeFilesystemSnapshot(config core.FilesystemSnapshotConfig) (core.FilesystemSnapshot, error) {
	return &memFilesystemSnapshot{
		fls: copyMemFs(fls),
	}, nil
}

var _ = core.FilesystemSnapshot((*memFilesystemSnapshot)(nil))

// memFilesystemSnapshot is partial implementation of core.FilesystemSnapshot,
// it only implements NewAdaptedFilesystem by returning a deep copy of fls.
type memFilesystemSnapshot struct {
	fls afs.Filesystem
}

func (s *memFilesystemSnapshot) NewAdaptedFilesystem(maxTotalStorageSizeHint core.ByteCount) (core.SnapshotableFilesystem, error) {
	return &snapshotableMemFilesystem{copyMemFs(s.fls)}, nil
}

func (s *memFilesystemSnapshot) WriteTo(fls afs.Filesystem, params core.SnapshotWriteToFilesystem) error {
	panic("unimplemented")
}

func (*memFilesystemSnapshot) Content(path string) (core.AddressableContent, error) {
	panic("unimplemented")
}

func (*memFilesystemSnapshot) ForEachEntry(func(m core.EntrySnapshotMetadata) error) error {
	panic("unimplemented")
}

func (*memFilesystemSnapshot) IsStoredLocally() bool {
	panic("unimplemented")
}

func (*memFilesystemSnapshot) Metadata(path string) (core.EntrySnapshotMetadata, error) {
	panic("unimplemented")
}

func (*memFilesystemSnapshot) RootDirEntries() []string {
	panic("unimplemented")
}
