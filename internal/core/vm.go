package core

import (
	"errors"
	"fmt"
	"math"
	"runtime/debug"
	"strings"
	"sync/atomic"
	"unsafe"

	permkind "github.com/inoxlang/inox/internal/core/permkind"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	VM_STACK_SIZE = 200
	MAX_FRAMES    = 20
)

var (
	ErrArgsProvidedToModule    = errors.New("cannot provide arguments when running module")
	ErrInvalidProvidedArgCount = errors.New("number of provided arguments is invalid")

	_ = parse.StackItem(frame{})
)

// VM is a virtual machine that executes bytecode.
type VM struct {
	constants        []Value
	stack            [VM_STACK_SIZE]Value
	sp               int
	global           *GlobalState
	module           *Module
	frames           [MAX_FRAMES]frame
	framesIndex      int //first is 1 not zero, the current frame is accessed with vm.frames[framesIndex-1]
	curFrame         *frame
	curInsts         []byte
	ip               int
	aborting         int64
	err              error
	moduleLocalCount int

	chunkStack []*parse.ChunkStackItem

	//the following fields are only set for isolated function calls.

	runFn              bool
	fnArgCount         int
	disabledArgSharing []bool
}

// frame represents a call frame.
type frame struct {
	fn           *CompiledFunction
	mustCall     bool
	bytecode     *Bytecode
	ip           int
	basePointer  int
	self         Value
	lockedValues []PotentiallySharable

	externalFunc       bool
	popCapturedGlobals bool
	originalConstants  []Value

	//set when a function is called or an included chunk starts to be executed.
	currentNodeSpan parse.NodeSpan
}

func (f frame) GetCurrentNodeSpan() (parse.NodeSpan, bool) {
	return f.currentNodeSpan, f.currentNodeSpan != parse.NodeSpan{}
}

func (f frame) GetChunk() (*parse.ParsedChunk, bool) {
	if f.fn.IncludedChunk != nil {
		return f.fn.IncludedChunk, true
	}
	return f.fn.Bytecode.module.MainChunk, true
}

type VMConfig struct {
	Bytecode *Bytecode //bytecode of the module or bytecode of the function called in isolation.
	State    *GlobalState
	Self     Value

	//isolated call
	Fn                 *InoxFunction
	FnArgs             []Value
	DisabledArgSharing []bool
}

// NewVM creates a virtual machine that will execute the fn function, if fn is nil the main function of bytecode will be executed.
// state is used to retrieve and set global variables.
func NewVM(config VMConfig) (*VM, error) {
	runFn := false

	bytecode := config.Bytecode
	inoxFn := config.Fn
	var fn *CompiledFunction
	state := config.State
	self := config.Self
	fnArgs := config.FnArgs

	if inoxFn == nil {
		fn = bytecode.main
		if len(fnArgs) != 0 {
			return nil, ErrArgsProvidedToModule
		}
		if fn.ParamCount != len(fnArgs) {
			return nil, ErrInvalidProvidedArgCount
		}
		if self != nil && config.Bytecode.module.ModuleKind != LifetimeJobModule {
			return nil, errors.New("cannot set self: module is not a lifetime job module")
		}
	} else {
		runFn = true
		fn = inoxFn.compiledFunction
		if fn.IsVariadic {
			return nil, errors.New("variadic function not supported yet")
		}
	}

	v := &VM{
		global:             state,
		constants:          bytecode.constants,
		sp:                 0,
		module:             bytecode.module,
		framesIndex:        1,
		ip:                 -1,
		moduleLocalCount:   fn.LocalCount,
		runFn:              runFn,
		fnArgCount:         len(fnArgs),
		disabledArgSharing: config.DisabledArgSharing,
	}
	v.frames[0].fn = fn
	v.frames[0].ip = -1
	v.frames[0].self = self
	v.frames[0].lockedValues = nil
	v.frames[0].bytecode = bytecode
	v.curFrame = &v.frames[0]
	v.curInsts = v.curFrame.fn.Instructions

	if runFn {
		v.sp++ // result slot

		copy(v.stack[1:], fnArgs)
		v.sp += len(fnArgs)

		v.stack[v.sp] = self
		v.sp++

		//callee
		v.stack[v.sp] = config.Fn
		v.sp++
	}

	return v, nil
}

// Abort aborts the execution.
func (v *VM) Abort() {
	atomic.StoreInt64(&v.aborting, 1)
}

// Run starts the execution.
func (v *VM) Run() (result Value, err error) {
	// reset state
	v.sp = v.moduleLocalCount
	v.curFrame = &(v.frames[0])
	v.curInsts = v.curFrame.fn.Instructions
	v.framesIndex = 1
	v.ip = -1

	if v.runFn {
		v.sp = 1 + v.fnArgCount + 1 + 1
		if !v.fnCall(v.fnArgCount, false, false, -1) {
			return nil, v.err
		}
	} else {
		v.chunkStack = []*parse.ChunkStackItem{
			{
				Chunk: v.module.MainChunk,
			},
		}
	}
	v.run()
	atomic.StoreInt64(&v.aborting, 0)
	err = v.err
	if err != nil {
		return nil, err
	}

	// return module's result

	if v.sp <= v.moduleLocalCount {
		//TODO: panic if < moduleLocalCount ?
		return Nil, nil
	}

	return v.stack[v.sp-1], nil
}

