package core

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestCompileModule(t *testing.T) {
	t.Parallel()

	joinLines := func(lines ...string) string {
		return strings.Join(lines, "\n")
	}

	t.Run("literals", func(t *testing.T) {
		expectBytecode(t, `1`,
			0,
			instrs(
				MakeInstruction(OpPushConstant, 0),
				MakeInstruction(OpSuspendVM),
			),
			[]Value{Int(1)},
		)

		expectBytecode(t, `"a"`,
			0,
			instrs(
				MakeInstruction(OpPushConstant, 0),
				MakeInstruction(OpSuspendVM),
			),
			[]Value{String("a")},
		)

		expectBytecode(t, `2020y-5mt-3d-0h-UTC`,
			0,
			instrs(
				MakeInstruction(OpPushConstant, 0),
				MakeInstruction(OpSuspendVM),
			),
			[]Value{DateTime(time.Date(2020, 5, 3, 0, 0, 0, 0, time.UTC))},
		)

		expectBytecode(t, `1s`,
			0,
			instrs(
				MakeInstruction(OpPushConstant, 0),
				MakeInstruction(OpSuspendVM),
			),
			[]Value{Duration(time.Second)},
		)

		expectBytecode(t, `1x/s`,
			0,
			instrs(
				MakeInstruction(OpPushConstant, 0),
				MakeInstruction(OpSuspendVM),
			),
			[]Value{Frequency(1)},
		)

	})

	t.Run("unary expressions", func(t *testing.T) {
		expectBytecode(t, `(- 2)`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpMinus),
				inst(OpSuspendVM),
			),
			[]Value{Int(2)},
		)
	})

	t.Run("binary expressions", func(t *testing.T) {
		expectBytecode(t, `(1 + 2)`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpNumBin),
				inst(OpSuspendVM),
			),
			[]Value{Int(1), Int(2)},
		)
	})

	t.Run("object literals", func(t *testing.T) {
		expectBytecode(t, `{}`,
			0,
			instrs(
				inst(OpCreateObject, 0, 0, 0),
				inst(OpSuspendVM),
			),
			[]Value{nil},
		)
		expectBytecode(t, `{a: 1}`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpCreateObject, 2, 0, 2),
				inst(OpSuspendVM),
			),
			[]Value{String("a"), Int(1), nil},
		)
		expectBytecode(t, `{a: 1, b: 2}`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpPushConstant, 3),
				inst(OpCreateObject, 4, 0, 4),
				inst(OpSuspendVM),
			),
			[]Value{String("a"), Int(1), String("b"), Int(2), nil},
		)
		expectBytecode(t, `{1,2}`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpPushConstant, 3),
				inst(OpCreateObject, 4, 2, 4),
				inst(OpSuspendVM),
			),
			[]Value{
				String("0"),
				Int(1),
				String("1"),
				Int(2),
				nil,
			},
		)
	})

	t.Run("top level return", func(t *testing.T) {

		expectBytecode(t, `
				return 1
			`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpReturn, 1),
				inst(OpSuspendVM),
			),
			[]Value{
				Int(1),
			},
		)

		expectBytecode(t, `
				return
			`,
			0,
			instrs(
				inst(OpReturn, 0),
				inst(OpSuspendVM),
			),
			nil,
		)
	})

	t.Run("list literals", func(t *testing.T) {
		expectBytecode(t, `[]`,
			0,
			instrs(
				inst(OpCreateList),
				inst(OpSuspendVM),
			),
			nil,
		)
		expectBytecode(t, `[1]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateList, 1),
				inst(OpSuspendVM),
			),
			[]Value{Int(1)},
		)
		expectBytecode(t, `[1,2]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpCreateList, 2),
				inst(OpSuspendVM),
			),
			[]Value{Int(1), Int(2)},
		)
		expectBytecode(t, `[...[1,2]]`,
			0,
			instrs(
				inst(OpCreateList, 0),
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpCreateList, 2),
				inst(OpSpreadList),
				inst(OpSuspendVM),
			),
			[]Value{Int(1), Int(2)},
		)
		expectBytecode(t, `[0, ...[1,2]]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateList, 1),
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpCreateList, 2),
				inst(OpSpreadList),
				inst(OpSuspendVM),
			),
			[]Value{Int(0), Int(1), Int(2)},
		)
		expectBytecode(t, `[0, ...[1,2], 3]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateList, 1),
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpCreateList, 2),
				inst(OpSpreadList),
				inst(OpPushConstant, 3),
				inst(OpAppend, 1),
				inst(OpSuspendVM),
			),
			[]Value{Int(0), Int(1), Int(2), Int(3)},
		)
		expectBytecode(t, `[0, ...[1,2], ...[3]]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateList, 1),
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpCreateList, 2),
				inst(OpSpreadList),
				inst(OpPushConstant, 3),
				inst(OpCreateList, 1),
				inst(OpSpreadList),
				inst(OpSuspendVM),
			),
			[]Value{Int(0), Int(1), Int(2), Int(3)},
		)
		expectBytecode(t, `[0, ...[1,2], ...[3], 4]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateList, 1),
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpCreateList, 2),
				inst(OpSpreadList),
				inst(OpPushConstant, 3),
				inst(OpCreateList, 1),
				inst(OpSpreadList),
				inst(OpPushConstant, 4),
				inst(OpAppend, 1),
				inst(OpSuspendVM),
			),
			[]Value{Int(0), Int(1), Int(2), Int(3), Int(4)},
		)
		expectBytecode(t, `[0, ...[1,2], 3, ...[4]]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateList, 1),
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpCreateList, 2),
				inst(OpSpreadList),
				inst(OpPushConstant, 3),
				inst(OpAppend, 1),
				inst(OpPushConstant, 4),
				inst(OpCreateList, 1),
				inst(OpSpreadList),
				inst(OpSuspendVM),
			),
			[]Value{Int(0), Int(1), Int(2), Int(3), Int(4)},
		)
	})

	t.Run("key list literals", func(t *testing.T) {
		expectBytecode(t, `.{}`,
			0,
			instrs(
				inst(OpCreateKeyList, 0),
				inst(OpSuspendVM),
			),
			nil,
		)
		expectBytecode(t, `.{a}`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateKeyList, 1),
				inst(OpSuspendVM),
			),
			[]Value{Identifier("a")},
		)
		expectBytecode(t, `.{a,b}`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpCreateKeyList, 2),
				inst(OpSuspendVM),
			),
			[]Value{Identifier("a"), Identifier("b")},
		)
	})

	t.Run("tuple literals", func(t *testing.T) {
		expectBytecode(t, `#[]`,
			0,
			instrs(
				inst(OpCreateTuple),
				inst(OpSuspendVM),
			),
			nil,
		)
		expectBytecode(t, `#[1]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateTuple, 1),
				inst(OpSuspendVM),
			),
			[]Value{Int(1)},
		)
		expectBytecode(t, `#[1,2]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpCreateTuple, 2),
				inst(OpSuspendVM),
			),
			[]Value{Int(1), Int(2)},
		)
		expectBytecode(t, `#[...#[1,2]]`,
			0,
			instrs(
				inst(OpCreateTuple, 0),
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpCreateTuple, 2),
				inst(OpSpreadTuple),
				inst(OpSuspendVM),
			),
			[]Value{Int(1), Int(2)},
		)
		expectBytecode(t, `#[0, ...#[1,2]]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateTuple, 1),
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpCreateTuple, 2),
				inst(OpSpreadTuple),
				inst(OpSuspendVM),
			),
			[]Value{Int(0), Int(1), Int(2)},
		)
		expectBytecode(t, `#[0, ...#[1,2], 3]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateTuple, 1),
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpCreateTuple, 2),
				inst(OpSpreadTuple),
				inst(OpPushConstant, 3),
				inst(OpAppend, 1),
				inst(OpSuspendVM),
			),
			[]Value{Int(0), Int(1), Int(2), Int(3)},
		)
		expectBytecode(t, `#[0, ...#[1,2], ...#[3]]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateTuple, 1),
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpCreateTuple, 2),
				inst(OpSpreadTuple),
				inst(OpPushConstant, 3),
				inst(OpCreateTuple, 1),
				inst(OpSpreadTuple),
				inst(OpSuspendVM),
			),
			[]Value{Int(0), Int(1), Int(2), Int(3)},
		)
		expectBytecode(t, `#[0, ...#[1,2], ...#[3], 4]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateTuple, 1),
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpCreateTuple, 2),
				inst(OpSpreadTuple),
				inst(OpPushConstant, 3),
				inst(OpCreateTuple, 1),
				inst(OpSpreadTuple),
				inst(OpPushConstant, 4),
				inst(OpAppend, 1),
				inst(OpSuspendVM),
			),
			[]Value{Int(0), Int(1), Int(2), Int(3), Int(4)},
		)
		expectBytecode(t, `#[0, ...#[1,2], 3, ...#[4]]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateTuple, 1),
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpCreateTuple, 2),
				inst(OpSpreadTuple),
				inst(OpPushConstant, 3),
				inst(OpAppend, 1),
				inst(OpPushConstant, 4),
				inst(OpCreateTuple, 1),
				inst(OpSpreadTuple),
				inst(OpSuspendVM),
			),
			[]Value{Int(0), Int(1), Int(2), Int(3), Int(4)},
		)
	})

	t.Run("dictionary literals", func(t *testing.T) {
		expectBytecode(t, `:{}`,
			0,
			instrs(
				inst(OpCreateDict, 0),
				inst(OpSuspendVM),
			),
			nil,
		)
		expectBytecode(t, `:{./a:1}`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpCreateDict, 2),
				inst(OpSuspendVM),
			),
			[]Value{
				Path("./a"),
				Int(1),
			},
		)
		expectBytecode(t, `:{./a:1,./b:2}`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpPushConstant, 3),
				inst(OpCreateDict, 4),
				inst(OpSuspendVM),
			),
			[]Value{
				Path("./a"),
				Int(1),
				Path("./b"),
				Int(2),
			},
		)
	})

	t.Run("constant declaration", func(t *testing.T) {

		expectBytecode(t, `const(A = 1)`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpSetGlobal, 1),
				inst(OpPushNil),
				inst(OpSuspendVM),
			),
			[]Value{Int(1), String("A")},
		)

	})

	t.Run("variable assignment at top level", func(t *testing.T) {
		expectBytecode(t, `$$A = 1`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpSetGlobal, 1),
				inst(OpSuspendVM),
			),
			[]Value{Int(1), String("A")},
		)

		expectBytecode(t, `a = 1`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpSetLocal, 0),
				inst(OpSuspendVM),
			),
			[]Value{Int(1)},
		)

	})

	t.Run("assignment of member expression", func(t *testing.T) {

		expectBytecode(t, `
				a = {}
				a.b = 1
			`,
			1,
			instrs(
				inst(OpCreateObject, 0, 0, 0),
				inst(OpSetLocal, 0),
				//
				inst(OpGetLocal, 0),
				inst(OpPushConstant, 1),
				inst(OpSetMember, 2),
				inst(OpSuspendVM),
			),
			[]Value{
				nil,
				Int(1),
				String("b"),
			},
		)

	})

	t.Run("assignment of identifier member expression", func(t *testing.T) {

		expectBytecode(t, `
			a = {}
			a.b = 1
		`,
			1,
			instrs(
				inst(OpCreateObject, 0, 0, 0),
				inst(OpSetLocal, 0),
				//
				inst(OpGetLocal, 0),
				inst(OpPushConstant, 1),
				inst(OpSetMember, 2),
				inst(OpSuspendVM),
			),
			[]Value{
				nil,
				Int(1),
				String("b"),
			},
		)

		expectBytecode(t, `
				a = {b: {}}
				a.b.c = 1
			`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateObject, 0, 0, 1),
				inst(OpCreateObject, 2, 0, 2),
				inst(OpSetLocal, 0),
				//
				inst(OpGetLocal, 0),
				inst(OpMemb, 3),
				inst(OpPushConstant, 4),
				inst(OpSetMember, 5),
				inst(OpSuspendVM),
			),
			[]Value{
				String("b"),
				nil,
				nil,
				String("b"),
				Int(1),
				String("c"),
			},
		)

	})

	t.Run("assignment at index", func(t *testing.T) {

		expectBytecode(t, `
			a = [1]
			a[0] = 2
		`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateList, 1),
				inst(OpSetLocal, 0),
				//
				inst(OpGetLocal, 0),
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpSetIndex),
				inst(OpSuspendVM),
			),
			[]Value{
				Int(1),
				Int(0),
				Int(2),
			},
		)

		expectBytecode(t, `
				a = {b: {}}
				a.b.c = 1
			`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateObject, 0, 0, 1),
				inst(OpCreateObject, 2, 0, 2),
				inst(OpSetLocal, 0),
				//
				inst(OpGetLocal, 0),
				inst(OpMemb, 3),
				inst(OpPushConstant, 4),
				inst(OpSetMember, 5),
				inst(OpSuspendVM),
			),
			[]Value{
				String("b"),
				nil,
				nil,
				String("b"),
				Int(1),
				String("c"),
			},
		)

	})

	t.Run("multi assignment", func(t *testing.T) {
		expectBytecode(t, `
				l = [1, 2]
				assign a b = l
			`,
			3,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpCreateList, 2),
				inst(OpSetLocal, 0),
				//
				inst(OpGetLocal, 0),
				inst(OpCopyTop),
				inst(OpPushConstant, 2),
				inst(OpAt),
				inst(OpSetLocal, 1),
				inst(OpPushConstant, 3),
				inst(OpAt),
				inst(OpSetLocal, 2),
				inst(OpSuspendVM),
			),
			[]Value{Int(1), Int(2), Int(0), Int(1)},
		)

	})

	t.Run("function declaration", func(t *testing.T) {
		expectBytecode(t, `fn f(){}`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpSetGlobal, 1),
				inst(OpSuspendVM),
			),
			[]Value{
				&InoxFunction{
					compiledFunction: &CompiledFunction{
						ParamCount:     0,
						IsVariadic:     false,
						LocalCount:     0,
						Instructions:   instrs(inst(OpReturn, 0)),
						SourceNodeSpan: parse.NodeSpan{Start: 0, End: 8},
					},
				},
				String("f"),
			},
		)

		expectBytecode(t, `fn f(){ return 1 }`,
			0,
			instrs(
				inst(OpPushConstant, 1),
				inst(OpSetGlobal, 2),
				inst(OpSuspendVM),
			),
			[]Value{
				Int(1),
				&InoxFunction{compiledFunction: &CompiledFunction{
					ParamCount: 0,
					IsVariadic: false,
					LocalCount: 0,
					Instructions: instrs(
						inst(OpPushConstant, 0),
						inst(OpReturn, 1),
					),
					SourceNodeSpan: parse.NodeSpan{Start: 0, End: 18},
				}},
				String("f"),
			},
		)
	})

	t.Run("function expression", func(t *testing.T) {
		expectBytecode(t, `f = fn(){}`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpSetLocal, 0),
				inst(OpSuspendVM),
			),
			[]Value{
				&InoxFunction{
					compiledFunction: &CompiledFunction{
						ParamCount:     0,
						IsVariadic:     false,
						LocalCount:     0,
						Instructions:   instrs(inst(OpReturn, 0)),
						SourceNodeSpan: parse.NodeSpan{Start: 4, End: 10},
					},
				},
			},
		)
		expectBytecode(t, `f = fn()=>1`,
			1,
			instrs(
				inst(OpPushConstant, 1),
				inst(OpSetLocal, 0),
				inst(OpSuspendVM),
			),
			[]Value{
				Int(1),
				&InoxFunction{
					compiledFunction: &CompiledFunction{
						ParamCount: 0,
						IsVariadic: false,
						LocalCount: 0,
						Instructions: instrs(
							inst(OpPushConstant, 0),
							inst(OpReturn, 1),
						),
						SourceNodeSpan: parse.NodeSpan{Start: 4, End: 11},
					},
				},
			},
		)
		expectBytecode(t, `f = fn(x){}`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpSetLocal, 0),
				inst(OpSuspendVM),
			),
			[]Value{
				&InoxFunction{
					compiledFunction: &CompiledFunction{
						ParamCount:     1,
						IsVariadic:     false,
						LocalCount:     1,
						Instructions:   instrs(inst(OpReturn, 0)),
						SourceNodeSpan: parse.NodeSpan{Start: 4, End: 11},
					},
				},
			},
		)

		expectBytecode(t,
			joinLines(
				"f = fn(x){}",
				"g = fn(x,y){}",
			),
			2,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpSetLocal, 0),
				inst(OpPushConstant, 1),
				inst(OpSetLocal, 1),
				inst(OpSuspendVM),
			),
			[]Value{
				&InoxFunction{
					compiledFunction: &CompiledFunction{
						ParamCount:     1,
						IsVariadic:     false,
						LocalCount:     1,
						Instructions:   instrs(inst(OpReturn, 0)),
						SourceNodeSpan: parse.NodeSpan{Start: 4, End: 11},
					},
				},
				&InoxFunction{
					compiledFunction: &CompiledFunction{
						ParamCount:     2,
						IsVariadic:     false,
						LocalCount:     2,
						Instructions:   instrs(inst(OpReturn, 0)),
						SourceNodeSpan: parse.NodeSpan{Start: 16, End: 25},
					},
				},
			},
		)

		expectBytecode(t, `f = fn(){ return 1 }`,
			1,
			instrs(
				inst(OpPushConstant, 1),
				inst(OpSetLocal, 0),
				inst(OpSuspendVM),
			),
			[]Value{
				Int(1),
				&InoxFunction{compiledFunction: &CompiledFunction{
					ParamCount: 0,
					IsVariadic: false,
					LocalCount: 0,
					Instructions: instrs(
						inst(OpPushConstant, 0),
						inst(OpReturn, 1),
					),
					SourceNodeSpan: parse.NodeSpan{Start: 4, End: 20},
				}},
			},
		)

		expectBytecode(t, `fn f(){ f = fn(){ return 1 } }`,
			0,
			instrs(
				inst(OpPushConstant, 2),
				inst(OpSetGlobal, 3),
				inst(OpSuspendVM),
			),
			[]Value{
				Int(1),
				&InoxFunction{
					compiledFunction: &CompiledFunction{
						ParamCount: 0,
						IsVariadic: false,
						LocalCount: 0,
						Instructions: instrs(
							inst(OpPushConstant, 0),
							inst(OpReturn, 1),
						),
						SourceNodeSpan: parse.NodeSpan{Start: 12, End: 28},
					},
				},
				&InoxFunction{
					compiledFunction: &CompiledFunction{
						ParamCount: 0,
						IsVariadic: false,
						LocalCount: 1,
						Instructions: instrs(
							inst(OpPushConstant, 1),
							inst(OpSetLocal, 0),
							inst(OpReturn, 0),
						),
						SourceNodeSpan: parse.NodeSpan{Start: 0, End: 30},
					},
				},
				String("f"),
			},
		)
	})

	t.Run("call declared function", func(t *testing.T) {
		expectBytecode(t,
			joinLines(
				"fn f(){}",
				"return f()",
			),
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpSetGlobal, 1),
				inst(OpPushNil),
				inst(OpPushNil),
				inst(OpGetGlobal, 2),
				inst(OpCall, 0, 0, 0),
				inst(OpReturn, 1),
				inst(OpSuspendVM),
			),
			[]Value{
				&InoxFunction{
					compiledFunction: &CompiledFunction{
						ParamCount:     0,
						IsVariadic:     false,
						LocalCount:     0,
						Instructions:   instrs(inst(OpReturn, 0)),
						SourceNodeSpan: parse.NodeSpan{Start: 0, End: 8},
					},
				},
				String("f"),
				String("f"),
			},
		)

	})

	t.Run("spawn", func(t *testing.T) {
		expectBytecode(t, joinLines(
			"fn f(){",
			"    return 1",
			"}",
			"go do f()",
		),
			0,
			instrs(
				inst(OpPushConstant, 1),
				inst(OpSetGlobal, 2),
				//
				inst(OpPushNil),
				inst(OpGetGlobal, 3),
				inst(OpSpawnLThread, 1, 4, 5, 6),
				inst(OpPop),
				inst(OpSuspendVM),
			),
			[]Value{
				Int(1),
				&InoxFunction{
					compiledFunction: &CompiledFunction{
						ParamCount: 0,
						IsVariadic: false,
						LocalCount: 0,
						Instructions: instrs(
							inst(OpPushConstant, 0),
							inst(OpReturn, 1),
						),
						SourceNodeSpan: parse.NodeSpan{Start: 0, End: 22},
					},
				},
				String("f"),
				String("f"),
				String("f"),
				nil,
				nil,
			},
			bytecodeConstantAssertionData{
				localCount: 0,
				input:      "",
				expectedInstructions: instrs(
					inst(OpPushNil),
					inst(OpPushNil),
					inst(OpGetGlobal, 0),
					inst(OpCall, 0, 0, 0),
					inst(OpSuspendVM),
				),
				expectedConstants: []Value{
					String("f"),
				},
			},
		)

	})

	t.Run("permission drop", func(t *testing.T) {
		expectBytecode(t, `
			drop-perms {
				read: {
					globals: "*"
				}
			}
		`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpCreateObject, 2, 0, 3),
				inst(OpCreateObject, 2, 0, 4),
				inst(OpDropPerms),
				inst(OpSuspendVM),
			),
			[]Value{String("read"), String("globals"), String("*"), nil, nil},
		)

	})

	t.Run("index expression", func(t *testing.T) {
		expectBytecode(t, `
				l = []
				l[1]
			`,
			1,
			instrs(
				//create list
				inst(OpCreateList, 0),
				inst(OpSetLocal, 0),
				//index
				inst(OpGetLocal, 0),
				inst(OpPushConstant, 0),
				inst(OpAt),
				inst(OpPop),
				inst(OpSuspendVM),
			),
			[]Value{Int(1)},
		)
	})

	t.Run("slice expression", func(t *testing.T) {
		expectBytecode(t, `
				l = []
				l[0:1]
			`,
			1,
			instrs(
				//create list
				inst(OpCreateList, 0),
				inst(OpSetLocal, 0),
				//index
				inst(OpGetLocal, 0),
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpSlice),
				inst(OpPop),
				inst(OpSuspendVM),
			),
			[]Value{Int(0), Int(1)},
		)

		expectBytecode(t, `
				l = []
				l[0:]
			`,
			1,
			instrs(
				//create list
				inst(OpCreateList, 0),
				inst(OpSetLocal, 0),
				//index
				inst(OpGetLocal, 0),
				inst(OpPushConstant, 0),
				inst(OpPushNil),
				inst(OpSlice),
				inst(OpPop),
				inst(OpSuspendVM),
			),
			[]Value{Int(0)},
		)

		expectBytecode(t, `
				l = []
				l[:1]
			`,
			1,
			instrs(
				//create list
				inst(OpCreateList, 0),
				inst(OpSetLocal, 0),
				//index
				inst(OpGetLocal, 0),
				inst(OpPushNil),
				inst(OpPushConstant, 0),
				inst(OpSlice),
				inst(OpPop),
				inst(OpSuspendVM),
			),
			[]Value{Int(1)},
		)
	})

	t.Run("if statement", func(t *testing.T) {
		expectBytecode(t, `
				a = 0
				if true {
					a = 1
				}
			`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpSetLocal, 0),
				//if statement
				//
				inst(OpPushTrue),
				inst(OpJumpIfFalse, 14),
				//consequent
				inst(OpPushConstant, 1),
				inst(OpSetLocal, 0),
				//
				inst(OpSuspendVM),
			),
			[]Value{Int(0), Int(1)},
		)
		expectBytecode(t, `
				a = 0
				if true {
					a = 1
				} else {
					a = 2
				}
			`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpSetLocal, 0),
				//if statement
				//
				inst(OpPushTrue),
				inst(OpJumpIfFalse, 17),
				//consequent
				inst(OpPushConstant, 1),
				inst(OpSetLocal, 0),
				inst(OpJump, 22),
				//alternate
				inst(OpPushConstant, 2),
				inst(OpSetLocal, 0),
				//
				inst(OpSuspendVM),
			),
			[]Value{Int(0), Int(1), Int(2)},
		)
	})

	t.Run("for-in statement", func(t *testing.T) {
		t.Run("no nesting", func(t *testing.T) {
			expectBytecode(t, `
					l = []
					a = 0
					for e in l {
						a = e
					}
				`,
				2+3,
				instrs(
					inst(OpCreateList, 0),
					inst(OpSetLocal, 0),
					inst(OpPushConstant, 0),
					inst(OpSetLocal, 1),
					//for statement
					//
					inst(OpGetLocal, 0),
					inst(OpIterInit, 0),
					inst(OpSetLocal, 2),
					//
					inst(OpGetLocal, 2),
					inst(OpIterNext, 3),
					//
					inst(OpJumpIfFalse, 36),
					inst(OpGetLocal, 2),
					inst(OpIterValue, 3),
					inst(OpSetLocal, 4),

					//body
					inst(OpGetLocal, 4),
					inst(OpSetLocal, 1),
					//
					inst(OpJump, 16),
					inst(OpSuspendVM),
				),
				[]Value{Int(0)},
			)

			expectBytecode(t, `
					l = []
					for e in l {
						if true {
							continue
						}
						break
					}
				`,
				1+3,
				instrs(
					inst(OpCreateList, 0),
					inst(OpSetLocal, 0),
					//for statement
					//
					inst(OpGetLocal, 0),
					inst(OpIterInit, 0),
					inst(OpSetLocal, 1),
					//
					inst(OpGetLocal, 1),
					inst(OpIterNext, 2),
					//
					inst(OpJumpIfFalse, 37),
					inst(OpGetLocal, 1),
					inst(OpIterValue, 2),
					inst(OpSetLocal, 3),

					//if
					inst(OpPushTrue),
					inst(OpJumpIfFalse, 31),
					//consequent
					inst(OpJump, 34), //continue
					inst(OpJump, 37), //break
					//
					inst(OpJump, 11),
					inst(OpSuspendVM),
				),
				nil,
			)
		})

		t.Run("nesting", func(t *testing.T) {
			expectBytecode(t, `
					l = []
					a = 0
					for e in l {
						for v in l {
							a = 1
						}
						a = 2
					}
				`,
				2+6,
				instrs(
					inst(OpCreateList, 0),
					inst(OpSetLocal, 0),
					inst(OpPushConstant, 0),
					inst(OpSetLocal, 1),
					//for statement
					//
					inst(OpGetLocal, 0),
					inst(OpIterInit, 0),
					inst(OpSetLocal, 2),
					//
					inst(OpGetLocal, 2),
					inst(OpIterNext, 3),
					//
					inst(OpJumpIfFalse, 64),
					inst(OpGetLocal, 2),
					inst(OpIterValue, 3),
					inst(OpSetLocal, 4),

					//body
					instrs(
						inst(OpGetLocal, 0),
						inst(OpIterInit, 0),
						inst(OpSetLocal, 5),
						//
						inst(OpGetLocal, 5),
						inst(OpIterNext, 6),
						//
						inst(OpJumpIfFalse, 56),
						inst(OpGetLocal, 5),
						inst(OpIterValue, 6),
						inst(OpSetLocal, 7),
						//body
						inst(OpPushConstant, 1),
						inst(OpSetLocal, 1),
						//
						inst(OpJump, 35),
					),
					inst(OpPushConstant, 2),
					inst(OpSetLocal, 1),
					//
					inst(OpJump, 16),
					inst(OpSuspendVM),
				),
				[]Value{Int(0), Int(1), Int(2)},
			)

			expectBytecode(t, `
					l = []
					for e in l {
						if true {
							for v in l {
								if true {
									continue
								}
								break
							}
							continue
						}
						break
					}
				`,
				1+6,
				instrs(
					inst(OpCreateList, 0),
					inst(OpSetLocal, 0),
					//for statement
					//
					inst(OpGetLocal, 0),
					inst(OpIterInit, 0),
					inst(OpSetLocal, 1),
					//
					inst(OpGetLocal, 1),
					inst(OpIterNext, 2),
					//
					inst(OpJumpIfFalse, 69),
					inst(OpGetLocal, 1),
					inst(OpIterValue, 2),
					inst(OpSetLocal, 3),

					//body
					instrs(
						inst(OpPushTrue),
						inst(OpJumpIfFalse, 63),
						//for statement
						inst(OpGetLocal, 0),
						inst(OpIterInit, 0),
						inst(OpSetLocal, 4),
						//
						inst(OpGetLocal, 4),
						inst(OpIterNext, 5),
						//
						inst(OpJumpIfFalse, 60),
						inst(OpGetLocal, 4),
						inst(OpIterValue, 5),
						inst(OpSetLocal, 6),

						//if
						inst(OpPushTrue),
						inst(OpJumpIfFalse, 54),
						//consequent
						inst(OpJump, 57), //continue
						inst(OpJump, 60), //break
						//
						inst(OpJump, 34),
						//end of for statement
						inst(OpJump, 66), //break (end of outer if statement)
						inst(OpJump, 69), //continue
					),
					//
					inst(OpJump, 11),
					inst(OpSuspendVM),
				),
				nil,
			)

			expectBytecode(t, `
					l = []
					for e in l {
						if true {
							walk ./ entry {
								if true {
									continue
								}
								break
							}
							continue
						}
						break
					}
				`,
				1+5,
				instrs(
					inst(OpCreateList, 0),
					inst(OpSetLocal, 0),
					//for statement
					//
					inst(OpGetLocal, 0),
					inst(OpIterInit, 0),
					inst(OpSetLocal, 1),
					//
					inst(OpGetLocal, 1),
					inst(OpIterNext, 2),
					//
					inst(OpJumpIfFalse, 69),
					inst(OpGetLocal, 1),
					inst(OpIterValue, 2),
					inst(OpSetLocal, 3),

					//body
					instrs(
						inst(OpPushTrue),
						inst(OpJumpIfFalse, 63),
						//walk statement
						inst(OpPushConstant, 0),
						inst(OpWalkerInit),
						inst(OpSetLocal, 4),
						//
						inst(OpGetLocal, 4),
						inst(OpIterNext, -1),
						//
						inst(OpJumpIfFalse, 60),
						inst(OpGetLocal, 4),
						inst(OpIterValue, -1),
						inst(OpSetLocal, 5),

						//if
						inst(OpPushTrue),
						inst(OpJumpIfFalse, 54),
						//consequent
						inst(OpJump, 57), //continue
						inst(OpJump, 60), //break
						//
						inst(OpJump, 34),
						//end of walk statement
						inst(OpJump, 66), //break (end of outer if statement)
						inst(OpJump, 69), //continue
					),
					//
					inst(OpJump, 11),
					inst(OpSuspendVM),
				),
				[]Value{Path("./")},
			)
		})

	})

	t.Run("switch statement", func(t *testing.T) {
		expectBytecode(t, `
				switch 1 {}
			`,
			0,
			instrs(
				inst(OpSuspendVM),
			),
			nil,
		)

		expectBytecode(t, `
				switch 1 {
					1 { a = 1 }
				}
			`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpEqual),
				inst(OpJumpIfFalse, 18),
				inst(OpPushConstant, 2),
				inst(OpSetLocal, 0),
				inst(OpJump, 18),
				inst(OpSuspendVM),
			),
			[]Value{Int(1), Int(1), Int(1)},
		)

		expectBytecode(t, `
				switch 1 {
					1 { a = 1 }
					2 { a = 2 }
				}
			`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				//first case
				inst(OpCopyTop),
				inst(OpPushConstant, 1),
				inst(OpEqual),
				inst(OpJumpIfFalse, 19),
				inst(OpPushConstant, 2),
				inst(OpSetLocal, 0),
				inst(OpJump, 34),
				//second case
				inst(OpPushConstant, 3),
				inst(OpEqual),
				inst(OpJumpIfFalse, 34),
				inst(OpPushConstant, 4),
				inst(OpSetLocal, 0),
				inst(OpJump, 34),
				inst(OpSuspendVM),
			),
			[]Value{Int(1), Int(1), Int(1), Int(2), Int(2)},
		)
	})

	t.Run("match statement", func(t *testing.T) {
		expectBytecode(t, `
				match 1 {}
			`,
			0,
			instrs(
				inst(OpSuspendVM),
			),
			nil,
		)

		expectBytecode(t, `
				match 1 {
					1 { a = 1 }
				}
			`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpMatch),
				inst(OpJumpIfFalse, 18),
				inst(OpPushConstant, 2),
				inst(OpSetLocal, 0),
				inst(OpJump, 18),
				inst(OpSuspendVM),
			),
			[]Value{Int(1), Int(1), Int(1)},
		)

		expectBytecode(t, `
				match 1 {
					1 { a = 1 }
					2 { a = 2 }
				}
			`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				//first case
				inst(OpCopyTop),
				inst(OpPushConstant, 1),
				inst(OpMatch),
				inst(OpJumpIfFalse, 19),
				inst(OpPushConstant, 2),
				inst(OpSetLocal, 0),
				inst(OpJump, 34),
				//second case
				inst(OpPushConstant, 3),
				inst(OpMatch),
				inst(OpJumpIfFalse, 34),
				inst(OpPushConstant, 4),
				inst(OpSetLocal, 0),
				inst(OpJump, 34),
				inst(OpSuspendVM),
			),
			[]Value{Int(1), Int(1), Int(1), Int(2), Int(2)},
		)
	})

	t.Run("path expression", func(t *testing.T) {

		expectBytecode(t, `
			username = "foo"
			/home/{username}
		`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpSetLocal, 0),
				//
				inst(OpPushConstant, 1),
				inst(OpGetLocal, 0),
				inst(OpCreatePath, 2, 2),
				inst(OpPop),
				inst(OpSuspendVM),
			),
			[]Value{
				String("foo"),
				String("/home/"),
				&List{underlyingList: &ValueList{elements: []Serializable{True, False}}},
			},
		)

	})

	t.Run("path pattern expression", func(t *testing.T) {

		expectBytecode(t, `
			username = "foo"
			%/home/{username}
		`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpSetLocal, 0),
				//
				inst(OpPushConstant, 1),
				inst(OpGetLocal, 0),
				inst(OpCreatePathPattern, 2, 2),
				inst(OpPop),
				inst(OpSuspendVM),
			),
			[]Value{
				String("foo"),
				String("/home/"),
				&List{underlyingList: &ValueList{elements: []Serializable{True, False}}},
			},
		)

	})

	t.Run("URL expression", func(t *testing.T) {

		expectBytecode(t, `
			username = "foo"
			https://example.com/home/{username}
		`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpSetLocal, 0),
				//
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpGetLocal, 0),
				inst(OpCreateURL, 3),
				inst(OpPop),
				inst(OpSuspendVM),
			),
			[]Value{
				String("foo"),
				Host("https://example.com"),
				String("/home/"),
				NewRecordFromMap(ValMap{
					"path-slice-count":   Int(2),
					"query-params":       &Tuple{},
					"static-path-slices": &Tuple{elements: []Serializable{True, False}},
				}),
			},
		)

		expectBytecode(t, `
			username = "foo"
			x = "0"
			https://example.com/home/{username}?x={x}
		`,
			2,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpSetLocal, 0),
				//
				inst(OpPushConstant, 1),
				inst(OpSetLocal, 1),
				//
				inst(OpPushConstant, 2),
				inst(OpPushConstant, 3),
				inst(OpGetLocal, 0),
				inst(OpPushConstant, 4),
				inst(OpGetLocal, 1),
				inst(OptStrQueryParamVal),
				inst(OpStrConcat),
				inst(OpCreateURL, 5),
				inst(OpPop),
				inst(OpSuspendVM),
			),
			[]Value{
				String("foo"),
				String("0"),
				Host("https://example.com"),
				String("/home/"),
				String(""),
				NewRecordFromMap(ValMap{
					"path-slice-count":   Int(2),
					"query-params":       &Tuple{elements: []Serializable{String("x"), Int(2)}},
					"static-path-slices": &Tuple{elements: []Serializable{True, False}},
				}),
			},
		)

		expectBytecode(t, `
			x = "0"
			https://example.com/home/?x={x}
		`,
			1,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpSetLocal, 0),
				//
				inst(OpPushConstant, 1),
				inst(OpPushConstant, 2),
				inst(OpPushConstant, 3),
				inst(OpGetLocal, 0),
				inst(OptStrQueryParamVal),
				inst(OpStrConcat),
				inst(OpCreateURL, 4),
				inst(OpPop),
				inst(OpSuspendVM),
			),
			[]Value{
				String("0"),
				Host("https://example.com"),
				String("/home/"),
				String(""),
				NewRecordFromMap(ValMap{
					"path-slice-count":   Int(1),
					"query-params":       &Tuple{elements: []Serializable{String("x"), Int(2)}},
					"static-path-slices": &Tuple{elements: []Serializable{True}},
				}),
			},
		)
	})

	t.Run("list pattern literals", func(t *testing.T) {
		expectBytecode(t, `%[]`,
			0,
			instrs(
				inst(OpCreateListPattern, 0, 0),
				inst(OpSuspendVM),
			),
			nil,
		)
		expectBytecode(t, `%[1]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpToPattern),
				inst(OpCreateListPattern, 1, 0),
				inst(OpSuspendVM),
			),
			[]Value{Int(1)},
		)
		expectBytecode(t, `%[1,2]`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpToPattern),
				inst(OpPushConstant, 1),
				inst(OpToPattern),
				//
				inst(OpCreateListPattern, 2, 0),
				inst(OpSuspendVM),
			),
			[]Value{Int(1), Int(2)},
		)
		expectBytecode(t, `%[]%int`,
			0,
			instrs(
				inst(OpResolvePattern, 0),
				inst(OpCreateListPattern, 0, 1),
				inst(OpSuspendVM),
			),
			[]Value{String("int")},
		)
	})

	t.Run("object pattern literals", func(t *testing.T) {
		expectBytecode(t, `%{}`,
			0,
			instrs(
				inst(OpCreateObjectPattern, 0, 1),
				inst(OpSuspendVM),
			),
			nil,
		)
		expectBytecode(t, `%{a: 1}`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpToPattern),
				inst(OpPushFalse),
				inst(OpCreateObjectPattern, 3, 1),
				inst(OpSuspendVM),
			),
			[]Value{String("a"), Int(1)},
		)
		expectBytecode(t, `%{a: 1, b: 2}`,
			0,
			instrs(
				//a: 1
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpToPattern),
				inst(OpPushFalse),
				//b: 2
				inst(OpPushConstant, 2),
				inst(OpPushConstant, 3),
				inst(OpToPattern),
				inst(OpPushFalse),
				//
				inst(OpCreateObjectPattern, 6, 1),
				inst(OpSuspendVM),
			),
			[]Value{String("a"), Int(1), String("b"), Int(2)},
		)
		// expectBytecode(t, `%{a: 1, ...}`,
		// 	0,
		// 	instrs(
		// 		inst(OpPushConstant, 0),
		// 		inst(OpPushConstant, 1),
		// 		inst(OpToPattern),
		// 		inst(OpPushFalse),
		// 		inst(OpCreateObjectPattern, 3, 1),
		// 		inst(OpSuspendVM),
		// 	),
		// 	[]Value{Str("a"), Int(1)},
		// )
		expectBytecode(t, `%{...%{}, a: 1}`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpPushConstant, 1),
				inst(OpToPattern),
				inst(OpPushFalse),
				inst(OpCreateObjectPattern, 3, 1),
				inst(OpCreateObjectPattern, 0, 1),
				inst(OpSpreadObjectPattern),
				inst(OpSuspendVM),
			),
			[]Value{String("a"), Int(1)},
		)
	})

	t.Run("pattern definitions", func(t *testing.T) {
		// expectBytecode(t, `%p = 1`,
		// 	0,
		// 	instrs(
		// 		inst(OpPushConstant, 0),
		// 		inst(OpToPattern),
		// 		inst(OpAddPattern, 1),
		// 		inst(OpSuspendVM),
		// 	),
		// 	[]Value{Int(1), Str("p")},
		// )

		expectBytecode(t, `pattern p = %str( "a" )`,
			0,
			instrs(
				inst(OpPushConstant, 0),
				inst(OpCreateSequenceStringPattern, 1, 1, 2),
				inst(OpToPattern),
				inst(OpAddPattern, 3),
				inst(OpSuspendVM),
			),
			[]Value{
				NewExactStringPattern("a"),
				KeyList{""},
				nil,
				String("p"),
			},
		)
	})

}

