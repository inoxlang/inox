package symbolic

import (
	"testing"

	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/sourcecode"
	utils "github.com/inoxlang/inox/internal/utils/common"
	"github.com/stretchr/testify/assert"
)

func TestSymbolicState(t *testing.T) {
	emptyChunk := utils.Must(parse.ParseChunkSource(sourcecode.InMemorySource{
		NameString: "",
		CodeString: "",
	}))

	t.Run("setLocal()", func(t *testing.T) {

		t.Run("no locals", func(t *testing.T) {
			ctx := NewSymbolicContext(nil, nil, nil)
			state := newSymbolicState(ctx, emptyChunk)

			assert.Panics(t, func() {
				state.setLocal("local", &Identifier{}, nil)
			})
		})

		t.Run("static provided", func(t *testing.T) {
			ctx := NewSymbolicContext(nil, nil, nil)
			state := newSymbolicState(ctx, emptyChunk)
			state.pushScope()

			static := &TypePattern{val: &Identifier{}}
			state.setLocal("local", &Identifier{}, static)

			info, ok := state.getLocal("local")
			assert.True(t, ok)
			assert.True(t, state.hasLocal("local"))
			assert.Equal(t, varSymbolicInfo{
				value:      &Identifier{},
				static:     &TypePattern{val: &Identifier{}},
				isConstant: false,
			}, info)
			assert.Same(t, static, info.static)
		})

		t.Run("no static provided", func(t *testing.T) {
			ctx := NewSymbolicContext(nil, nil, nil)
			state := newSymbolicState(ctx, emptyChunk)
			state.pushScope()

			state.setLocal("local", &Identifier{}, nil)

			info, ok := state.getLocal("local")
			assert.True(t, ok)
			assert.True(t, state.hasLocal("local"))
			assert.Equal(t, varSymbolicInfo{
				value:      &Identifier{},
				static:     &TypePattern{val: &Identifier{}},
				isConstant: false,
			}, info)
			assert.NotSame(t, info.value, info.static)
		})
	})

	t.Run("setGlobal()", func(t *testing.T) {
		ctx := NewSymbolicContext(nil, nil, nil)
		state := newSymbolicState(ctx, emptyChunk)

		state.setGlobal("g", &Identifier{}, GlobalVar)
		info, ok := state.getGlobal("g")
		assert.True(t, ok)
		assert.True(t, state.hasGlobal("g"))

		assert.Equal(t, varSymbolicInfo{
			value:      &Identifier{},
			static:     &TypePattern{val: &Identifier{}},
			isConstant: false,
		}, info)

		assert.NotSame(t, info.value, info.static)
	})

	t.Run("updateGlobal", func(t *testing.T) {
		t.Run("updating the value of a global should change its varSymbolicInfo.value but not its varSymbolicInfo.static", func(t *testing.T) {
			ctx := NewSymbolicContext(nil, nil, nil)
			state := newSymbolicState(ctx, emptyChunk)

			state.setGlobal("g", &Identifier{}, GlobalVar)
			state.updateGlobal("g", &Identifier{name: "foo"}, nil)
			info, _ := state.getGlobal("g")

			assert.Equal(t, varSymbolicInfo{
				value:      &Identifier{name: "foo"},
				static:     &TypePattern{val: &Identifier{}},
				isConstant: false,
			}, info)
		})
	})

	t.Run("updateLocal()", func(t *testing.T) {
		t.Run("updating the value of a local should change its varSymbolicInfo.value but not its varSymbolicInfo.static", func(t *testing.T) {
			ctx := NewSymbolicContext(nil, nil, nil)
			state := newSymbolicState(ctx, emptyChunk)
			state.pushScope()

			state.setLocal("local", &Identifier{}, nil)
			state.updateLocal("local", &Identifier{name: "foo"}, nil)
			info, _ := state.getLocal("local")

			assert.Equal(t, varSymbolicInfo{
				value:      &Identifier{name: "foo"},
				static:     &TypePattern{val: &Identifier{}},
				isConstant: false,
			}, info)
		})
	})

	t.Run("forking", func(t *testing.T) {
		t.Run("forked state's globals and locals are not shared with the parent, but have the same values", func(t *testing.T) {
			ctx := NewSymbolicContext(nil, nil, nil)
			state := newSymbolicState(ctx, emptyChunk)

			state.setGlobal("g", &Int{}, GlobalConst)
			state.pushScope()
			state.setLocal("local", ANY_BOOL, nil)

			fork := state.fork()

			//check globals
			parentStateGlobal, _ := state.getGlobal("g")
			forkStateGlobal, ok := fork.getGlobal("g")
			assert.True(t, ok)

			assert.Equal(t, parentStateGlobal, forkStateGlobal)
			assert.Same(t, parentStateGlobal.value, forkStateGlobal.value)
			assert.Same(t, parentStateGlobal.static, forkStateGlobal.static)

			//check locals
			parentStateLocal, _ := state.getLocal("local")
			forkStateLocal, ok := fork.getLocal("local")
			assert.True(t, ok)

			assert.Equal(t, parentStateLocal, forkStateLocal)
			assert.Same(t, parentStateLocal.value, forkStateLocal.value)
			assert.Same(t, parentStateLocal.static, forkStateLocal.static)
		})

		t.Run("setting a new global in the fork does not modify the parent", func(t *testing.T) {
			ctx := NewSymbolicContext(nil, nil, nil)
			state := newSymbolicState(ctx, emptyChunk)

			fork := state.fork()
			assert.True(t, fork.setGlobal("g", &String{}, GlobalConst))
			assert.Zero(t, state.globalCount())
			assert.Equal(t, 1, fork.globalCount())

			_, ok := state.getGlobal("g")
			assert.False(t, ok)
			assert.False(t, state.hasGlobal("g"))

			info, ok := fork.getGlobal("g")
			assert.True(t, ok)
			assert.Equal(t, varSymbolicInfo{
				value:      &String{},
				static:     &TypePattern{val: &String{}},
				isConstant: true,
			}, info)
		})

		t.Run("updating a global in the fork does not modify the parent", func(t *testing.T) {
			ctx := NewSymbolicContext(nil, nil, nil)
			state := newSymbolicState(ctx, emptyChunk)
			state.setGlobal("g", &Identifier{}, GlobalConst)
			infoInParent, _ := state.getGlobal("g")

			fork := state.fork()
			assert.True(t, fork.updateGlobal("g", &Identifier{name: "i"}, nil))

			newInfoInParent, _ := state.getGlobal("g")
			assert.Equal(t, infoInParent, newInfoInParent)

			infoInFork, _ := fork.getGlobal("g")
			assert.Equal(t, varSymbolicInfo{
				value:      &Identifier{name: "i"},
				static:     &TypePattern{val: &Identifier{}},
				isConstant: true,
			}, infoInFork)
		})

		t.Run("setting a new local in the fork does not modify the parent", func(t *testing.T) {
			ctx := NewSymbolicContext(nil, nil, nil)
			state := newSymbolicState(ctx, emptyChunk)
			state.pushScope()

			fork := state.fork()
			fork.setLocal("local", &String{}, nil)
			assert.Zero(t, state.localCount())
			assert.Equal(t, 1, fork.localCount())

			_, ok := state.getLocal("g")
			assert.False(t, ok)
			assert.False(t, state.hasLocal("local"))

			info, ok := fork.getLocal("local")
			assert.True(t, ok)
			assert.Equal(t, varSymbolicInfo{
				value:  &String{},
				static: &TypePattern{val: &String{}},
			}, info)
		})

		t.Run("updating a local in the fork does not modify the parent", func(t *testing.T) {
			ctx := NewSymbolicContext(nil, nil, nil)
			state := newSymbolicState(ctx, emptyChunk)
			state.pushScope()

			state.setLocal("local", &Identifier{}, nil)
			infoInParent, _ := state.getLocal("local")

			fork := state.fork()
			assert.True(t, fork.updateLocal("local", &Identifier{name: "i"}, nil))

			newInfoInParent, _ := state.getLocal("local")
			assert.Equal(t, infoInParent, newInfoInParent)

			infoInFork, _ := fork.getLocal("local")
			assert.Equal(t, varSymbolicInfo{
				value:  &Identifier{name: "i"},
				static: &TypePattern{val: &Identifier{}},
			}, infoInFork)
		})

		t.Run("the fork state ignores the local scope of the parent", func(t *testing.T) {
			ctx := NewSymbolicContext(nil, nil, nil)
			state := newSymbolicState(ctx, emptyChunk)
			state.pushScope()

			state.setLocal("local", &Identifier{}, nil)

			fork := state.fork()
			fork.pushScope()

			_, ok := fork.getLocal("local")
			assert.False(t, ok)
			assert.False(t, fork.hasLocal("local"))
		})
	})

}