func (v *VM) run() {

	ip := -1
	defer func() {
		e := recover()

		if e == nil {
			e = v.err
		}

		if e != nil {
			var assertionErr *AssertionError

			if er, ok := e.(error); ok {
				if errors.As(er, &assertionErr) {
					assertionErr = assertionErr.ShallowCopy()
					er = assertionErr
				}

				v.err = fmt.Errorf("vm: error: %w %s", er, debug.Stack())
			} else {
				v.err = fmt.Errorf("vm: %s", e)
			}

			//add location to error message

			if !v.runFn {
				//build the stack
				currentNodePos := v.curFrame.fn.GetSourcePositionRange(ip)

				var stackItems []parse.StackItem

				//add the mainn chunk and the included chunks
				for _, chunk := range v.chunkStack {
					stackItems = append(stackItems, chunk)
				}

				//add call frames.
				//note: we start at 1 because the first frame is the module's frame.
				for i := 1; i < v.framesIndex; i++ {
					frame := v.frames[i]

					functionChunk, ok := frame.GetChunk()
					//add the position of the function's start.
					if ok {
						functionStackItem := parse.ChunkStackItem{
							Chunk:           functionChunk,
							CurrentNodeSpan: frame.fn.SourceNodeSpan,
						}
						stackItems = append(stackItems, functionStackItem)
					}

					stackItems = append(stackItems, frame)
				}

				positionStack, formatted := parse.GetSourcePositionStack(currentNodePos.Span, stackItems)

				//wrap v.err in a LocatedEvalError

				v.err = LocatedEvalError{
					error:    fmt.Errorf("%s %w", formatted, v.err),
					Message:  v.err.Error(),
					Location: positionStack,
				}
			} else {
				//TODO
			}
		}

		// for i := len(v.curFrame.lockedValues) - 1; i >= 0; i-- {
		// 	// val := v.curFrame.lockedValues[i]
		// 	// v.curFrame.lockedValues = v.curFrame.lockedValues[:i]
		// 	// val.SynchronizedBlockUnlock(v.global)
		// }

		for _, locked := range v.global.lockedValues {
			locked.ForceUnlock()
		}

	}()

	// main evaluation loop
	// While we are not aborting we increment the instruction pointer and we execute the current instruction.
	for atomic.LoadInt64(&v.aborting) == 0 {
		v.ip++

		//TODO: turn into instruction for better performance| abort if done ?
		select {
		case <-v.global.Ctx.Done():
			panic(v.global.Ctx.Err())
		default:
		}

		ip = v.ip

		switch v.curInsts[ip] {
		//STACK OPERATIONS AND CONSTANTS
		case OpPushConstant:
			v.ip += 2
			cidx := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8

			v.stack[v.sp] = v.constants[cidx]
			v.sp++
		case OpPushNil:
			v.stack[v.sp] = Nil
			v.sp++
		case OpPop:
			v.sp--
		case OpCopyTop:
			v.stack[v.sp] = v.stack[v.sp-1]
			v.sp++
		case OpSwap:
			temp := v.stack[v.sp-1]
			v.stack[v.sp-1] = v.stack[v.sp-2]
			v.stack[v.sp-2] = temp
		case OpMoveThirdTop:
			third := v.stack[v.sp-3]
			second := v.stack[v.sp-2]
			top := v.stack[v.sp-1]

			v.stack[v.sp-1] = third
			v.stack[v.sp-2] = top
			v.stack[v.sp-3] = second
		case OpPushTrue:
			v.stack[v.sp] = True
			v.sp++
		case OpPushFalse:
			v.stack[v.sp] = False
			v.sp++
		//CONTROL FLOW
		case OpJumpIfFalse:
			v.ip += 2
			v.sp--
			if !v.stack[v.sp].(Bool) {
				pos := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
				v.ip = pos - 1
			}
		case OpAndJump:
			v.ip += 2
			if !v.stack[v.sp-1].(Bool) {
				pos := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
				v.ip = pos - 1
			} else {
				v.sp--
			}
		case OpOrJump:
			v.ip += 2
			if !v.stack[v.sp-1].(Bool) {
				v.sp--
			} else {
				pos := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
				v.ip = pos - 1
			}
		case OpJump:
			pos := int(v.curInsts[v.ip+2]) | int(v.curInsts[v.ip+1])<<8
			v.ip = pos - 1
		case OpCall:
			callIp := v.ip

			numArgs := int(v.curInsts[v.ip+1])
			spread := int(v.curInsts[v.ip+2])
			must := int(v.curInsts[v.ip+3])
			v.ip += 3

			if !v.fnCall(numArgs, spread == 1, must == 1, callIp) {
				return
			}
		case OpReturn:
			v.ip++
			var retVal Value
			isValOnStack := int(v.curInsts[v.ip]) == 1
			if isValOnStack {
				retVal = v.stack[v.sp-1]

				if v.curFrame.mustCall {
					if transformed, err := checkTransformInoxMustCallResult(retVal); err == nil {
						retVal = transformed
					} else {
						v.err = err
						return
					}
				}
			} else {
				retVal = Nil
			}

			topLevelFnEval := v.runFn && v.framesIndex == 2

			if v.framesIndex == 1 { //top level return in module
				if isValOnStack {
					v.stack[v.sp-1] = retVal
				} else {
					v.stack[v.sp] = retVal
					v.sp++
				}
				return
			}

			if v.curFrame.popCapturedGlobals {
				v.curFrame.popCapturedGlobals = false
				v.global.Globals.PopCapturedGlobals()
			}

			if v.curFrame.externalFunc {
				shared, err := ShareOrClone(retVal, v.global)
				if err != nil {
					v.err = fmt.Errorf("failed to share a return value: %w", err)
					return
				}
				retVal = shared
				v.constants = v.curFrame.originalConstants
				v.curFrame.originalConstants = nil
			}

			if !topLevelFnEval {
				v.framesIndex--
				v.curFrame = &v.frames[v.framesIndex-1]
				v.curInsts = v.curFrame.fn.Instructions
				v.ip = v.curFrame.ip
				v.sp = v.frames[v.framesIndex].basePointer

				if !v.runFn {
					v.curFrame.currentNodeSpan = parse.NodeSpan{}
					v.chunkStack[len(v.chunkStack)-1].CurrentNodeSpan = parse.NodeSpan{}
				} else {
					//TODO: support isolated function call
				}
			}

			v.stack[v.sp-1] = retVal

			if topLevelFnEval {
				return
			}
		//COMPARISON <, <= , =>, >
		case OpLess:
			leftOperand := v.stack[v.sp-2].(Comparable)
			rightOperand := v.stack[v.sp-1]
			result, comparable := leftOperand.Compare(rightOperand)

			if !comparable {
				if !v.checkComparisonOperands(leftOperand, rightOperand) {
					v.err = ErrNotComparable
				}
				return
			}
			v.sp--
			v.stack[v.sp-1] = Bool(result < 0)
		case OpLessEqual:
			leftOperand := v.stack[v.sp-2].(Comparable)
			rightOperand := v.stack[v.sp-1]
			result, comparable := leftOperand.Compare(rightOperand)

			if !comparable {
				if !v.checkComparisonOperands(leftOperand, rightOperand) {
					v.err = ErrNotComparable
				}
				return
			}
			v.sp--
			v.stack[v.sp-1] = Bool(result <= 0)
		case OpGreater:
			leftOperand := v.stack[v.sp-2].(Comparable)
			rightOperand := v.stack[v.sp-1]
			result, comparable := leftOperand.Compare(rightOperand)

			if !comparable {
				if !v.checkComparisonOperands(leftOperand, rightOperand) {
					v.err = ErrNotComparable
				}
				return
			}
			v.sp--
			v.stack[v.sp-1] = Bool(result > 0)
		case OpGreaterEqual:
			leftOperand := v.stack[v.sp-2].(Comparable)
			rightOperand := v.stack[v.sp-1]
			result, comparable := leftOperand.Compare(rightOperand)

			if !comparable {
				if !v.checkComparisonOperands(leftOperand, rightOperand) {
					v.err = ErrNotComparable
				}
				return
			}
			v.sp--
			v.stack[v.sp-1] = Bool(result >= 0)
		//ARITHMETIC
		case OpIntBin:
			v.doSafeIntBinOp()
			if v.err != nil {
				return
			}
		case OpFloatBin:
			v.doSafeFloatBinOp()
			if v.err != nil {
				return
			}
		case OpNumBin:
			if _, ok := v.stack[v.sp-2].(Int); ok {
				v.doSafeIntBinOp()
			} else {
				v.doSafeFloatBinOp()
			}
			if v.err != nil {
				return
			}
		case OpMinus:
			operand := v.stack[v.sp-1]
			v.sp--

			switch x := operand.(type) {
			case Int:
				if x == -x && x != 0 {
					v.err = ErrNegationWithOverflow
					return
				}
				var res Value = -x
				v.stack[v.sp] = res
				v.sp++
			case Float:
				var res Value = -x
				v.stack[v.sp] = res
				v.sp++
			default:
				v.err = fmt.Errorf("invalid operation: -%s", Stringify(operand, v.global.Ctx))
				return
			}
		//CONCATENATION
		case OpStrConcat:
			right := v.stack[v.sp-1]
			left := v.stack[v.sp-2]
			res := Str(left.(WrappedString).UnderlyingString() + right.(WrappedString).UnderlyingString())

			v.stack[v.sp-2] = res
			v.sp--
		case OpConcat:
			v.ip += 3
			numElements := int(v.curInsts[v.ip-2])
			spreadElemSetConstantIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			spreadElemSet := v.constants[spreadElemSetConstantIndex].(*List).underlyingList.(*BoolList)

			values := make([]Value, numElements)
			copy(values, v.stack[v.sp-numElements:v.sp])

			if spreadElemSet.elements.Any() {
				index := 0
				valuesAfterIndex := make([]Value, numElements)

				// TODO: if iterables are all indexable & not shared we can pre allocate a list of the right size

				ctx := v.global.Ctx

				for i := 0; i < numElements; i++ {
					if !spreadElemSet.BoolAt(i) {
						index++
						continue
					}
					copiedCount := copy(valuesAfterIndex, values[index+1:]) //save values after current index
					iterable := values[index].(Iterable)
					values = values[:index]

					it := iterable.Iterator(ctx, IteratorConfiguration{})
					for it.Next(ctx) {
						values = append(values, it.Value(ctx))
					}

					index = len(values)
					values = append(values, valuesAfterIndex[:copiedCount]...)
				}
			}

			v.sp -= numElements
			result, err := concatValues(v.global.Ctx, values)
			if err != nil {
				v.err = err
				return
			}
			v.stack[v.sp] = result
			v.sp++
		case OpRange:
			right := v.stack[v.sp-1]
			left := v.stack[v.sp-2]

			v.ip++
			exclEnd := v.curInsts[v.ip] == 1

			var res Value

			switch left.(type) {
			case Int:
				res = IntRange{
					inclusiveEnd: !exclEnd,
					start:        int64(left.(Int)),
					end:          int64(right.(Int)),
					step:         1,
				}
			case Float:
				res = FloatRange{
					inclusiveEnd: !exclEnd,
					start:        float64(left.(Float)),
					end:          float64(right.(Float)),
				}
			default:
				res = QuantityRange{
					inclusiveEnd: !exclEnd,
					start:        left.(Serializable),
					end:          right.(Serializable),
				}
			}
			v.stack[v.sp-2] = res
			v.sp--
		//BINARY OPERATIONS
		case OpNotEqual, OpEqual:
			op := v.curInsts[v.ip]

			right := v.stack[v.sp-1]
			left := v.stack[v.sp-2]

			v.sp -= 2

			if left.Equal(v.global.Ctx, right, map[uintptr]uintptr{}, 0) == (op == OpEqual) {
				v.stack[v.sp] = True
			} else {
				v.stack[v.sp] = False
			}
			v.sp++
		case OpIsNot, OpIs:
			op := v.curInsts[v.ip]
			right := v.stack[v.sp-1]
			left := v.stack[v.sp-2]

			v.sp -= 2

			if Same(left, right) == (op == OpIs) {
				v.stack[v.sp] = True
			} else {
				v.stack[v.sp] = False
			}
			v.sp++
		case OpBooleanNot:
			operand := v.stack[v.sp-1]
			v.sp--
			if operand.(Bool) {
				v.stack[v.sp] = False
			} else {
				v.stack[v.sp] = True
			}
			v.sp++
		case OpMatch:
			left := v.stack[v.sp-2]
			right := v.stack[v.sp-1]
			v.sp -= 2

			if pattern, ok := right.(Pattern); ok {
				if pattern.Test(v.global.Ctx, left) {
					v.stack[v.sp] = True
				} else {
					v.stack[v.sp] = False
				}
			} else {
				if right.Equal(v.global.Ctx, left, map[uintptr]uintptr{}, 0) {
					v.stack[v.sp] = True
				} else {
					v.stack[v.sp] = False
				}
			}

			v.sp++
		case OpIn:
			left := v.stack[v.sp-2]
			right := v.stack[v.sp-1]
			v.sp -= 2

			var val Value

			switch rightVal := right.(type) {
			case Container:
				val = Bool(rightVal.Contains(v.global.Ctx, left.(Serializable)))
			default:
				v.err = fmt.Errorf("invalid binary expression: cannot check if value is inside a(n) %T", rightVal)
				return
			}

			if val == nil {
				val = False
			}

			v.stack[v.sp] = val
			v.sp++
		case OpSubstrOf:
			left := v.stack[v.sp-2]
			right := v.stack[v.sp-1]
			v.sp -= 2

			val := strings.Contains(right.(WrappedString).UnderlyingString(), left.(WrappedString).UnderlyingString())

			v.stack[v.sp] = Bool(val)
			v.sp++
		case OpKeyOf:
			left := v.stack[v.sp-2].(Str)
			right := v.stack[v.sp-1].(*Object)
			v.sp -= 2

			v.stack[v.sp] = Bool(right.HasProp(v.global.Ctx, string(left)))
			v.sp++
		case OpUrlOf:
			left := v.stack[v.sp-2].(URL)
			right, isUrlHolder := v.stack[v.sp-1].(UrlHolder)
			v.sp -= 2

			var result = false
			if isUrlHolder {
				actualURL, ok := right.URL()
				if ok {
					result = left.Equal(v.global.Ctx, actualURL, nil, 0)
				}
			}

			v.stack[v.sp] = Bool(result)
			v.sp++
		case OpNilCoalesce:
			left := v.stack[v.sp-2]
			right := v.stack[v.sp-1]
			v.sp -= 2

			val := left

			if _, ok := left.(NilT); ok {
				val = right
			}
			v.stack[v.sp] = val
			v.sp++
		//VARIABLES
		case OpSetLocal:
			v.ip++
			localIndex := int(v.curInsts[v.ip])
			sp := v.curFrame.basePointer + localIndex

			// local variables can be mutated by other actions
			// so always store the copy of popped value
			//true for Inox ?
			val := v.stack[v.sp-1]
			v.sp--
			v.stack[sp] = val
		case OpGetLocal:
			v.ip++
			localIndex := int(v.curInsts[v.ip])
			val := v.stack[v.curFrame.basePointer+localIndex]
			v.stack[v.sp] = val
			v.sp++
		case OpSetGlobal:
			v.ip += 2
			v.sp--
			globalNameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			globalName := v.constants[globalNameIndex].(Str)

			val := v.stack[v.sp]

			err := v.global.Globals.SetCheck(string(globalName), val, func(defined bool) error {
				perm := GlobalVarPermission{Kind_: permkind.Create, Name: string(globalName)}
				if defined {
					perm.Kind_ = permkind.Update
				}

				return v.global.Ctx.CheckHasPermission(perm)
			})

			if err != nil {
				v.err = err
				return
			}

			if watchable, ok := val.(SystemGraphNodeValue); ok {
				v.global.ProposeSystemGraph(watchable, string(globalName))
			}
		case OpGetGlobal:
			v.ip += 2
			globalNameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			globalName := v.constants[globalNameIndex].(Str)

			val := v.global.Globals.Get(string(globalName))

			if val == nil {
				v.err = fmt.Errorf("global '%s' is not defined", globalName)
				return
			}
			v.stack[v.sp] = val
			v.sp++
		//MEMBER AND ELEMENT
		case OpSetMember:
			v.ip += 2
			v.sp--
			memberNameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			memberName := v.constants[memberNameIndex].(Str)
			val := v.stack[v.sp]

			iprops := v.stack[v.sp-1].(IProps)
			v.sp--

			if err := iprops.SetProp(v.global.Ctx, string(memberName), val); err != nil {
				v.err = err
				return
			}
		case OpSetIndex:
			slice := v.stack[v.sp-3].(MutableSequence)
			index := int(v.stack[v.sp-2].(Int))
			val := v.stack[v.sp-1]
			v.sp -= 3
			slice.set(v.global.Ctx, index, val)
		case OpSetSlice:
			slice := v.stack[v.sp-4].(MutableSequence)
			startIndexVal := v.stack[v.sp-3]
			endIndexVal := v.stack[v.sp-2]
			val := v.stack[v.sp-1]
			v.sp -= 4

			startIndex, ok := startIndexVal.(Int)
			if !ok {
				startIndex = 0
			}

			endIndex, ok := endIndexVal.(Int)
			if !ok {
				endIndex = min(endIndex, Int(slice.Len()))
			}

			slice.SetSlice(v.global.Ctx, int(startIndex), int(endIndex), val.(Sequence))
		case OpSetBoolField:
			v.ip += 4
			structSize := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
			offset := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			struct_ := v.stack[v.sp-2].(*Struct)
			value := v.stack[v.sp-1].(Bool)
			v.sp -= 2

			structHelperFromPtr(struct_, structSize).SetBool(offset, value)
		case OpSetIntField:
			v.ip += 4
			structSize := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
			offset := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			struct_ := v.stack[v.sp-2].(*Struct)
			value := v.stack[v.sp-1].(Int)
			v.sp -= 2

			structHelperFromPtr(struct_, structSize).SetInt(offset, value)
		case OpSetFloatField:
			v.ip += 4
			structSize := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
			offset := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			struct_ := v.stack[v.sp-2].(*Struct)
			value := v.stack[v.sp-1].(Float)
			v.sp -= 2

			structHelperFromPtr(struct_, structSize).SetFloat(offset, value)
		case OpSetStructPtrField:
			v.ip += 4
			structSize := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
			offset := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			struct_ := v.stack[v.sp-2].(*Struct)
			value := v.stack[v.sp-1].(*Struct)
			v.sp -= 2

			structHelperFromPtr(struct_, structSize).SetStructPointer(offset, value)
		case OpOptionalMemb:
			object := v.stack[v.sp-1]
			v.ip += 2
			memberNameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			memberName := string(v.constants[memberNameIndex].(Str))

			iprops := object.(IProps)

			var memb Value
			if !utils.SliceContains(iprops.PropertyNames(v.global.Ctx), memberName) {
				memb = Nil
			} else {
				memb = iprops.Prop(v.global.Ctx, memberName)
			}

			v.stack[v.sp-1] = memb
		case OpComputedMemb:
			object := v.stack[v.sp-2]
			propNameVal := v.stack[v.sp-1]
			propName := propNameVal.(StringLike).GetOrBuildString()

			memb := object.(IProps).Prop(v.global.Ctx, propName)
			v.stack[v.sp-2] = memb
			v.sp--
		case OpAt:
			index := v.stack[v.sp-1]
			left := v.stack[v.sp-2]
			v.sp -= 2

			val := left.(Indexable).At(v.global.Ctx, int(index.(Int)))
			if val == nil {
				val = Nil
			}

			v.stack[v.sp] = val
			v.sp++
		case OpSafeAt:
			index := v.stack[v.sp-1]
			left := v.stack[v.sp-2]
			v.sp -= 2

			var val Value
			indexable := left.(Indexable)
			_index := int(index.(Int))
			if _index >= indexable.Len() {
				val = Nil
			} else {
				val = indexable.At(v.global.Ctx, _index)
			}

			v.stack[v.sp] = val
			v.sp++
		case OpSlice:
			high := v.stack[v.sp-1]
			low := v.stack[v.sp-2]
			left := v.stack[v.sp-3]
			v.sp -= 3
			slice := left.(Sequence)

			var lowIdx int = 0
			if low != Nil {
				lowIdx = int(low.(Int))
			}

			if lowIdx < 0 {
				v.err = ErrNegativeLowerIndex
				return
			}

			var highIdx int = math.MaxInt
			if high != Nil {
				highIdx = int(high.(Int))
			}
			highIdx = min(highIdx, int(slice.Len()))

			val := slice.slice(lowIdx, highIdx)

			v.stack[v.sp] = val
			v.sp++
		case OpMemb:
			object := v.stack[v.sp-1]
			v.ip += 2
			memberNameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			memberName := string(v.constants[memberNameIndex].(Str))

			memb := object.(IProps).Prop(v.global.Ctx, memberName)
			v.stack[v.sp-1] = memb
		case OpGetBoolField:
			v.ip += 4
			structSize := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
			offset := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			struct_ := v.stack[v.sp-1].(*Struct)

			v.stack[v.sp-1] = structHelperFromPtr(struct_, structSize).GetBool(offset)
		case OpGetIntField:
			v.ip += 4
			structSize := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
			offset := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			struct_ := v.stack[v.sp-1].(*Struct)

			v.stack[v.sp-1] = structHelperFromPtr(struct_, structSize).GetInt(offset)
		case OpGetFloatField:
			v.ip += 4
			structSize := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
			offset := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			struct_ := v.stack[v.sp-1].(*Struct)

			v.stack[v.sp-1] = structHelperFromPtr(struct_, structSize).GetFloat(offset)
		case OpGetStructPtrField:
			v.ip += 4
			structSize := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
			offset := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			struct_ := v.stack[v.sp-1].(*Struct)

			v.stack[v.sp-1] = structHelperFromPtr(struct_, structSize).GetStructPointer(offset)
		case OpObjPropNotStored:
			val := v.stack[v.sp-1]
			v.ip += 2
			memberNameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			memberName := string(v.constants[memberNameIndex].(Str))

			object := val.(*Object)
			memb := object.PropNotStored(v.global.Ctx, memberName)
			v.stack[v.sp-1] = memb
		case OpExtensionMethod:
			v.ip += 4
			extensionIdIndex := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
			extensionId := v.constants[extensionIdIndex].(Str).GetOrBuildString()

			memberNameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			memberName := string(v.constants[memberNameIndex].(Str))

			extension := v.global.Ctx.GetTypeExtension(extensionId)
			var method *InoxFunction

			for _, propExpr := range extension.propertyExpressions {
				if propExpr.name == memberName {
					method = propExpr.method
					break
				}
			}

			if method == nil {
				v.err = fmt.Errorf("%w: extension method should have been found", ErrUnreachable)
				return
			}

			v.stack[v.sp] = method
			v.sp++
		//DATA STRUCTURE CREATION
		case OpCreateList:
			v.ip += 2
			numElements := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8

			var elements []Serializable
			if numElements > 0 {
				elements = make([]Serializable, numElements)
			}

			ind := 0
			for i := v.sp - numElements; i < v.sp; i++ {
				elements[ind] = v.stack[i].(Serializable)
				ind++
			}
			v.sp -= numElements

			var arr Value = &List{underlyingList: &ValueList{elements: elements}}

			v.stack[v.sp] = arr
			v.sp++
		case OpCreateTuple:
			v.ip += 2
			numElements := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8

			var elements = make([]Serializable, 0, numElements)
			for i := v.sp - numElements; i < v.sp; i++ {
				elements = append(elements, v.stack[i].(Serializable))
			}
			v.sp -= numElements

			var arr Value = &Tuple{elements: elements}

			v.stack[v.sp] = arr
			v.sp++
		case OpCreateObject:
			v.ip += 6
			numElements := int(v.curInsts[v.ip-4]) | int(v.curInsts[v.ip-5])<<8
			implicitPropCount := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
			astNodeIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8

			astNode := v.constants[astNodeIndex].(AstNode).Node.(*parse.ObjectLiteral)

			var obj *Object
			if numElements > 0 {
				obj = newUnitializedObjectWithPropCount(numElements / 2)

				propIndex := 0
				for i := v.sp - numElements; i < v.sp; i += 2 {
					obj.values[propIndex] = v.stack[i+1].(Serializable)
					obj.keys[propIndex] = string(v.stack[i].(Str))
					propIndex++
				}
				obj.sortProps()
				// add handlers before because jobs can mutate the object
				if err := obj.addMessageHandlers(v.global.Ctx); err != nil {
					v.err = err
					return
				}
				if err := obj.instantiateLifetimeJobs(v.global.Ctx); err != nil {
					v.err = err
					return
				}
			} else {
				obj = &Object{}
			}

			initializeMetaproperties(obj, astNode.MetaProperties)
			obj.implicitPropCount = implicitPropCount

			v.sp -= numElements
			v.stack[v.sp] = obj
			v.sp++
		case OpCreateRecord:
			v.ip += 4
			implicitPropCount := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
			numElements := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8

			var rec *Record
			if numElements > 0 {
				rec = &Record{
					keys:   make([]string, numElements/2),
					values: make([]Serializable, numElements/2),
				}

				propIndex := 0
				for i := v.sp - numElements; i < v.sp; i += 2 {
					rec.values[propIndex] = v.stack[i+1].(Serializable)
					rec.keys[propIndex] = string(v.stack[i].(Str))
					propIndex++
				}
				rec.sortProps()
			} else {
				rec = &Record{}
			}

			//TODO: initializeMetaproperties(obj, astNode.MetaProperties)
			rec.implicitPropCount = implicitPropCount

			v.sp -= numElements
			v.stack[v.sp] = rec
			v.sp++
		case OpCreateDict:
			v.ip += 2
			numElements := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			var dict = &Dictionary{
				entries: make(map[string]Serializable, numElements/2),
				keys:    make(map[string]Serializable, numElements/2),
			}

			for i := v.sp - numElements; i < v.sp; i += 2 {
				key := v.stack[i].(Serializable)
				keyRepr := string(GetRepresentation(key, v.global.Ctx))
				value := v.stack[i+1]
				dict.entries[keyRepr] = value.(Serializable)
				dict.keys[keyRepr] = key
			}
			v.sp -= numElements
			v.stack[v.sp] = dict
			v.sp++
		case OpCreateMapping:
			v.ip += 2
			nodeIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			node := v.constants[nodeIndex].(AstNode).Node.(*parse.MappingExpression)

			mapping, err := NewMapping(node, v.global)
			if err != nil {
				v.err = err
				return
			}
			v.stack[v.sp] = mapping
			v.sp++
		case OpCreateTreedata:
			v.ip += 2
			numHiearchyEntries := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			treedata := &Treedata{}

			for i := v.sp - numHiearchyEntries; i < v.sp; i++ {
				entry := v.stack[i]
				treedata.HiearchyEntries = append(treedata.HiearchyEntries, entry.(TreedataHiearchyEntry))
			}

			v.sp -= numHiearchyEntries
			treedata.Root = v.stack[v.sp-1].(Serializable)
			v.stack[v.sp-1] = treedata
		case OpCreateTreedataHiearchyEntry:
			v.ip += 2
			numChildren := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			entry := TreedataHiearchyEntry{}

			for i := v.sp - numChildren; i < v.sp; i++ {
				child := v.stack[i]
				entry.Children = append(entry.Children, child.(TreedataHiearchyEntry))
			}

			v.sp -= numChildren
			entry.Value = v.stack[v.sp-1].(Serializable)
			v.stack[v.sp-1] = entry
		case OpCreateOrderedPair:
			first := v.stack[v.sp-2]
			second := v.stack[v.sp-1]
			v.stack[v.sp-2] = NewOrderedPair(first.(Serializable), second.(Serializable))
			v.sp--
		case OpCreateStruct:
			v.ip += 3
			structTypeIndex := int(v.curInsts[v.ip-1]) | int(v.curInsts[v.ip-2])<<8
			numElements := int(v.curInsts[v.ip])

			structType := v.constants[structTypeIndex].(*ModuleParamsPattern)

			values := make([]Value, numElements)
			fieldIndex := 0
			for i := v.sp - numElements; i < v.sp; i++ {
				values[fieldIndex] = v.stack[i]
				fieldIndex++
			}

			v.sp -= numElements
			v.stack[v.sp] = &ModuleArgs{
				structType: structType,
				values:     values,
			}
			v.sp++
		case OpSpreadObject:
			object := v.stack[v.sp-1].(*Object)
			spreadObject := v.stack[v.sp-2].(*Object)
			v.sp -= 2

			for i, v := range spreadObject.values {
				object.keys = append(object.keys, spreadObject.keys[i])
				object.values = append(object.values, v)
			}

			v.stack[v.sp] = object
			v.sp++
		case OpExtractProps:
			v.ip += 2
			object := v.stack[v.sp-1].(*Object)
			keyListIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			keyList := v.constants[keyListIndex].(KeyList)

			newObject := newUnitializedObjectWithPropCount(len(keyList))

			for _, key := range keyList {
				newObject.SetProp(v.global.Ctx, key, object.Prop(v.global.Ctx, key))
			}
			v.stack[v.sp-1] = object
		case OpSpreadList:
			destList := v.stack[v.sp-2].(*List)
			spreadList := v.stack[v.sp-1].(*List)

			destList.append(v.global.Ctx, spreadList.GetOrBuildElements(v.global.Ctx)...)
			v.stack[v.sp-2] = destList
			v.sp--
		case OpSpreadTuple:
			destTuple := v.stack[v.sp-2].(*Tuple)
			spreadTuple := v.stack[v.sp-1].(*Tuple)

			destTuple.elements = append(destTuple.elements, spreadTuple.elements...)
			v.stack[v.sp-2] = destTuple
			v.sp--
		case OpToBool:
			val := v.stack[v.sp-1]
			boolVal := Bool(coerceToBool(val))

			v.stack[v.sp-1] = boolVal
		case OpCreateString:
			v.ip += 4
			typed := v.curInsts[v.ip-3] == 1
			numElements := int(v.curInsts[v.ip-2])
			nodeIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			node := v.constants[nodeIndex].(AstNode).Node.(*parse.StringTemplateLiteral)

			var sliceValues []Value

			for i := v.sp - numElements; i < v.sp; i++ {
				sliceValues = append(sliceValues, v.stack[i])
			}
			v.sp -= numElements

			var val Value
			var err error

			if typed {
				val, err = NewCheckedString(sliceValues, node, v.global.Ctx)
			} else {
				val, err = NewStringFromSlices(sliceValues, node, v.global.Ctx)
			}

			if err != nil {
				v.err = err
				return
			}
			v.stack[v.sp] = val
			v.sp++
		//SELF
		case OpGetSelf:
			v.stack[v.sp] = v.curFrame.self
			v.sp++
		case OpSetSelf:
			v.curFrame.self = v.stack[v.sp-1]
			v.sp--
		//ITERATION AND WALKING
		case OpIterInit:
			v.ip++
			hasConfig := v.curInsts[v.ip]
			var keyPattern, valuePattern Pattern
			if hasConfig == 1 {
				keyPattern, _ = v.stack[v.sp-2].(Pattern)
				valuePattern, _ = v.stack[v.sp-1].(Pattern)
				v.sp -= 2
			}

			dst := v.stack[v.sp-1]
			switch val := dst.(type) {
			case Iterable:
				iterator := val.Iterator(v.global.Ctx, IteratorConfiguration{
					KeyFilter:   keyPattern,
					ValueFilter: valuePattern,
				})
				v.stack[v.sp-1] = iterator
			case StreamSource:
				stream := val.Stream(v.global.Ctx, &ReadableStreamConfiguration{
					Filter: valuePattern,
				})
				v.stack[v.sp-1] = stream
			default:
				panic(ErrUnreachable)
			}
		case OpIterNext:
			v.ip++
			streamElemIndex := int(v.curInsts[v.ip])

			switch val := v.stack[v.sp-1].(type) {
			case Iterator:
				it := val
				hasMore := it.HasNext(v.global.Ctx)
				if hasMore {
					it.Next(v.global.Ctx)
					v.stack[v.sp-1] = True
				} else {
					v.stack[v.sp-1] = False
				}
			case ReadableStream:
				stream := val

				for {
					select {
					case <-v.global.Ctx.Done():
						v.err = v.global.Ctx.Err()
						return
					default:
					}

					next, err := stream.WaitNext(v.global.Ctx, nil, STREAM_ITERATION_WAIT_TIMEOUT)

					if errors.Is(err, ErrEndOfStream) {
						v.stack[v.sp-1] = False
						break
					}

					if errors.Is(err, ErrStreamElemWaitTimeout) {
						continue
					}
					if err != nil {
						v.err = err
						return
					}
					v.stack[streamElemIndex] = next
					v.stack[v.sp-1] = True
					break
				}
			default:
				panic(ErrUnreachable)
			}
		case OpIterNextChunk:
			v.ip++
			streamElemIndex := int(v.curInsts[v.ip])

			switch val := v.stack[v.sp-1].(type) {
			case Iterator:
				panic(errors.New("chunked iteration of iterables not supported yet"))
			case ReadableStream:
				stream := val

				for {
					select {
					case <-v.global.Ctx.Done():
						v.err = v.global.Ctx.Err()
						return
					default:

					}

					if stream.IsStopped() {
						v.stack[v.sp-1] = False
						break
					}

					chunkSizeRange := NewIncludedEndIntRange(DEFAULT_MIN_STREAM_CHUNK_SIZE, DEFAULT_MAX_STREAM_CHUNK_SIZE)
					chunk, err := stream.WaitNextChunk(v.global.Ctx, nil, chunkSizeRange, STREAM_ITERATION_WAIT_TIMEOUT)

					if errors.Is(err, ErrEndOfStream) {
						if chunk != nil { // last chunk
							v.stack[streamElemIndex] = chunk
							v.stack[v.sp-1] = True
							break
						}
						v.stack[v.sp-1] = False
						break
					}
					if errors.Is(err, ErrStreamChunkWaitTimeout) {
						continue
					}
					if err != nil {
						v.err = err
						return
					}
					v.stack[streamElemIndex] = chunk
					v.stack[v.sp-1] = True
					break
				}
			default:
				panic(ErrUnreachable)
			}
		case OpIterKey:
			switch val := v.stack[v.sp-1].(type) {
			case Iterator:
				v.stack[v.sp-1] = val.Key(v.global.Ctx)
			default:
				panic(ErrUnreachable)
			}
		case OpIterValue:
			v.ip++
			streamElemIndex := int(v.curInsts[v.ip])

			switch val := v.stack[v.sp-1].(type) {
			case Iterator:
				v.stack[v.sp-1] = val.Value(v.global.Ctx)
			case ReadableStream:
				v.stack[v.sp-1] = v.stack[streamElemIndex]
				v.stack[streamElemIndex] = Nil
			default:
				panic(ErrUnreachable)
			}
		case OpWalkerInit:
			dst := v.stack[v.sp-1]
			v.sp--
			walker, err := dst.(Walkable).Walker(v.global.Ctx)
			if err != nil {
				v.err = err
				return
			}

			v.stack[v.sp] = walker
			v.sp++
		case OpIterPrune:
			v.ip += 1
			iteratorIndex := int(v.curInsts[v.ip])
			iterator := v.stack[v.curFrame.basePointer+iteratorIndex]
			iterator.(Walker).Prune(v.global.Ctx)
		//OTHER
		case OpGroupMatch:
			v.ip += 2
			localIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8

			left := v.stack[v.sp-2]
			right := v.stack[v.sp-1]
			v.sp -= 2

			pattern := right.(GroupPattern)
			groups, ok, err := pattern.MatchGroups(v.global.Ctx, left.(Serializable))
			if err != nil {
				v.err = err
				return
			}

			if ok {
				v.stack[v.sp] = True
				v.stack[v.curFrame.basePointer+localIndex] = objFrom(groups)
			} else {
				v.stack[v.sp] = False
			}

			v.sp++
		case BindCapturedLocals:
			v.ip++
			numCaptured := int(v.curInsts[v.ip])

			fn := v.stack[v.sp-numCaptured-1].(*InoxFunction)
			newFn := &InoxFunction{
				Node:             fn.Node,
				Chunk:            fn.Chunk,
				compiledFunction: fn.compiledFunction,
			}

			for i := v.sp - numCaptured; i < v.sp; i++ {
				localVal := v.stack[i]
				shared, err := ShareOrClone(localVal, v.global)
				if err != nil {
					v.err = fmt.Errorf("failed to share a capture local: %w", err)
					return
				}
				newFn.capturedLocals = append(newFn.capturedLocals, shared)
			}

			v.sp -= numCaptured
			v.stack[v.sp-1] = newFn
		case OpNoOp:
		case OpSuspendVM:
			return
		default:
			if !v.handleOtherOpcodes(v.curInsts[ip]) {
				if v.err == nil {
					panic(ErrUnreachable)
				}
				return
			}
		}
	}
}

