package symbolic

import "reflect"

type asInterface interface {
	SymbolicValue
	as(itf reflect.Type) SymbolicValue
}

func as(v SymbolicValue, itf reflect.Type) SymbolicValue {
	val, ok := v.(asInterface)
	if !ok {
		return v
	}

	return val.as(itf)
}

func AsSerializable(v SymbolicValue) SymbolicValue {
	return as(v, SERIALIZABLE_INTERFACE_TYPE)
}

func AsIprops(v SymbolicValue) SymbolicValue {
	return as(v, IPROPS_INTERFACE_TYPE)
}

func asIndexable(v SymbolicValue) SymbolicValue {
	return as(v, INDEXABLE_INTERFACE_TYPE)
}

func asSequence(v SymbolicValue) SymbolicValue {
	return as(v, INDEXABLE_INTERFACE_TYPE)
}

func asIterable(v SymbolicValue) SymbolicValue {
	return as(v, ITERABLE_INTERFACE_TYPE)
}

func asStreamable(v SymbolicValue) SymbolicValue {
	return as(v, STREAMABLE_INTERFACE_TYPE)
}

func asWatchable(v SymbolicValue) SymbolicValue {
	return as(v, WATCHABLE_INTERFACE_TYPE)
}