func inst(op Opcode, operands ...int) []byte {
	return MakeInstruction(op, operands...)
}

func instrs(instructions ...[]byte) []byte {
	var concat []byte
	for _, i := range instructions {
		concat = append(concat, i...)
	}
	return concat
}

type bytecodeConstantAssertionData struct {
	input                string
	localCount           int
	expectedInstructions []byte
	expectedConstants    []Value
}

func expectBytecode(t *testing.T, input string, localCount int, expectedInstructions []byte, expectedConstants []Value, bytecodeConstantAssertions ...bytecodeConstantAssertionData) {
	actual, trace, err := traceCompile(t, input, nil)

	defer func() {
		for _, tr := range trace {
			t.Log(tr)
		}
	}()

	if !assert.NoError(t, err) {
		return
	}

	t.Helper()
	_expectBytecode(t, actual, localCount, expectedInstructions, expectedConstants, bytecodeConstantAssertions...)

}

func _expectBytecode(t *testing.T, actualBytecode *Bytecode, localCount int, expectedInstructions []byte, expectedConstants []Value, bytecodeConstantAssertions ...bytecodeConstantAssertionData) {

	var bytecodeList []*Bytecode

	for i, c := range actualBytecode.constants {

		switch constant := c.(type) {
		case *InoxFunction:
			constant.Node = nil
			constant.Chunk = nil
			if constant.compiledFunction != nil {
				constant.compiledFunction.SourceMap = nil
				constant.compiledFunction.Bytecode = nil
			}
		case *Bytecode:
			bytecodeList = append(bytecodeList, constant)
			actualBytecode.constants[i] = nil
		case *Module:
			actualBytecode.constants[i] = nil
		case AstNode:
			actualBytecode.constants[i] = nil
		}

	}

	assert.Equal(t, localCount, actualBytecode.main.LocalCount)
	assert.Equal(t, expectedInstructions, actualBytecode.main.Instructions)
	assert.Equal(t, expectedConstants, actualBytecode.constants)

	if assert.Equal(t, len(bytecodeList), len(bytecodeConstantAssertions)) {
		for i, data := range bytecodeConstantAssertions {
			_expectBytecode(t, bytecodeList[i], data.localCount, data.expectedInstructions, data.expectedConstants)
		}
	}

}