func (v *VM) doSafeIntBinOp() {
	right := v.stack[v.sp-1].(Int)
	left := v.stack[v.sp-2].(Int)
	v.ip++

	operator := parse.BinaryOperator(v.curInsts[v.ip])

	var res Value
	switch operator {
	case parse.Add:
		res, v.err = intAdd(left, right)
		if v.err != nil {
			return
		}
	case parse.Sub:
		res, v.err = intSub(left, right)
		if v.err != nil {
			return
		}
	case parse.Mul:
		if right > 0 {
			if left > math.MaxInt64/right || left < math.MinInt64/right {
				v.err = ErrIntOverflow
				return
			}
		} else if right < 0 {
			if right == -1 {
				if left == math.MinInt64 {
					v.err = ErrIntOverflow
					return
				}
			} else if left < math.MaxInt64/right || left > math.MinInt64/right {
				v.err = ErrIntUnderflow
				return
			}
		}
		res = left * right
	case parse.Div:
		if right == 0 {
			v.err = ErrIntDivisionByZero
			return
		}
		if left == math.MinInt64 && right == -1 {
			v.err = ErrIntOverflow
			return
		}
		res = left / right
	case parse.LessThan:
		res = Bool(left < right)
	case parse.LessOrEqual:
		res = Bool(left <= right)
	case parse.GreaterThan:
		res = Bool(left > right)
	case parse.GreaterOrEqual:
		res = Bool(left >= right)
	default:
		v.err = fmt.Errorf("invalid binary operator")
		return
	}

	v.stack[v.sp-2] = res
	v.sp--
}

