package core

import (
	"errors"
	"fmt"
	"sync"

	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrCannotCreateDynamicMemberFromSharedValue                 = errors.New("cannot create dynamic member from shared value")
	ErrCannotCreateDynamicMemberMissingProperty                 = errors.New("cannot create dynamic member: property is missing")
	ErrCannotCreateDynamicMapInvocationValueInDynValNotIterable = errors.New("cannot create dynamic map invocation: value in passed dynamic value is not iterable")
	ErrCannotCreateIfCondNotBoolean                             = errors.New("cannot create dynamic if: condition value is not a boolean")
	ErrDynCallNonFunctionCalee                                  = errors.New("callee in dynamic call value is not a function")
	ErrUnknownDynamicOp                                         = errors.New("unknown dynamic operation")
)

func init() {
	RegisterSymbolicGoFunctions([]any{
		NewDynamicIf, func(ctx *symbolic.Context, cond *symbolic.DynamicValue, consequent, alternate symbolic.SymbolicValue) *symbolic.DynamicValue {
			return symbolic.NewDynamicValue(symbolic.NewMultivalue(consequent, alternate))
		},
		NewDynamicCall, func(ctx *symbolic.Context, callee symbolic.SymbolicValue, args ...symbolic.SymbolicValue) *symbolic.DynamicValue {
			switch callee.(type) {
			case *symbolic.InoxFunction, *symbolic.GoFunction:
			default:
				ctx.AddSymbolicGoFunctionError("callee should be a function")
			}

			return symbolic.NewDynamicValue(symbolic.ANY)
		},
	})

}

// A DynamicValue resolves to a Value by performing an operation on another value (getting a property, ...),
// DynamicValue implements Value
type DynamicValue struct {
	value    Value // value on which we apply the operation
	innerDyn *DynamicValue

	opData0  Value //additional data needed to apply the operation
	opData1  Value //additional data needed to apply the operation
	op       dynOp
	opResult Value //this field is set for some operations

	lock              sync.Mutex
	mutationCallbacks *MutationCallbacks
}

type dynOp int

const (
	dynMemb dynOp = iota + 1
	dynMapInvoc
	dynIf
	dynCall
)

func (op dynOp) hasIterableResult() bool {
	return op == dynMapInvoc
}

func (op dynOp) String() string {
	switch op {
	case dynMemb:
		return "dynamic-member"
	case dynMapInvoc:
		return "dynamic-map-invocation"
	case dynIf:
		return "dynamic-if"
	case dynCall:
		return "dynamic-call"
	default:
		panic(fmt.Errorf("invalid dynamic operation: %d", op))
	}
}

func NewDynamicMemberValue(ctx *Context, object Value, memberName string) (*DynamicValue, error) {
	// if IsShared(object) {
	// 	return nil, ErrCannotCreateDynamicMemberFromSharedValue
	// }

	ok := false
	for _, name := range object.(IProps).PropertyNames(ctx) {
		if name == memberName {
			ok = true
			break
		}
	}

	if !ok {
		return nil, ErrCannotCreateDynamicMemberMissingProperty
	}

	var value Value = object
	innerDyn, ok := object.(*DynamicValue)
	if ok {
		value = innerDyn.Resolve(ctx)
	}

	dyn := &DynamicValue{
		value:             value,
		innerDyn:          innerDyn,
		opData0:           Str(memberName),
		op:                dynMemb,
		mutationCallbacks: NewMutationCallbacks(),
	}

	if innerDyn != nil {
		_, err := innerDyn.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			registerAgain = true

			dyn.lock.Lock()
			unlock := true
			defer func() {
				if unlock {
					dyn.lock.Unlock()
				}
			}()
			dyn.value = innerDyn.Resolve(ctx)
			unlock = false
			dyn.lock.Unlock()

			dyn.mutationCallbacks.CallMicrotasks(ctx, NewUnspecifiedMutation(ShallowWatching, ""))

			return
		}, MutationWatchingConfiguration{Depth: ShallowWatching})

		if err != nil {
			return nil, fmt.Errorf("failed to create dynamic value: %w", err)
		}
	}

	watchable, ok := dyn.value.(Watchable)
	if ok {
		_, err := watchable.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			registerAgain = true

			switch mutation.Kind {
			case AddProp, UpdateProp:
				if mutation.AffectedProperty(ctx) == memberName {
					dyn.mutationCallbacks.CallMicrotasks(ctx, NewUnspecifiedMutation(ShallowWatching, ""))
				}
			}

			return
		}, MutationWatchingConfiguration{Depth: ShallowWatching})

		if err != nil {
			return nil, fmt.Errorf("failed to create dynamic value: %w", err)
		}
	}

	return dyn, nil
}

func NewDynamicMapInvocation(ctx *Context, iterable Iterable, mapper Value) (*DynamicValue, error) {
	var value Value = iterable
	innerDyn, ok := iterable.(*DynamicValue)
	if ok {
		value = innerDyn.Resolve(ctx)
		if _, ok := value.(Iterable); !ok {
			return nil, fmt.Errorf("%w: %T", ErrCannotCreateDynamicMapInvocationValueInDynValNotIterable, value)
		}
	}

	dyn := &DynamicValue{
		value:             value,
		innerDyn:          innerDyn,
		opData0:           mapper,
		op:                dynMapInvoc,
		mutationCallbacks: NewMutationCallbacks(),
	}

	if innerDyn != nil {
		_, err := innerDyn.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			registerAgain = true

			dyn.lock.Lock()
			unlock := true
			defer func() {
				if unlock {
					dyn.lock.Unlock()
				}
			}()
			dyn.value = innerDyn.Resolve(ctx)
			unlock = false
			dyn.lock.Unlock()

			dyn.mutationCallbacks.CallMicrotasks(ctx, NewUnspecifiedMutation(ShallowWatching, ""))

			return
		}, MutationWatchingConfiguration{Depth: ShallowWatching})

		if err != nil {
			return nil, fmt.Errorf("failed to create dynamic value: %w", err)
		}
	}

	watchable, ok := dyn.value.(Watchable)
	if ok {
		_, err := watchable.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
			registerAgain = true

			switch mutation.Kind {
			case UnspecifiedMutation, InsertElemAtIndex, SetElemAtIndex: // TODO: add other mutation kinds when they are implemented
				dyn.mutationCallbacks.CallMicrotasks(ctx, NewUnspecifiedMutation(ShallowWatching, ""))
			}

			return
		}, MutationWatchingConfiguration{Depth: ShallowWatching})

		if err != nil {
			return nil, fmt.Errorf("failed to create dynamic value: %w", err)
		}
	}

	return dyn, nil
}

