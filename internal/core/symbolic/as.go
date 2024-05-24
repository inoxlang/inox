package symbolic

import "reflect"

// asInterface should be implemented by complex values that conditionally implement interfaces
// such as Indexable, Iterable, Serializable.
// The primary implementation of asInterface is Multivalue.
type asInterface interface {
	Value
	as(itf reflect.Type) Value
}

func as(v Value, itf reflect.Type) Value {
	val, ok := v.(asInterface)
	if !ok {
		return v
	}

	return val.as(itf)
}

func AsSerializable(v Value) Value {
	return as(v, SERIALIZABLE_INTERFACE_TYPE)
}

func AsSerializableChecked(v Value) Serializable {
	return AsSerializable(v).(Serializable)
}

func AsIprops(v Value) Value {
	return as(v, IPROPS_INTERFACE_TYPE)
}

func asIndexable(v Value) Value {
	return as(v, INDEXABLE_INTERFACE_TYPE)
}

func asSequence(v Value) Value {
	return as(v, INDEXABLE_INTERFACE_TYPE)
}

func asIterable(v Value) Value {
	return as(v, ITERABLE_INTERFACE_TYPE)
}

func asStreamable(v Value) Value {
	return as(v, STREAMABLE_INTERFACE_TYPE)
}

func asWatchable(v Value) Value {
	return as(v, WATCHABLE_INTERFACE_TYPE)
}

func AsStringLike(v Value) Value {
	return as(v, STRLIKE_INTERFACE_TYPE)
}