func (v *VM) doSafeFloatBinOp() {
	right := v.stack[v.sp-1].(Float)
	left := v.stack[v.sp-2].(Float)
	v.ip++

	if math.IsNaN(float64(left)) || math.IsInf(float64(left), 0) {
		v.err = ErrNaNinfinityOperand
		return
	}

	if math.IsNaN(float64(right)) || math.IsInf(float64(right), 0) {
		v.err = ErrNaNinfinityOperand
		return
	}

	operator := parse.BinaryOperator(v.curInsts[v.ip])

	var res Value
	switch operator {
	case parse.Add:
		res = left + right
	case parse.Sub:
		res = left - right
	case parse.Mul:
		f := left * right
		res = f
		if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
			v.err = ErrNaNinfinityResult
			return
		}
	case parse.Div:
		f := left / right
		res = f
		if math.IsNaN(float64(f)) || math.IsInf(float64(f), 0) {
			v.err = ErrNaNinfinityResult
			return
		}
	case parse.LessThan:
		res = Bool(left < right)
	case parse.LessOrEqual:
		res = Bool(left <= right)
	case parse.GreaterThan:
		res = Bool(left > right)
	case parse.GreaterOrEqual:
		res = Bool(left >= right)
	default:
		v.err = fmt.Errorf("invalid binary operator")
		return
	}

	v.stack[v.sp-2] = res
	v.sp--
}

