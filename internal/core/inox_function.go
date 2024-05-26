package core

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/ast"
	"github.com/inoxlang/inox/internal/parse"

	"github.com/inoxlang/inox/internal/core/staticcheck"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"golang.org/x/exp/maps"
)

// An InoxFunction is a Value that represents a function declared inside Inox code.
// Inox functions that are declared inside modules executed by the bytecode interpreter
// stores their bytecode and some other information.
type InoxFunction struct {
	Node  ast.Node
	Chunk *parse.ParsedChunkSource

	originState     *GlobalState
	shared          atomic.Bool
	capturedGlobals []capturedGlobal // set when shared, should not be nil in this case

	treeWalkCapturedLocals map[string]Value
	compiledFunction       *CompiledFunction //can be nil
	capturedLocals         []Value           //alway empty if .CompiledFunction is nil

	symbolicValue *symbolic.InoxFunction
	staticData    *staticcheck.FunctionData

	mutationFieldsLock sync.Mutex // exclusive access for initializing .watchers & .mutationCallbacks
	watchers           *ValueWatchers
	mutationCallbacks  *MutationCallbacks
	watchingDepth      WatchingDepth
}

type capturedGlobal struct {
	name  string
	value Value
}

func (fn *InoxFunction) FuncExpr() *ast.FunctionExpression {
	switch node := fn.Node.(type) {
	case *ast.FunctionDeclaration:
		return node.Function
	case *ast.FunctionExpression:
		return node
	default:
		if node == nil {
			return nil
		}
		panic(fmt.Errorf("InoxFunction has an invalid value for .Node: %#v", node))
	}
}

// Call executes the function with the provided global state, `self` value and arguments.
// If the function is compiled the bytecode interpreter is used.
func (fn *InoxFunction) Call(globalState *GlobalState, self Value, args []Value, disabledArgSharing []bool) (Value, error) {
	if fn.compiledFunction != nil {
		vm, err := NewVM(VMConfig{
			Bytecode:           fn.compiledFunction.Bytecode,
			Fn:                 fn,
			State:              globalState,
			Self:               self,
			FnArgs:             args,
			DisabledArgSharing: disabledArgSharing,
		})
		if err != nil {
			return nil, err
		}
		return vm.Run()
	} else {
		newState := NewTreeWalkStateWithGlobal(globalState)

		return TreeWalkCallFunc(TreeWalkCall{
			callee:             fn,
			self:               self,
			state:              newState,
			arguments:          args,
			disabledArgSharing: disabledArgSharing,
		})
	}
}

func (fn *InoxFunction) IsSharable(originState *GlobalState) (bool, string) {
	//TODO: only sharable if sharable captured locals ?

	if fn.staticData == nil {
		return true, ""
		//TODO: return false, "function is not sharable because static data is missing"
	}

	return true, ""
}

func (fn *InoxFunction) Share(originState *GlobalState) {
	if fn.shared.CompareAndSwap(false, true) {
		fn.originState = originState
		if fn.staticData != nil && len(fn.staticData.CapturedGlobals()) > 0 {
			capturedGlobals := fn.staticData.CapturedGlobals()

			fn.capturedGlobals = make([]capturedGlobal, len(capturedGlobals))
			for i, name := range capturedGlobals {

				value := originState.Globals.Get(name)
				if value == nil {
					panic(fmt.Errorf("function sharing: failed to capture global variable '%s' of origin state", name))
				}
				fn.capturedGlobals[i] = capturedGlobal{
					name:  name,
					value: value,
				}
			}
		}
	}
}

func (fn *InoxFunction) IsShared() bool {
	return fn.shared.Load()
}

func (fn *InoxFunction) SmartLock(state *GlobalState) {

}

func (fn *InoxFunction) SmartUnlock(state *GlobalState) {

}

func (fn *InoxFunction) CapturedLocals() []Value {
	if fn.capturedLocals != nil {
		return slices.Clone(fn.capturedLocals)
	}
	return maps.Values(fn.treeWalkCapturedLocals)
}

func (fn *InoxFunction) OriginStateEqual(state *GlobalState) bool {
	return fn.originState == state
}

// checkTransformInoxMustCallResult checks the result of an Inox function 'must' call:
// - if the result is an error it returns (nil, the error).
// - if the result is an Array of length > 2, the function returns a slice of the array with one less element.
// - if the result is an Array of length 2, the function returns the first element.
// - if the result is not an error, it is returned unmodified.
func checkTransformInoxMustCallResult(result Value) (Value, error) {
	reflectVal := reflect.ValueOf(result)
	if reflectVal.Type().Implements(ERROR_INTERFACE_TYPE) {
		return nil, reflectVal.Interface().(error)
	}

	if array, isArray := result.(*Array); isArray {
		if array.Len() < 2 {
			return nil, errors.New("unreachable: array of length < 2 returned by a 'must' call")
		}

		length := array.Len()
		lastElem := reflect.ValueOf((*array)[length-1])
		if lastElem.Type().Implements(ERROR_INTERFACE_TYPE) {
			return nil, lastElem.Interface().(error)
		}
		if length == 2 {
			return (*array)[0], nil
		}
		slice := (*array)[:length-1]
		return &slice, nil
	}

	return result, nil
}