// func expectCompileError(t *testing.T, input, expected string) {
// 	_, trace, err := traceCompile(t, input, nil)

// 	var ok bool
// 	defer func() {
// 		if !ok {
// 			for _, tr := range trace {
// 				t.Log(tr)
// 			}
// 		}
// 	}()

// 	assert.NoError(t, err)
// 	assert.True(t, strings.Contains(err.Error(), expected),
// 		"expected error string: %s, got: %s", expected, err.Error())
// 	ok = true
// }

type compileTracer struct {
	Out []string
}

func (o *compileTracer) Write(p []byte) (n int, err error) {
	o.Out = append(o.Out, string(p))
	return len(p), nil
}

func traceCompile(
	t *testing.T,
	input string,
	globals map[string]Value,
) (res *Bytecode, trace []string, err error) {

	chunk := utils.Must(parse.ParseChunkSource(parse.InMemorySource{
		NameString: "in-memory-test",
		CodeString: input,
	}))

	module := &Module{
		MainChunk: chunk,
	}

	tr := &compileTracer{}

	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)

	bytecode, err := Compile(CompilationInput{
		Mod:         module,
		Globals:     globals,
		TraceWriter: tr,
		Context:     ctx,
	})
	res = bytecode

	trace = append(trace, fmt.Sprintf("compiler trace:\n%s", strings.Join(tr.Out, "")))
	trace = append(trace, fmt.Sprintf("compiled constants:\n%s", strings.Join(res.FormatConstants(ctx, ""), "\n")))
	trace = append(trace, fmt.Sprintf("compiled instructions:\n%s\n", strings.Join(res.FormatInstructions(ctx, ""), "\n")))

	return
}