//go:noinline
func (v *VM) checkComparisonOperands(leftOperand, rightOperand Value) bool {
	leftF, ok := leftOperand.(Float)
	if ok && (math.IsNaN(float64(leftF)) || math.IsInf(float64(leftF), 0)) {
		v.err = ErrNaNinfinityOperand
		return false
	}
	rightF, ok := leftOperand.(Float)

	if ok && (math.IsNaN(float64(rightF)) || math.IsInf(float64(rightF), 0)) {
		v.err = ErrNaNinfinityOperand
		return false
	}

	return true
}

//go:noinline
func (v *VM) handleOtherOpcodes(op byte) (_continue bool) {

	switch op {
	//PATTERN CREATION AND RESOLUTION
	case OpToPattern:
		val := v.stack[v.sp-1].(Serializable)

		if _, ok := val.(Pattern); !ok {
			v.stack[v.sp-1] = NewMostAdaptedExactPattern(val)
		}
	case OpDoSetDifference:
		left := v.stack[v.sp-2]
		right := v.stack[v.sp-1]
		v.sp -= 2

		if _, ok := right.(Pattern); !ok {
			right = NewExactValuePattern(right.(Serializable))
		}
		v.stack[v.sp] = &DifferencePattern{base: left.(Pattern), removed: right.(Pattern)}
		v.sp++
	case OpResolvePattern:
		v.ip += 2
		nameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		name := v.constants[nameIndex].(Str)

		val := v.global.Ctx.ResolveNamedPattern(string(name))
		if val == nil {
			val = &DynamicStringPatternElement{name: string(name), ctx: v.global.Ctx}
		}
		v.stack[v.sp] = val
		v.sp++
	case OpAddPattern:
		v.ip += 2
		nameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		name := v.constants[nameIndex].(Str)

		val := v.stack[v.sp-1].(Pattern)
		v.global.Ctx.AddNamedPattern(string(name), val)
		v.sp--
	case OpResolvePatternNamespace:
		v.ip += 2
		nameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		name := v.constants[nameIndex].(Str)

		val := v.global.Ctx.ResolvePatternNamespace(string(name))
		if val == nil {
			v.err = fmt.Errorf("pattern namespace %%%s is not defined", name)
			return
		}
		v.stack[v.sp] = val
		v.sp++
	case OpAddPatternNamespace:
		v.ip += 2
		nameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		name := v.constants[nameIndex].(Str)

		val := v.stack[v.sp-1].(*PatternNamespace)
		v.global.Ctx.AddPatternNamespace(string(name), val)
		v.sp--
	case OpPatternNamespaceMemb:
		v.ip += 4
		namespaceNameIndex := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
		namespaceName := string(v.constants[namespaceNameIndex].(Str))

		memberNameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		memberName := string(v.constants[memberNameIndex].(Str))

		namespace := v.global.Ctx.ResolvePatternNamespace(namespaceName)
		if namespace == nil {
			v.err = fmt.Errorf("pattern namespace %%%s is not defined", namespaceName)
			return
		}

		patt, ok := namespace.Patterns[memberName]
		if !ok {
			v.err = fmt.Errorf("pattern namespace %s has not a pattern named %s", namespaceName, memberName)
			return
		}

		v.stack[v.sp] = patt
		v.sp++
	case OpCallPattern:
		numArgs := int(v.curInsts[v.ip+1])
		v.ip += 1

		args := make([]Serializable, numArgs)
		for i, arg := range v.stack[v.sp-numArgs : v.sp] {
			args[i] = arg.(Serializable)
		}
		v.sp -= numArgs

		callee := v.stack[v.sp-1].(Pattern)

		patt, err := callee.Call(args)
		if err != nil {
			v.err = err
			return
		}
		v.stack[v.sp-1] = patt
	case OpCreateListPattern:
		v.ip += 3
		numElements := int(v.curInsts[v.ip-1]) | int(v.curInsts[v.ip-2])<<8
		hasGeneralElem := int(v.curInsts[v.ip])

		patt := &ListPattern{}
		if hasGeneralElem == 1 {
			patt.generalElementPattern = v.stack[v.sp-1].(Pattern)
			v.stack[v.sp-1] = patt
		} else {
			patt.elementPatterns = make([]Pattern, 0, numElements)
			for i := v.sp - numElements; i < v.sp; i++ {
				patt.elementPatterns = append(patt.elementPatterns, v.stack[i].(Pattern))
			}
			v.sp -= numElements
			v.stack[v.sp] = patt
			v.sp++
		}
	case OpCreateTuplePattern:
		v.ip += 3
		numElements := int(v.curInsts[v.ip-1]) | int(v.curInsts[v.ip-2])<<8
		hasGeneralElem := int(v.curInsts[v.ip])

		patt := &TuplePattern{}
		if hasGeneralElem == 1 {
			patt.generalElementPattern = v.stack[v.sp-1].(Pattern)
			v.stack[v.sp-1] = patt
		} else {
			patt.elementPatterns = make([]Pattern, 0, numElements)
			for i := v.sp - numElements; i < v.sp; i++ {
				patt.elementPatterns = append(patt.elementPatterns, v.stack[i].(Pattern))
			}
			v.sp -= numElements
			v.stack[v.sp] = patt
			v.sp++
		}
	case OpCreateObjectPattern:
		v.ip += 3
		numElements := int(v.curInsts[v.ip-1]) | int(v.curInsts[v.ip-2])<<8
		isInexact := int(v.curInsts[v.ip])

		var entries []ObjectPatternEntry

		for i := v.sp - numElements; i < v.sp; i += 3 {
			key := v.stack[i].(Str)
			value := v.stack[i+1].(Pattern)
			isOptional := v.stack[i+2].(Bool)

			entries = append(entries, ObjectPatternEntry{
				Name:       string(key),
				Pattern:    value,
				IsOptional: bool(isOptional),
			})
		}

		pattern := &ObjectPattern{
			entries: entries,
			inexact: isInexact == 1,
		}
		pattern.init()

		v.sp -= numElements
		v.stack[v.sp] = pattern
		v.sp++
	case OpCreateRecordPattern:
		v.ip += 3
		numElements := int(v.curInsts[v.ip-1]) | int(v.curInsts[v.ip-2])<<8
		isInexact := int(v.curInsts[v.ip])

		var entries []RecordPatternEntry

		for i := v.sp - numElements; i < v.sp; i += 3 {
			key := v.stack[i].(Str)
			value := v.stack[i+1].(Pattern)
			isOptional := v.stack[i+2].(Bool)

			entries = append(entries, RecordPatternEntry{
				Name:       string(key),
				Pattern:    value,
				IsOptional: bool(isOptional),
			})
		}

		pattern := &RecordPattern{
			entries: entries,
			inexact: isInexact == 1,
		}
		pattern.init()

		v.sp -= numElements
		v.stack[v.sp] = pattern
		v.sp++
	case OpCreateOptionPattern:
		v.ip += 2
		nameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		name := v.constants[nameIndex].(Str)

		value := v.stack[v.sp-1].(Pattern)
		v.stack[v.sp-1] = &OptionPattern{name: string(name), value: value}
	case OpCreateUnionPattern:
		v.ip += 2
		numElements := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8

		var cases []Pattern

		for i := v.sp - numElements; i < v.sp; i++ {
			cases = append(cases, v.stack[i].(Pattern))
		}
		v.sp -= numElements
		patt := &UnionPattern{
			node:  nil,
			cases: cases,
		}

		v.stack[v.sp] = patt
		v.sp++
	case OpCreateStringUnionPattern:
		v.ip += 2
		numElements := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8

		var cases []StringPattern

		for i := v.sp - numElements; i < v.sp; i++ {
			cases = append(cases, v.stack[i].(StringPattern))
		}
		v.sp -= numElements
		patt, err := NewUnionStringPattern(nil, cases)
		if err != nil {
			v.err = err
			return
		}

		v.stack[v.sp] = patt
		v.sp++
	case OpCreateRepeatedPatternElement:
		v.ip += 2
		occurence := parse.OcurrenceCountModifier(v.curInsts[v.ip-1])
		exactOccurenceCount := int(v.curInsts[v.ip])

		patternElement := v.stack[v.sp-1].(StringPattern)

		v.stack[v.sp-1] = &RepeatedPatternElement{
			//regexp:            regexp.MustCompile(subpatternRegex),
			ocurrenceModifier: occurence,
			exactCount:        exactOccurenceCount,
			element:           patternElement,
		}
	case OpCreateSequenceStringPattern:
		v.ip += 5
		numElements := int(v.curInsts[v.ip-4])
		nameListIndex := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
		nameList := v.constants[nameListIndex].(KeyList)
		nodeIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		node := v.constants[nodeIndex].(AstNode)

		var subpatterns []StringPattern

		for i := v.sp - numElements; i < v.sp; i++ {
			subpatterns = append(subpatterns, v.stack[i].(StringPattern))
		}
		v.sp -= numElements

		val, err := NewSequenceStringPattern(node.Node.(*parse.ComplexStringPatternPiece), node.chunk.Node, subpatterns, nameList)
		if err != nil {
			v.err = err
			return
		}
		v.stack[v.sp] = val
		v.sp++
	case OpCreatePatternNamespace:
		init := v.stack[v.sp-1]

		val, err := CreatePatternNamespace(v.global.Ctx, init)
		if err != nil {
			v.err = err
			return
		}
		v.stack[v.sp-1] = val
	case OpCreateOptionalPattern:
		patt := v.stack[v.sp-1].(Pattern)
		val, err := NewOptionalPattern(v.global.Ctx, patt)
		if v.err != nil {
			v.err = err
			return
		}
		v.stack[v.sp-1] = val
	case OpSpreadObjectPattern:
		patt := v.stack[v.sp-2].(*ObjectPattern)
		spreadObjectPatt := v.stack[v.sp-1].(*ObjectPattern)

		for _, entry := range spreadObjectPatt.entries {
			//priority to property pattern defined earlier.
			if patt.HasRequiredOrOptionalEntry(entry.Name) {
				//already present.
				continue
			}

			patt.entries = append(patt.entries, ObjectPatternEntry{
				Name:       entry.Name,
				Pattern:    entry.Pattern,
				IsOptional: entry.IsOptional,
				//ignore dependencies.
			})
		}
		patt.init()
		v.sp--
	case OpSpreadRecordPattern:
		patt := v.stack[v.sp-2].(*RecordPattern)
		spreadRecordPatt := v.stack[v.sp-1].(*RecordPattern)

		for _, entry := range spreadRecordPatt.entries {
			//priority to property pattern defined earlier.
			if patt.HasRequiredOrOptionalEntry(entry.Name) {
				//already present.
				continue
			}

			patt.entries = append(patt.entries, RecordPatternEntry{
				Name:       entry.Name,
				Pattern:    entry.Pattern,
				IsOptional: entry.IsOptional,
			})
		}
		patt.init()
		v.sp--
	//MESSAGING
	case OpCreateReceptionHandler:
		pattern := v.stack[v.sp-2].(Pattern)
		handler, _ := v.stack[v.sp-1].(*InoxFunction)

		v.stack[v.sp-2] = NewSynchronousMessageHandler(v.global.Ctx, handler, pattern)
		v.sp--
	case OpSendValue:
		value := v.stack[v.sp-2]
		receiver, ok := v.stack[v.sp-1].(MessageReceiver)

		if v.curFrame.self == nil {
			v.err = ErrSelfNotDefined
			return
		}

		if ok {
			if err := SendVal(v.global.Ctx, value, receiver, v.curFrame.self); err != nil {
				v.err = err
				return
			}
		}
		v.sp -= 1
		v.stack[v.sp-1] = Nil
	//CHILD CHUNK
	case OpPushIncludedChunk:
		importIp := v.ip
		v.ip += 2
		inclusionStmtIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		inclusionStmt := v.constants[inclusionStmtIndex].(AstNode).Node.(*parse.InclusionImportStatement)

		chunk, ok := v.module.InclusionStatementMap[inclusionStmt]
		if !ok {
			panic(ErrUnreachable)
		}
		v.chunkStack[len(v.chunkStack)-1].CurrentNodeSpan = v.curFrame.fn.GetSourcePositionRange(importIp).Span
		v.chunkStack = append(v.chunkStack, &parse.ChunkStackItem{
			Chunk: chunk.ParsedChunk,
		})
	case OpPopIncludedChunk:
		v.chunkStack = v.chunkStack[:len(v.chunkStack)-1]
		v.chunkStack[len(v.chunkStack)-1].CurrentNodeSpan = parse.NodeSpan{}
	//XML
	case OpCreateXMLelem:
		v.ip += 6
		tagNameIndex := int(v.curInsts[v.ip-4]) | int(v.curInsts[v.ip-5])<<8
		attributeCount := int(v.curInsts[v.ip-3])
		rawContentIndex := int(v.curInsts[v.ip-1]) | int(v.curInsts[v.ip-2])<<8
		rawContent := v.constants[rawContentIndex]
		childCount := int(v.curInsts[v.ip])
		tagName := string(v.constants[tagNameIndex].(Str))

		var attributes []XMLAttribute
		if attributeCount > 0 {
			attributes = make([]XMLAttribute, attributeCount)

			attributesStart := v.sp - childCount - 2*attributeCount
			for i := 0; i < 2*attributeCount; i += 2 {
				attributes[i/2] = XMLAttribute{
					name:  string(v.stack[attributesStart+i].(Str)),
					value: v.stack[attributesStart+i+1],
				}
			}
		}

		var elem *XMLElement

		if rawContent != Nil {
			elem = NewRawTextXmlElement(tagName, attributes, string(rawContent.(Str)))
		} else {
			childrenStart := v.sp - childCount
			var children []Value
			if childCount > 0 {
				children = make([]Value, childCount)
				copy(children, v.stack[childrenStart:v.sp])
			}
			elem = NewXmlElement(tagName, attributes, children)
		}

		v.sp -= (childCount + 2*attributeCount)
		v.stack[v.sp] = elem
		v.sp++
	case OpCallFromXMLFactory:
		xmlElem := v.stack[v.sp-2].(*XMLElement)

		ns := v.stack[v.sp-1].(*Namespace)
		factory := ns.Prop(v.global.Ctx, symbolic.FROM_XML_FACTORY_NAME).(*GoFunction)

		v.sp--

		result, err := factory.Call([]any{xmlElem}, v.global, nil, false, false)
		if err != nil {
			v.err = err
			return
		}

		v.stack[v.sp-1] = result
	//RESOURCE NAMES
	case OpCreatePath:
		v.ip += 3
		argCount := int(v.curInsts[v.ip-2])
		listIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8

		var args []Value

		var isStaticPathSliceList []bool
		_isStaticPathSliceList := v.constants[listIndex].(*List)
		for _, e := range _isStaticPathSliceList.GetOrBuildElements(v.global.Ctx) {
			isStaticPathSliceList = append(isStaticPathSliceList, bool(e.(Bool)))
		}

		for i := v.sp - argCount; i < v.sp; i++ {
			args = append(args, v.stack[i])
		}
		v.sp -= argCount

		val, err := NewPath(args, isStaticPathSliceList)
		if err != nil {
			v.err = err
			return
		}
		v.stack[v.sp] = val
		v.sp++
	case OpCreatePathPattern:
		v.ip += 3
		argCount := int(v.curInsts[v.ip-2])
		listIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8

		var args []Value

		var isStaticPathSliceList []bool
		_isStaticPathSliceList := v.constants[listIndex].(*List)
		for _, e := range _isStaticPathSliceList.GetOrBuildElements(v.global.Ctx) {
			isStaticPathSliceList = append(isStaticPathSliceList, bool(e.(Bool)))
		}

		for i := v.sp - argCount; i < v.sp; i++ {
			args = append(args, v.stack[i])
		}
		v.sp -= argCount

		val, err := NewPathPattern(args, isStaticPathSliceList)
		if err != nil {
			v.err = err
			return
		}
		v.stack[v.sp] = val
		v.sp++
	case OpCreateURL:
		v.ip += 2
		infoIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		info := v.constants[infoIndex].(*Record)
		pathSliceCount := int(info.Prop(v.global.Ctx, "path-slice-count").(Int))
		queryParamInfo := info.Prop(v.global.Ctx, "query-params").(*Tuple)
		staticPathSlices := info.Prop(v.global.Ctx, "static-path-slices").(*Tuple)

		var pathSlices []Value
		var isStaticPathSliceList []bool
		var queryParamNames []Value
		var queryValues []Value

		//query
		queryParamCount := queryParamInfo.Len() / 2

		for i := 0; i < queryParamCount; i++ {
			queryParamNames = append(queryParamNames, queryParamInfo.At(v.global.Ctx, 2*i).(Str))
		}

		for i := v.sp - queryParamCount; i < v.sp; i++ {
			queryValues = append(queryValues, v.stack[i])
		}
		v.sp -= queryParamCount

		///path
		for _, e := range staticPathSlices.elements {
			isStaticPathSliceList = append(isStaticPathSliceList, bool(e.(Bool)))
		}

		for i := v.sp - pathSliceCount; i < v.sp; i++ {
			pathSlices = append(pathSlices, v.stack[i])
		}
		v.sp -= pathSliceCount
		v.sp--

		//host
		host := v.stack[v.sp]

		val, err := NewURL(host, pathSlices, isStaticPathSliceList, queryParamNames, queryValues)
		if err != nil {
			v.err = err
			return
		}
		v.stack[v.sp] = val
		v.sp++
	case OpCreateHost:
		v.ip += 2
		schemeIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		scheme := v.constants[schemeIndex].(Str)

		hostnamePort := v.stack[v.sp-1].(Str)
		val, err := NewHost(hostnamePort, string(scheme))
		if err != nil {
			v.err = err
			return
		}
		v.stack[v.sp-1] = val
	//RANGES
	case OpCreateRuneRange:
		lower := v.stack[v.sp-2].(Rune)
		upper := v.stack[v.sp-1].(Rune)
		v.stack[v.sp-2] = RuneRange{Start: rune(lower), End: rune(upper)}
		v.sp--
	case OpCreateIntRange:
		lower := int64(v.stack[v.sp-2].(Int))
		upper := int64(v.stack[v.sp-1].(Int))
		v.stack[v.sp-2] = IntRange{
			unknownStart: false,
			inclusiveEnd: true,
			start:        lower,
			end:          upper,
			step:         1,
		}
		v.sp--
	case OpCreateFloatRange:
		lower := float64(v.stack[v.sp-2].(Float))
		upper := float64(v.stack[v.sp-1].(Float))
		v.stack[v.sp-2] = FloatRange{
			unknownStart: false,
			inclusiveEnd: true,
			start:        lower,
			end:          upper,
		}
		v.sp--
	case OpCreateUpperBoundRange:
		upperBound := v.stack[v.sp-1]

		switch val := upperBound.(type) {
		case Int:
			v.stack[v.sp-1] = IntRange{
				unknownStart: true,
				inclusiveEnd: true,
				end:          int64(val),
				step:         1,
			}
		case Float:
			v.stack[v.sp-1] = FloatRange{
				unknownStart: true,
				inclusiveEnd: true,
				end:          float64(val),
			}
		default:
			v.stack[v.sp-1] = QuantityRange{
				unknownStart: true,
				inclusiveEnd: true,
				end:          val.(Serializable),
			}
		}
	//TESTING
	case OpPopJumpIfTestDisabled:
		v.ip += 2
		testItem := v.stack[v.sp-1].(TestItem)

		if enabled, _ := v.global.TestingState.Filters.IsTestEnabled(testItem, v.global); !enabled {
			pos := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8

			v.stack[v.sp-1] = Nil
			v.sp--
			v.ip = pos
		}
	case OpCreateTestSuite:
		v.ip += 4
		nodeIndex := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
		node := v.constants[nodeIndex].(AstNode)
		parentChunk := node.chunk

		embeddedChunkNodeIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		embeddedChunkNode := v.constants[embeddedChunkNodeIndex].(AstNode)

		embeddedModChunk := embeddedChunkNode.Node.(*parse.Chunk)
		meta := v.stack[v.sp-1]

		suite, err := NewTestSuite(TestSuiteCreationInput{
			Meta:             meta,
			Node:             node.Node.(*parse.TestSuiteExpression),
			EmbeddedModChunk: embeddedModChunk,
			ParentChunk:      parentChunk,
			ParentState:      v.global,
		})
		if err != nil {
			v.err = err
			return
		}
		v.stack[v.sp-1] = suite
	case OpCreateTestCase:
		v.ip += 4
		nodeIndex := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
		node := v.constants[nodeIndex].(AstNode)

		modNodeIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		modNode := v.constants[modNodeIndex].(AstNode)

		embeddedModChunk := modNode.Node.(*parse.Chunk)
		parentChunk := node.chunk
		meta := v.stack[v.sp-1]

		//TODO: add location to test case
		suite, err := NewTestCase(TestCaseCreationInput{
			Meta: meta,
			Node: node.Node.(*parse.TestCaseExpression),

			ModChunk:          embeddedModChunk,
			ParentState:       v.global,
			ParentChunk:       parentChunk,
			PositionStack:     nil,
			FormattedLocation: "",
		})
		if err != nil {
			v.err = err
			return
		}
		v.stack[v.sp-1] = suite
	case OpAddTestSuiteResult:
		lthread := v.stack[v.sp-2].(*LThread)
		testSuite := v.stack[v.sp-3].(*TestSuite)
		//v.stack[v.sp-1] : result (unused)

		//create test result and add it to .TestSuiteResults.
		err := func() error {
			if !lthread.state.TestingState.ResultsLock.TryLock() {
				return errors.New("test results should not be locked")
			}
			defer lthread.state.TestingState.ResultsLock.Unlock()

			testCaseResults := lthread.state.TestingState.CaseResults
			testSuiteResults := lthread.state.TestingState.SuiteResults

			result, err := NewTestSuiteResult(v.global.Ctx, testCaseResults, testSuiteResults, testSuite)
			if err != nil {
				return err
			}

			v.global.TestingState.ResultsLock.Lock()
			defer v.global.TestingState.ResultsLock.Unlock()

			v.global.TestingState.SuiteResults = append(v.global.TestingState.SuiteResults, result)
			return nil
		}()

		if err != nil {
			v.err = err
			return
		}

		v.stack[v.sp-1] = Nil
		v.stack[v.sp-2] = Nil
		v.stack[v.sp-3] = Nil
		v.sp -= 3
	case OpAddTestCaseResult:
		v.err = ErrNotImplementedYet
		return
		{
			//TODO
			// lthread := v.stack[v.sp-2].(*LThread)
			// testCase := v.stack[v.sp-3].(*TestCase)
			// //v.stack[v.sp-1] : result (unused)

			// if v.global.Module.ModuleKind == TestSuiteModule {
			// 	//create test result and add it to .TestSuiteResults.
			// 	err := func() error {
			// 		lthread.state.TestingState.TestResultsLock.Lock()
			// 		defer lthread.state.TestingState.TestResultsLock.Unlock()

			// 		testCaseResults := lthread.state.TestingState.TestCaseResults
			// 		testSuiteResults := lthread.state.TestingState.TestSuiteResults

			// 		result, err := NewTestCaseResult(v.global.Ctx, testCase)
			// 		if err != nil {
			// 			return err
			// 		}

			// 		v.global.TestSuiteResults = append(v.global.TestSuiteResults, result)
			// 		return nil
			// 	}()

			// 	if err != nil {
			// 		v.err = err
			// 		return
			// 	}
			// }

			// v.sp -= 3
		}
	//HOST ALIAS
	case OpResolveHost:
		v.ip += 2
		aliasIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		aliasName := string(v.constants[aliasIndex].(Str))[1:]
		val := v.global.Ctx.ResolveHostAlias(aliasName)
		v.stack[v.sp] = val
		v.sp++
	case OpAddHostAlias:
		v.ip += 2
		aliasIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		aliasName := string(v.constants[aliasIndex].(Str))[1:]
		val := v.stack[v.sp-1].(Host)
		v.sp--

		v.global.Ctx.AddHostAlias(aliasName, val)
	//CHILD MODULE
	case OpImport:
		v.ip += 2
		globalNameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		globalName := v.constants[globalNameIndex].(Str)

		source := v.stack[v.sp-2]
		configObject := v.stack[v.sp-1].(*Object)

		varPerm := GlobalVarPermission{permkind.Create, string(globalName)}
		if err := v.global.Ctx.CheckHasPermission(varPerm); err != nil {
			v.err = fmt.Errorf("import: %s", err.Error())
			return
		}

		config, err := buildImportConfig(configObject, source.(ResourceName), v.global)
		if err != nil {
			v.err = err
			return
		}

		result, err := ImportWaitModule(config)
		if err != nil {
			v.err = err
			return
		}

		v.sp -= 2
		v.global.Globals.Set(string(globalName), result)
	case OpCreateLifetimeJob:
		v.ip += 4
		modIndex := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
		mod := v.constants[modIndex].(*Module)

		bytecodeIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		bytecode := v.constants[bytecodeIndex].(*Bytecode)

		meta := v.stack[v.sp-2]
		subject, _ := v.stack[v.sp-1].(Pattern)

		job, err := NewLifetimeJob(meta, subject, mod, bytecode, v.global)
		if err != nil {
			v.err = err
			return
		}
		v.stack[v.sp-2] = job
		v.sp--
	case OpSpawnLThread:
		v.ip += 7
		isSingleExpr := v.curInsts[v.ip-6]
		calleeNameindex := int(v.curInsts[v.ip-4]) | int(v.curInsts[v.ip-5])<<8
		caleeName := v.constants[calleeNameindex].(Str)

		lthreadModConstantIndex := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
		lthreadMod := v.constants[lthreadModConstantIndex].(*Module)

		lthreadBytecodeConstantIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		lthreadBytecode := v.constants[lthreadBytecodeConstantIndex].(*Bytecode)

		meta := v.stack[v.sp-2]
		singleExprCallee := v.stack[v.sp-1]

		var (
			group       *LThreadGroup
			globalsDesc Value
			permListing *Object
		)

		if meta != nil && meta != Nil {
			metaMap := meta.(*ModuleArgs).ValueMap()

			group, globalsDesc, permListing, v.err = readLThreadMeta(metaMap, v.global.Ctx)
			if v.err != nil {
				return
			}
		}
		actualGlobals := make(map[string]Value)
		var startConstants []string

		//pass constant globals

		v.global.Globals.Foreach(func(name string, v Value, isConstant bool) error {
			if isConstant {
				actualGlobals[name] = v
				startConstants = append(startConstants, name)
			}
			return nil
		})

		var ctx *Context

		// pass global variables

		switch g := globalsDesc.(type) {
		case *ModuleArgs:
			for i, v := range g.values {
				k := g.structType.keys[i]
				actualGlobals[k] = v
			}
		case KeyList:
			for _, name := range g {
				actualGlobals[name] = v.global.Globals.Get(name)
			}
		case NilT:
			break
		case nil:
		default:
			v.err = fmt.Errorf("spawn expression: globals: only objects and keylists are supported, not %T", g)
			return
		}

		if isSingleExpr == 1 {
			actualGlobals[string(caleeName)] = singleExprCallee
		}

		//create context
		if permListing != nil {
			perms, err := getPermissionsFromListing(v.global.Ctx, permListing, nil, nil, true)
			if err != nil {
				v.err = fmt.Errorf("spawn expression: %w", err)
				return
			}

			for _, perm := range perms {
				if err := v.global.Ctx.CheckHasPermission(perm); err != nil {
					v.err = fmt.Errorf("spawn: cannot allow permission: %w", err)
					return
				}
			}
			ctx = NewContext(ContextConfig{
				Permissions:          perms,
				ForbiddenPermissions: v.global.Ctx.forbiddenPermissions,
				ParentContext:        v.global.Ctx,
			})
		} else {
			removedPerms := IMPLICITLY_REMOVED_ROUTINE_PERMS
			remainingPerms := RemovePerms(v.global.Ctx.GetGrantedPermissions(), IMPLICITLY_REMOVED_ROUTINE_PERMS)

			ctx = NewContext(ContextConfig{
				ParentContext:        v.global.Ctx,
				Permissions:          remainingPerms,
				ForbiddenPermissions: removedPerms,
			})
		}

		lthread, err := SpawnLThread(LthreadSpawnArgs{
			SpawnerState: v.global,
			Globals:      GlobalVariablesFromMap(actualGlobals, startConstants),
			Module:       lthreadMod,
			Bytecode:     lthreadBytecode,
			LthreadCtx:   ctx,
			UseBytecode:  true,
		})

		if err != nil {
			v.err = err
			return
		}

		if group != nil {
			group.Add(lthread)
		}

		v.sp -= 1
		v.stack[v.sp-1] = lthread
		// isCall := v.curInsts[v.ip] == 1

		// groupVal := v.stack[v.sp-4]
		// globalDesc := v.stack[v.sp-3]

		// upper := v.stack[v.sp-1].(Rune)
	//MISCELLANEOUS
	case OpLoadDBVal:
		url := v.stack[v.sp-1].(URL)

		value, err := getOrLoadValueAtURL(v.global.Ctx, url, v.global)
		if err != nil {
			v.err = err
			return
		}

		v.stack[v.sp-1] = value
	case OpCreateKeyList:
		v.ip += 2
		numElements := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		keyList := make(KeyList, 0, numElements)

		for i := v.sp - numElements; i < v.sp; i++ {
			keyList = append(keyList, string(v.stack[i].(Identifier)))
		}
		v.sp -= numElements

		v.stack[v.sp] = keyList
		v.sp++
	case OptStrQueryParamVal:
		val, err := stringifyQueryParamValue(v.stack[v.sp-1])
		if err != nil {
			v.err = err
			return
		}
		v.stack[v.sp-1] = Str(val)
	case OpCreateAddTypeExtension:
		v.ip += 2
		extendStmtIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		extendStmt := v.constants[extendStmtIndex].(AstNode).Node.(*parse.ExtendStatement)

		extendedPattern := v.stack[v.sp-2].(Pattern)
		methodList := v.stack[v.sp-1].(*List)

		lastCtxData, ok := v.global.SymbolicData.GetContextData(extendStmt, nil)
		if !ok {
			panic(ErrUnreachable)
		}
		symbolicExtension := lastCtxData.Extensions[len(lastCtxData.Extensions)-1]

		if symbolicExtension.Statement != extendStmt {
			panic(ErrUnreachable)
		}

		extension := &TypeExtension{
			extendedPattern:   extendedPattern,
			symbolicExtension: symbolicExtension,
		}

		for _, symbolicPropExpr := range symbolicExtension.PropertyExpressions {
			if symbolicPropExpr.Expression != nil {
				extension.propertyExpressions = append(extension.propertyExpressions, propertyExpression{
					name:       symbolicPropExpr.Name,
					expression: symbolicPropExpr.Expression,
				})
			}
		}

		methodIndex := 0
		for _, prop := range extendStmt.Extension.(*parse.ObjectLiteral).Properties {
			_, ok := prop.Value.(*parse.FunctionExpression)

			if !ok {
				continue
			}

			extension.propertyExpressions = append(extension.propertyExpressions, propertyExpression{
				name:   prop.Name(),
				method: methodList.At(v.global.Ctx, methodIndex).(*InoxFunction),
			})
			methodIndex++
		}

		v.global.Ctx.AddTypeExtension(extension)
		v.sp -= 2
	case OpAllocStruct:
		v.ip += 4
		size := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
		alignment := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8

		addr := Alloc[byte](v.global.Heap, size, alignment)
		v.stack[v.sp] = (*Struct)(unsafe.Pointer(addr))
		v.sp++
	case OpRuntimeTypecheck:
		v.ip += 2
		astNodeIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		astNode := v.constants[astNodeIndex].(AstNode).Node.(*parse.RuntimeTypeCheckExpression)

		pattern, ok := v.global.SymbolicData.GetRuntimeTypecheckPattern(astNode)
		if !ok {
			v.err = ErrMissinggRuntimeTypecheckSymbData
			return
		}
		if pattern != nil { //enabled
			patt := pattern.(Pattern)
			val := v.stack[v.sp-1]

			if !patt.Test(v.global.Ctx, val) {
				v.err = FormatRuntimeTypeCheckFailed(patt, v.global.Ctx)
				return
			}
		}
		//keep the value on top of the stack
	case OpAssert:
		v.ip += 2
		nodeIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		stmt := v.constants[nodeIndex].(AstNode).Node.(*parse.AssertionStatement)

		ok := v.stack[v.sp-1].(Bool)
		v.sp--

		modKind := v.global.Module.ModuleKind
		isTestAssertion := modKind == TestSuiteModule || modKind == TestCaseModule
		var testModule *Module
		if isTestAssertion {
			testModule = v.global.Module
		}

		if !ok {
			data := &AssertionData{
				assertionStatement: stmt,
				intermediaryValues: map[parse.Node]Value{},
			}
			v.err = &AssertionError{
				msg:             "assertion is false",
				data:            data,
				isTestAssertion: isTestAssertion,
				testModule:      testModule,
			}
			return
		}
	case OpCreateOption:
		v.ip += 2
		nameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		name := v.constants[nameIndex].(Str)

		value := v.stack[v.sp-1]
		v.stack[v.sp-1] = Option{Name: string(name), Value: value}
	case OpDynMemb:
		object := v.stack[v.sp-1]
		v.ip += 2
		memberNameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
		memberName := string(v.constants[memberNameIndex].(Str))

		val, err := NewDynamicMemberValue(v.global.Ctx, object, string(memberName))
		if err != nil {
			v.err = err
			return
		}
		v.stack[v.sp-1] = val
	case OpYield:
		v.ip++
		var retVal Value
		isValOnStack := int(v.curInsts[v.ip]) == 1
		if isValOnStack {
			retVal = v.stack[v.sp-1]
			v.sp--
		} else {
			retVal = Nil
		}

		if v.global.LThread == nil {
			v.err = errors.New("failed to yield: no associated lthread")
			return
		}
		v.global.LThread.yield(v.global.Ctx, retVal)
	case OpBlockLock:
		v.ip++
		numValues := int(v.curInsts[v.ip])
		for _, val := range v.stack[v.sp-numValues : v.sp] {
			if !val.IsMutable() {
				continue
			}
			potentiallySharable := val.(PotentiallySharable)
			if !utils.Ret0(potentiallySharable.IsSharable(v.global)) {
				v.err = ErrCannotLockUnsharableValue
				return
			}

			for _, locked := range v.global.lockedValues {
				if potentiallySharable == locked {
					continue
				}
			}

			potentiallySharable.Share(v.global)
			potentiallySharable.ForceLock()

			// update list of locked values
			v.global.lockedValues = append(v.global.lockedValues, potentiallySharable)
			v.curFrame.lockedValues = append(v.curFrame.lockedValues, potentiallySharable)
		}
		v.sp -= numValues
	case OpBlockUnlock:
		lockedValues := v.curFrame.lockedValues
		v.curFrame.lockedValues = nil

		for i := len(lockedValues) - 1; i >= 0; i-- {
			locked := lockedValues[i]
			locked.ForceUnlock()
		}

		var newLockedValues []PotentiallySharable
		// update list of locked values
	loop:
		for _, lockedVal := range v.global.lockedValues {
			for _, unlockedVal := range lockedValues {
				if lockedVal == unlockedVal {
					continue loop
				}
			}
			newLockedValues = append(newLockedValues, lockedVal)
		}
		v.global.lockedValues = newLockedValues
	case OpDropPerms:
		permListing := v.stack[v.sp-1].(*Object)
		v.sp--

		//TODO: check listing ?

		perms, err := getPermissionsFromListing(v.global.Ctx, permListing, nil, nil, false)
		if err != nil {
			v.err = err
			return
		}

		v.global.Ctx.DropPermissions(perms)
	default:
		v.err = fmt.Errorf("unknown opcode: %d (at %d)", v.curInsts[v.ip], v.ip)
		return
	}
	_continue = true
	return
}