func NewDynamicIf(ctx *Context, condition *DynamicValue, consequent Value, alternate Value) *DynamicValue {
	var value Value = consequent

	if condVal, ok := condition.Resolve(ctx).(Bool); !ok {
		panic(ErrCannotCreateIfCondNotBoolean)
	} else if !condVal {
		value = alternate
	}

	dyn := &DynamicValue{
		value:             value,
		innerDyn:          condition,
		opData0:           consequent,
		opData1:           alternate,
		op:                dynIf,
		mutationCallbacks: NewMutationCallbacks(),
	}

	_, err := dyn.innerDyn.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
		registerAgain = true

		dyn.lock.Lock()
		unlock := true
		defer func() {
			if unlock {
				dyn.lock.Unlock()
			}
		}()

		condVal, ok := dyn.innerDyn.Resolve(ctx).(Bool)
		if !ok {
			panic(fmt.Errorf("condition of dynamic if resolved to a %T but a boolean is expected", condVal))
		}
		if condVal {
			dyn.value = dyn.opData0
		} else {
			dyn.value = dyn.opData1
		}

		unlock = false
		dyn.lock.Unlock()

		dyn.mutationCallbacks.CallMicrotasks(ctx, NewUnspecifiedMutation(ShallowWatching, ""))

		return
	}, MutationWatchingConfiguration{Depth: ShallowWatching})

	if err != nil {
		panic(fmt.Errorf("failed to create dynamic value: %w", err))
	}

	return dyn
}

// TODO: restrict callee to functions without side effects
func NewDynamicCall(ctx *Context, callee Value, args ...Value) *DynamicValue {

	actualArgs := make([]Value, len(args))

	call := func(ctx *Context) (Value, error) {
		state := ctx.GetClosestState()
		for i, arg := range args {
			actualArgs[i] = Unwrap(ctx, arg)
		}

		switch c := callee.(type) {
		case *InoxFunction:
			return c.Call(state, nil, actualArgs, nil)
		case *GoFunction:
			args := utils.MapSlice(actualArgs, func(arg Value) any { return arg })
			return c.Call(args, state, nil, false, true)
		default:
			panic(ErrDynCallNonFunctionCalee)
		}
	}

	firstCallResult, err := call(ctx)
	if err != nil {
		panic(fmt.Errorf("failed to create dynamic call: error on first call: %w", err))
	}

	//TODO: support any arg types

	dyn := &DynamicValue{
		value:             firstCallResult,
		opData0:           NewWrappedValueList(ToSerializableSlice(args)...),
		op:                dynCall,
		mutationCallbacks: NewMutationCallbacks(),
	}

	var watchables []Value
	for _, arg := range args {
		if watchable, ok := arg.(Watchable); ok {
			watchables = append(watchables, watchable)

			_, err := watchable.OnMutation(ctx, func(ctx *Context, mutation Mutation) (registerAgain bool) {
				registerAgain = true

				dyn.lock.Lock()
				defer dyn.lock.Unlock()

				callResult, err := call(ctx)
				if err != nil {
					ctx.Logger().Print("error during dynamic call: ", err)
					return
				}
				dyn.value = callResult
				dyn.mutationCallbacks.CallMicrotasks(ctx, NewUnspecifiedMutation(ShallowWatching, ""))

				return
			}, MutationWatchingConfiguration{Depth: ShallowWatching})

			if err != nil {
				panic(fmt.Errorf("failed to create dynamic call: failed to register a mutation callback: %w", err))
			}
		}
	}

	dyn.opData1 = NewWrappedValueList(ToSerializableSlice(watchables)...)

	return dyn
}

func (dyn *DynamicValue) memberName() string {
	return string(dyn.opData0.(Str))
}

func (dyn *DynamicValue) Resolve(ctx *Context) Value {
	dyn.lock.Lock()
	defer dyn.lock.Unlock()

	switch dyn.op {
	case dynMemb:
		return dyn.value.(IProps).Prop(ctx, dyn.memberName())
	case dynMapInvoc:
		if dyn.opResult == nil {
			//TODO: this can be quite long and could cause the dynamic value to be locked a long time, try lazy mapping ?
			dyn.opResult = Map(ctx, dyn.value.(Iterable), dyn.opData0)
		}
		return dyn.opResult
	case dynIf, dynCall:
		return dyn.value
	default:
		panic(fmt.Errorf("%w: %d", ErrUnknownDynamicOp, dyn.op))
	}
}

func (dyn *DynamicValue) PropertyNames(ctx *Context) []string {
	return dyn.Resolve(ctx).(IProps).PropertyNames(ctx)
}

func (dyn *DynamicValue) Prop(ctx *Context, name string) Value {
	return utils.Must(NewDynamicMemberValue(ctx, dyn, name))
}

func (dyn *DynamicValue) SetProp(ctx *Context, name string, value Value) error {
	return ErrCannotSetProp
}
