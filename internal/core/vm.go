package core

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"runtime/debug"
	"strings"
	"sync/atomic"

	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	permkind "github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	VM_STACK_SIZE = 200
	MAX_FRAMES    = 20
)

var (
	ErrArgsProvidedToModule    = errors.New("cannot provide arguments when running module")
	ErrInvalidProvidedArgCount = errors.New("number of provided arguments is invalid")
)

// VM is a virtual machine that executes bytecode.
type VM struct {
	constants        []Value
	stack            [VM_STACK_SIZE]Value
	sp               int
	global           *GlobalState
	module           *Module
	frames           [MAX_FRAMES]frame
	framesIndex      int
	curFrame         *frame
	curInsts         []byte
	ip               int
	aborting         int64
	err              error
	moduleLocalCount int

	runFn              bool
	fnArgCount         int
	disabledArgSharing []bool
}

// frame represents a call frame.
type frame struct {
	fn           *CompiledFunction
	bytecode     *Bytecode
	ip           int
	basePointer  int
	self         Value
	lockedValues []PotentiallySharable

	externalFunc       bool
	popCapturedGlobals bool
	originalConstants  []Value
}

type VMConfig struct {
	Bytecode *Bytecode
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
		if !v.fnCall(v.fnArgCount, false, false) {
			return nil, v.err
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
			sourcePos := v.curFrame.fn.GetSourcePositionRange(ip)
			positionStack := parse.SourcePositionStack{sourcePos}
			if sourcePos.SourceName != "" {
				locationPartBuff := bytes.NewBuffer(nil)

				locationPartBuff.Write(utils.StringAsBytes(sourcePos.String()))
				locationPartBuff.WriteByte(' ')

				frameIndex := v.framesIndex
				var frame *frame
				for frameIndex > 1 {
					frameIndex--
					frame = &v.frames[frameIndex-1]
					sourcePos = frame.fn.GetSourcePositionRange(frame.ip - 1)
					positionStack = append(positionStack, sourcePos)

					locationPartBuff.Write(utils.StringAsBytes(sourcePos.String()))
					locationPartBuff.WriteByte(' ')
				}

				location := locationPartBuff.String()
				if assertionErr != nil {
					assertionErr.msg = location + " " + assertionErr.msg
				}

				v.err = LocatedEvalError{
					error:    fmt.Errorf("%s %w", location, v.err),
					Message:  v.err.Error(),
					Location: positionStack,
				}
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

	doIntBinOp := func() {
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

	doFloatBinOp := func() {
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
		case OpPushConstant:
			v.ip += 2
			cidx := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8

			v.stack[v.sp] = v.constants[cidx]
			v.sp++
		case OpPushNil:
			v.stack[v.sp] = Nil
			v.sp++
		case OpIntBin:
			doIntBinOp()
			if v.err != nil {
				return
			}
		case OpFloatBin:
			doFloatBinOp()
			if v.err != nil {
				return
			}
		case OpNumBin:
			if _, ok := v.stack[v.sp-2].(Int); ok {
				doIntBinOp()
			} else {
				doFloatBinOp()
			}
			if v.err != nil {
				return
			}
		case OpStrConcat:
			right := v.stack[v.sp-1]
			left := v.stack[v.sp-2]
			res := Str(left.(WrappedString).UnderlyingString() + right.(WrappedString).UnderlyingString())

			v.stack[v.sp-2] = res
			v.sp--
		case OptStrQueryParamVal:
			val, err := stringifyQueryParamValue(v.stack[v.sp-1])
			if err != nil {
				v.err = err
				return
			}
			v.stack[v.sp-1] = Str(val)
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
					Start:        int64(left.(Int)),
					End:          int64(right.(Int)),
					Step:         1,
				}
			case Float:
				v.err = fmt.Errorf("floating point ranges not supported")
				return
			default:
				res = QuantityRange{
					inclusiveEnd: !exclEnd,
					Start:        left.(Serializable),
					End:          right.(Serializable),
				}
			}
			v.stack[v.sp-2] = res
			v.sp--
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
		case OpPop:
			v.sp--
		case OpCopyTop:
			v.stack[v.sp] = v.stack[v.sp-1]
			v.sp++
		case OpSwap:
			temp := v.stack[v.sp-1]
			v.stack[v.sp-1] = v.stack[v.sp-2]
			v.stack[v.sp-2] = temp
		case OpPushTrue:
			v.stack[v.sp] = True
			v.sp++
		case OpPushFalse:
			v.stack[v.sp] = False
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
		case OpMinus:
			operand := v.stack[v.sp-1]
			v.sp--

			switch x := operand.(type) {
			case Int:
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
		case OpIn:
			left := v.stack[v.sp-2]
			right := v.stack[v.sp-1]
			v.sp -= 2

			var val Value

			switch rightVal := right.(type) {
			case *List:
				it := rightVal.Iterator(v.global.Ctx, IteratorConfiguration{})
				for it.Next(v.global.Ctx) {
					e := it.Value(v.global.Ctx)
					if left.Equal(v.global.Ctx, e, map[uintptr]uintptr{}, 0) {
						val = True
					}
				}
			case *Object:
				for _, _v := range rightVal.values {
					if left.Equal(v.global.Ctx, _v, map[uintptr]uintptr{}, 0) {
						val = True
					}
				}
			default:
				v.err = fmt.Errorf("invalid binary expression: cannot check if value is inside a %T", rightVal)
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

			val := strings.Contains(left.(WrappedString).UnderlyingString(), right.(WrappedString).UnderlyingString())

			v.stack[v.sp] = Bool(val)
			v.sp++
		case OpKeyOf:
			left := v.stack[v.sp-2].(Str)
			right := v.stack[v.sp-1].(*Object)
			v.sp -= 2

			v.stack[v.sp] = Bool(right.HasProp(v.global.Ctx, string(left)))
			v.sp++
		case OpDoSetDifference:
			left := v.stack[v.sp-2]
			right := v.stack[v.sp-1]
			v.sp -= 2

			if _, ok := right.(Pattern); !ok {
				right = NewExactValuePattern(right.(Serializable))
			}
			v.stack[v.sp] = &DifferencePattern{base: left.(Pattern), removed: right.(Pattern)}
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
		case OpSetGlobal:
			v.ip += 2
			v.sp--
			globalNameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			globalName := v.constants[globalNameIndex].(Str)

			val := v.stack[v.sp]
			v.global.Globals.Set(string(globalName), val)

			if watchable, ok := val.(SystemGraphNodeValue); ok {
				v.global.ProposeSystemGraph(watchable, string(globalName))
			}
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
				endIndex = utils.Min(endIndex, Int(slice.Len()))
			}

			slice.SetSlice(v.global.Ctx, int(startIndex), int(endIndex), val.(Sequence))
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

			var arr Value = &List{underylingList: &ValueList{elements: elements}}

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
				obj.initPartList(v.global.Ctx)
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
				Entries: make(map[string]Serializable, numElements/2),
				Keys:    make(map[string]Serializable, numElements/2),
			}

			for i := v.sp - numElements; i < v.sp; i += 2 {
				key := v.stack[i].(Serializable)
				keyRepr := string(GetRepresentation(key, v.global.Ctx))
				value := v.stack[i+1]
				dict.Entries[keyRepr] = value.(Serializable)
				dict.Keys[keyRepr] = key
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
		case OpCreateUData:
			v.ip += 2
			numHiearchyEntries := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			udata := &UData{}

			for i := v.sp - numHiearchyEntries; i < v.sp; i++ {
				entry := v.stack[i]
				udata.HiearchyEntries = append(udata.HiearchyEntries, entry.(UDataHiearchyEntry))
			}

			v.sp -= numHiearchyEntries
			udata.Root = v.stack[v.sp-1].(Serializable)
			v.stack[v.sp-1] = udata
		case OpCreateUdataHiearchyEntry:
			v.ip += 2
			numChildren := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			entry := UDataHiearchyEntry{}

			for i := v.sp - numChildren; i < v.sp; i++ {
				child := v.stack[i]
				entry.Children = append(entry.Children, child.(UDataHiearchyEntry))
			}

			v.sp -= numChildren
			entry.Value = v.stack[v.sp-1].(Serializable)
			v.stack[v.sp-1] = entry
		case OpCreateStruct:
			v.ip += 3
			structTypeIndex := int(v.curInsts[v.ip-1]) | int(v.curInsts[v.ip-2])<<8
			numElements := int(v.curInsts[v.ip])

			structType := v.constants[structTypeIndex].(*StructPattern)

			values := make([]Value, numElements)
			fieldIndex := 0
			for i := v.sp - numElements; i < v.sp; i++ {
				values[fieldIndex] = v.stack[i]
				fieldIndex++
			}

			v.sp -= numElements
			v.stack[v.sp] = &Struct{
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
		case OpSpreadObjectPattern:
			patt := v.stack[v.sp-2].(*ObjectPattern)
			spreadObjectPatt := v.stack[v.sp-1].(*ObjectPattern)

			for k, v := range spreadObjectPatt.entryPatterns {
				patt.entryPatterns[k] = v
				if _, ok := spreadObjectPatt.optionalEntries[k]; !ok {
					continue
				}
				//set as optional
				if patt.optionalEntries == nil {
					patt.optionalEntries = map[string]struct{}{}
				}
				patt.optionalEntries[k] = struct{}{}
			}
			v.sp--
		case BindCapturedLocals:
			v.ip++
			numCaptured := int(v.curInsts[v.ip])

			fn := v.stack[v.sp-numCaptured-1].(*InoxFunction)
			newFn := &InoxFunction{
				Node:             fn.Node,
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
		case OpCreateObjectPattern:
			v.ip += 3
			numElements := int(v.curInsts[v.ip-1]) | int(v.curInsts[v.ip-2])<<8
			isInexact := int(v.curInsts[v.ip])

			pattern := &ObjectPattern{
				entryPatterns: make(map[string]Pattern),
				inexact:       isInexact == 1,
			}

			for i := v.sp - numElements; i < v.sp; i += 3 {
				key := v.stack[i].(Str)
				value := v.stack[i+1].(Pattern)
				isOptional := v.stack[i+2].(Bool)
				pattern.entryPatterns[string(key)] = value
				if isOptional {
					if pattern.optionalEntries == nil {
						pattern.optionalEntries = make(map[string]struct{}, 1)
					}
					pattern.optionalEntries[string(key)] = struct{}{}
				}
			}
			v.sp -= numElements
			v.stack[v.sp] = pattern
			v.sp++
		case OpCreateOptionPattern:
			v.ip += 2
			nameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			name := v.constants[nameIndex].(Str)

			value := v.stack[v.sp-1].(Pattern)
			v.stack[v.sp-1] = &OptionPattern{Name: string(name), Value: value}
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
			v.ip += 3
			numElements := int(v.curInsts[v.ip-2])
			nameListIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			nameList := v.constants[nameListIndex].(KeyList)

			var subpatterns []StringPattern

			for i := v.sp - numElements; i < v.sp; i++ {
				subpatterns = append(subpatterns, v.stack[i].(StringPattern))
			}
			v.sp -= numElements

			val, err := NewSequenceStringPattern(nil, subpatterns, nameList)
			if err != nil {
				v.err = err
				return
			}
			v.stack[v.sp] = val
			v.sp++
		case OpCreatePatternNamespace:
			init := v.stack[v.sp-1]

			val, err := CreatePatternNamespace(init)
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
		case OpToPattern:
			val := v.stack[v.sp-1].(Serializable)

			if _, ok := val.(Pattern); !ok {
				v.stack[v.sp-1] = NewMostAdaptedExactPattern(val)
			}
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
		case OpCreateOption:
			v.ip += 2
			nameIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			name := v.constants[nameIndex].(Str)

			value := v.stack[v.sp-1]
			v.stack[v.sp-1] = Option{Name: string(name), Value: value}
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
				Start:        lower,
				End:          upper,
				Step:         1,
			}
			v.sp--
		case OpCreateUpperBoundRange:
			upperBound := v.stack[v.sp-1]

			switch val := upperBound.(type) {
			case Int:
				v.stack[v.sp-1] = IntRange{
					unknownStart: true,
					inclusiveEnd: true,
					End:          int64(val),
					Step:         1,
				}
			case Float:
				v.err = fmt.Errorf("floating point ranges not supported")
			default:
				v.stack[v.sp-1] = QuantityRange{
					unknownStart: true,
					inclusiveEnd: true,
					End:          val.(Serializable),
				}
			}
		case OpCreateTestSuite:
			v.ip += 2
			nodeIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			chunk := v.constants[nodeIndex].(AstNode).Node.(*parse.Chunk)
			meta := v.stack[v.sp-1]

			suite, err := NewTestSuite(meta, chunk, v.global)
			if err != nil {
				v.err = err
				return
			}
			v.stack[v.sp-1] = suite
		case OpCreateTestCase:
			v.ip += 2
			nodeIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			chunk := v.constants[nodeIndex].(AstNode).Node.(*parse.Chunk)
			meta := v.stack[v.sp-1]

			suite, err := NewTestCase(meta, chunk, v.global)
			if err != nil {
				v.err = err
				return
			}
			v.stack[v.sp-1] = suite
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
			highIdx = utils.Min(highIdx, int(slice.Len()))

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
		case OpCall:
			numArgs := int(v.curInsts[v.ip+1])
			spread := int(v.curInsts[v.ip+2])
			must := int(v.curInsts[v.ip+3])
			v.ip += 3

			if !v.fnCall(numArgs, spread == 1, must == 1) {
				return
			}
		case OpReturn:
			v.ip++
			var retVal Value
			isValOnStack := int(v.curInsts[v.ip]) == 1
			if isValOnStack {
				retVal = v.stack[v.sp-1]
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
			}

			v.stack[v.sp-1] = retVal

			if topLevelFnEval {
				return
			}
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

			if v.global.Routine == nil {
				v.err = errors.New("failed to yield: no associated routine")
				return
			}
			v.global.Routine.yield(v.global.Ctx, retVal)
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
		case OpGetSelf:
			v.stack[v.sp] = v.curFrame.self
			v.sp++
		case OpGetSupersys:
			part, ok := v.curFrame.self.(SystemPart)
			if !ok {
				v.err = ErrNotAttachedToSupersystem
				return
			}
			v.stack[v.sp], v.err = part.System()
			if v.err != nil {
				return
			}
			v.sp++
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
		case OpDropPerms:
			permListing := v.stack[v.sp-1].(*Object)
			v.sp--

			//TODO: check listing ?

			perms, err := getPermissionsFromListing(permListing, nil, nil, false)
			if err != nil {
				v.err = err
				return
			}

			v.global.Ctx.DropPermissions(perms)
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

			config, err := buildImportConfig(configObject, source, v.global)
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
		case OpSpawnRoutine:
			v.ip += 5
			isSingleExpr := v.curInsts[v.ip-4]
			calleeNameindex := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
			caleeName := v.constants[calleeNameindex].(Str)

			routimeModConstantIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			routineMod := v.constants[routimeModConstantIndex].(*Module)

			meta := v.stack[v.sp-2]
			singleExprCallee := v.stack[v.sp-1]

			var (
				group       *RoutineGroup
				globalsDesc Value
				permListing *Object
			)

			if meta != nil && meta != Nil {
				metaMap := meta.(*Struct).ValueMap()

				group, globalsDesc, permListing, v.err = readRoutineMeta(metaMap, v.global.Ctx)
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
			case *Struct:
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
				perms, err := getPermissionsFromListing(permListing, nil, nil, true)
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
				newCtx, err := v.global.Ctx.ChildWithout(IMPLICITLY_REMOVED_ROUTINE_PERMS)
				if err != nil {
					v.err = fmt.Errorf("spawn expression: new context: %w", err)
					return
				}
				ctx = newCtx
			}

			routine, err := SpawnRoutine(RoutineSpawnArgs{
				SpawnerState: v.global,
				Globals:      GlobalVariablesFromMap(actualGlobals, startConstants),
				Module:       routineMod,
				RoutineCtx:   ctx,
				UseBytecode:  true,
			})

			if err != nil {
				v.err = err
				return
			}

			if group != nil {
				group.Add(routine)
			}

			v.sp -= 1
			v.stack[v.sp-1] = routine
			// isCall := v.curInsts[v.ip] == 1

			// groupVal := v.stack[v.sp-4]
			// globalDesc := v.stack[v.sp-3]

			// upper := v.stack[v.sp-1].(Rune)
		case OpCreateLifetimeJob:
			v.ip += 2
			nodeIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			mod := v.constants[nodeIndex].(*Module)

			meta := v.stack[v.sp-2]
			subject, _ := v.stack[v.sp-1].(Pattern)

			job, err := NewLifetimeJob(meta, subject, mod, v.global)
			if err != nil {
				v.err = err
				return
			}
			v.stack[v.sp-2] = job
			v.sp--
		case OpCreateReceptionHandler:
			pattern := v.stack[v.sp-2].(Pattern)
			handler, _ := v.stack[v.sp-1].(*InoxFunction)

			v.stack[v.sp-2] = NewSynchronousMessageHandler(v.global.Ctx, handler, pattern)
			v.sp--
		case OpCreateXMLelem:
			v.ip += 4
			tagNameIndex := int(v.curInsts[v.ip-2]) | int(v.curInsts[v.ip-3])<<8
			attributeCount := int(v.curInsts[v.ip-1])
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

			childrenStart := v.sp - childCount
			var children []Value
			if childCount > 0 {
				children = make([]Value, childCount)
				copy(children, v.stack[childrenStart:v.sp])
			}

			v.sp -= (childCount + 2*attributeCount)

			v.stack[v.sp] = NewXmlElement(tagName, attributes, children)
			v.sp++
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
		case OpAssert:
			v.ip += 2
			nodeIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			stmt := v.constants[nodeIndex].(AstNode).Node.(*parse.AssertionStatement)

			ok := v.stack[v.sp-1].(Bool)
			v.sp--

			if !ok {
				data := &AssertionData{
					assertionStatement: stmt,
					intermediaryValues: map[parse.Node]Value{},
				}
				v.err = &AssertionError{msg: "assertion is false", data: data}
				return
			}
		case OpConcat:
			v.ip += 3
			numElements := int(v.curInsts[v.ip-2])
			spreadElemSetConstantIndex := int(v.curInsts[v.ip]) | int(v.curInsts[v.ip-1])<<8
			spreadElemSet := v.constants[spreadElemSetConstantIndex].(*List).underylingList.(*BoolList)

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
		case OpSuspendVM:
			return
		default:
			v.err = fmt.Errorf("unknown opcode: %d (at %d)", v.curInsts[v.ip], v.ip)
			return
		}
	}
}

func (v *VM) fnCall(numArgs int, spread, must bool) bool {
	var (
		objectVal       = v.stack[v.sp-2]
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
			extBytecode = extState.Module.Bytecode
		}

	} else {
		goFn := callee.(*GoFunction)
		isSharedFunction = goFn.IsShared()
		if isSharedFunction {
			extState = goFn.originState
			extBytecode = extState.Module.Bytecode
		}
	}

	//get rid of callee & object
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

		// update call frame
		v.curFrame.ip = v.ip
		v.curFrame = &(v.frames[v.framesIndex])
		v.curFrame.self, _ = objectVal.(*Object)
		v.curFrame.externalFunc = isSharedFunction
		v.curFrame.fn = compiled
		v.curFrame.basePointer = v.sp - numArgs
		v.curFrame.lockedValues = nil
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