//go:noinline
func (v *VM) fnCall(numArgs int, spread, must bool, callIp int) bool {
	var (
		selfVal         = v.stack[v.sp-2]
		callee          = v.stack[v.sp-1]
		extState        *GlobalState
		extBytecode     *Bytecode
		capturedGlobals []capturedGlobal
	)

	isSharedFunction := false

	if inoxFn, ok := callee.(*InoxFunction); ok {
		isSharedFunction = inoxFn.IsShared()
		capturedGlobals = inoxFn.capturedGlobals

		if isSharedFunction {
			extState = inoxFn.originState
			extBytecode = extState.Bytecode
		}

	} else {
		goFn := callee.(*GoFunction)
		isSharedFunction = goFn.IsShared()
		if isSharedFunction {
			extState = goFn.originState
			extBytecode = extState.Bytecode
		}
	}

	//remove the callee and the object from the stack.
	v.stack[v.sp-1] = Nil
	v.stack[v.sp-2] = Nil
	v.sp -= 2

	var spreadArg *Array
	var passedSpreadArg Iterable

	if spread {
		passedSpreadArg = v.stack[v.sp-1].(Iterable)
		v.sp--
	}

	if isSharedFunction { //share arguments

		for i := v.sp - numArgs; i < v.sp; i++ {
			if len(v.disabledArgSharing) > i && v.disabledArgSharing[i] {
				continue
			}
			shared, err := ShareOrClone(v.stack[i], v.global)
			if err != nil {
				v.err = fmt.Errorf("failed to share an argument: %T: %w", v.stack[i], err)
				return false
			}
			v.stack[i] = shared
		}

		if spread {
			spreadArg = &Array{}
			it := passedSpreadArg.Iterator(v.global.Ctx, IteratorConfiguration{KeysNeverRead: true})

			for it.Next(v.global.Ctx) {
				elem := it.Value(v.global.Ctx)

				shared, err := ShareOrClone(elem, v.global)
				if err != nil {
					v.err = fmt.Errorf("failed to share an element of a spread argument: %T: %w", elem, err)
					return false
				}
				*spreadArg = append(*spreadArg, shared)
			}
		}
	} else if spread {
		spreadArg = &Array{}
		it := passedSpreadArg.Iterator(v.global.Ctx, IteratorConfiguration{KeysNeverRead: true})

		for it.Next(v.global.Ctx) {
			elem := it.Value(v.global.Ctx)
			*spreadArg = append(*spreadArg, elem)
		}
	}

	if InoxFunction, ok := callee.(*InoxFunction); ok {
		compiled := InoxFunction.compiledFunction
		if compiled.IsVariadic {
			// if the closure is variadic, roll up all variadic parameters into a List
			realArgs := compiled.ParamCount - 1
			varArgs := numArgs - realArgs
			if varArgs >= 0 {
				numArgs = realArgs + 1
				args := make(Array, varArgs)
				spStart := v.sp - varArgs
				for i := spStart; i < v.sp; i++ {
					args[i-spStart] = v.stack[i]
				}
				if spreadArg != nil {
					for _, e := range *spreadArg {
						args = append(args, e)
					}
				}

				v.stack[spStart] = &args
				v.sp = spStart + 1
			}
		}

		if numArgs != compiled.ParamCount {
			if compiled.IsVariadic {
				v.err = fmt.Errorf("wrong number of arguments: %d are wanted but got%d", compiled.ParamCount-1, numArgs)
			} else {
				v.err = fmt.Errorf("wrong number of arguments: %d are wanted but got %d", compiled.ParamCount, numArgs)
			}
			return false
		}

		if len(InoxFunction.capturedLocals) > 0 {
			for i := v.sp; i < v.sp+len(InoxFunction.capturedLocals); i++ {
				val := InoxFunction.capturedLocals[i-v.sp]
				// TODO: already shared ?

				if isSharedFunction {
					shared, err := ShareOrClone(val, v.global)
					if err != nil {
						v.err = fmt.Errorf("failed to share a captured local: %w", err)
						return false
					}
					val = shared
				}
				v.stack[i] = val
			}
		}

		//TODO?: tail call

		if v.framesIndex >= MAX_FRAMES {
			v.err = ErrStackOverflow
			return false
		}

		//update parent call frame
		if callIp >= 0 && !v.runFn {
			v.curFrame.currentNodeSpan = v.curFrame.fn.GetSourcePositionRange(callIp).Span
			v.chunkStack[len(v.chunkStack)-1].CurrentNodeSpan = v.curFrame.currentNodeSpan
			//TODO: support isolated function call
		}

		// update call frame
		v.curFrame.ip = v.ip
		v.curFrame = &(v.frames[v.framesIndex])
		v.curFrame.self = selfVal
		v.curFrame.externalFunc = isSharedFunction
		v.curFrame.fn = compiled
		v.curFrame.basePointer = v.sp - numArgs
		v.curFrame.lockedValues = nil
		v.curFrame.mustCall = must
		v.curInsts = compiled.Instructions

		if capturedGlobals != nil {
			v.global.Globals.PushCapturedGlobals(capturedGlobals)
			v.curFrame.popCapturedGlobals = true
		}

		if isSharedFunction {
			v.curFrame.originalConstants = v.constants
			v.constants = extBytecode.constants
			v.curFrame.bytecode = extBytecode
		} else {
			v.curFrame.originalConstants = nil
			v.curFrame.externalFunc = false
		}

		v.ip = -1
		v.framesIndex++
		v.sp = v.sp - numArgs + compiled.LocalCount
	} else { //Go function
		var args []any
		for _, arg := range v.stack[v.sp-numArgs : v.sp] {
			args = append(args, arg)
		}
		if spreadArg != nil {
			for _, arg := range *spreadArg {
				args = append(args, arg)
			}
		}

		goFunc := callee.(*GoFunction)

		ret, err := goFunc.Call(args, v.global, extState, isSharedFunction, must)
		if err != nil {
			v.err = err
			return false
		}

		//
		v.sp -= numArgs + 1

		// nil return -> undefined
		if ret == nil {
			ret = Nil
		}
		v.stack[v.sp] = ret
		v.sp++
	}

	return true
}

func (v *VM) IsStackEmpty() bool {
	return v.sp == 0
}
