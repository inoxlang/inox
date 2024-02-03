package core

import (
	"errors"
	"reflect"
)

var (
	ErrCannotAddNonSharableToSharedContainer = errors.New("cannot add a non sharable element to a shared container")

	_ = []Container{
		(*List)(nil), (*Tuple)(nil), (*Object)(nil), (*Record)(nil),
		IntRange{}, FloatRange{}, RuneRange{}, QuantityRange{},
	}
)

// The Container interface should be implemented by data structures able to tell if they contain a specific value.
// Implementations can contain an infinite number of values.
type Container interface {
	Serializable
	Iterable

	//Contains should return true:
	// - if the value has a URL AND there is an element such as Same(element, value) is true.
	// - if the value has not a URL AND there is an element equal to value.
	Contains(ctx *Context, value Serializable) bool

	IsEmpty(ctx *Context) bool
}

// Implementations of Container for some core types.

func (l *List) Contains(ctx *Context, value Serializable) bool {
	if urlHolder, ok := value.(UrlHolder); ok {
		_, ok := urlHolder.URL()
		if ok {
			for i := 0; i < l.Len(); i++ {
				e := l.underlyingList.At(ctx, i)
				if Same(e, value) {
					return true
				}
			}
			return false
		}
	}

	for i := 0; i < l.Len(); i++ {
		e := l.underlyingList.At(ctx, i)
		if value.Equal(ctx, e, map[uintptr]uintptr{}, 0) {
			return true
		}
	}
	return false
}

func (l *List) IsEmpty(ctx *Context) bool {
	return l.Len() == 0
}

func (t *Tuple) Contains(ctx *Context, value Serializable) bool {
	if urlHolder, ok := value.(UrlHolder); ok {
		_, ok := urlHolder.URL()
		if ok {
			for i := 0; i < t.Len(); i++ {
				e := t.elements[i]
				if Same(e, value) {
					return true
				}
			}
			return false
		}
	}

	for i := 0; i < t.Len(); i++ {
		e := t.elements[i]
		if value.Equal(ctx, e, map[uintptr]uintptr{}, 0) {
			return true
		}
	}
	return false
}

func (t *Tuple) IsEmpty(ctx *Context) bool {
	return t.Len() == 0
}

func (obj *Object) Contains(ctx *Context, value Serializable) bool {
	obj.waitForOtherTxsToTerminate(ctx, false)

	closestState := ctx.GetClosestState()
	obj._lock(closestState)
	defer obj._unlock(closestState)

	if urlHolder, ok := value.(UrlHolder); ok {
		_, ok := urlHolder.URL()
		valueCount := len(obj.values)
		if ok {
			for i := 0; i < valueCount; i++ {
				e := obj.values[i]
				if Same(e, value) {
					return true
				}
			}
			return false
		}
	}

	valueCount := len(obj.values)
	for i := 0; i < valueCount; i++ {
		e := obj.values[i]
		if value.Equal(ctx, e, map[uintptr]uintptr{}, 0) {
			return true
		}
	}
	return false
}

func (obj *Object) IsEmpty(ctx *Context) bool {
	obj.waitForOtherTxsToTerminate(ctx, false)

	closestState := ctx.GetClosestState()
	obj._lock(closestState)
	defer obj._unlock(closestState)

	return len(obj.keys) == 0
}

func (rec *Record) Contains(ctx *Context, value Serializable) bool {
	if urlHolder, ok := value.(UrlHolder); ok {
		_, ok := urlHolder.URL()
		valueCount := len(rec.values)
		if ok {
			for i := 0; i < valueCount; i++ {
				e := rec.values[i]
				if Same(e, value) {
					return true
				}
			}
			return false
		}
	}

	valueCount := len(rec.values)
	for i := 0; i < valueCount; i++ {
		e := rec.values[i]
		if value.Equal(ctx, e, map[uintptr]uintptr{}, 0) {
			return true
		}
	}
	return false
}

func (rec *Record) IsEmpty(ctx *Context) bool {
	return len(rec.keys) == 0
}

func (r IntRange) Contains(ctx *Context, v Serializable) bool {
	i, ok := v.(Int)
	return ok && r.Includes(ctx, i)
}

func (r IntRange) IsEmpty(ctx *Context) bool {
	if r.HasKnownStart() {
		return r.Len() == 0
	}
	//TODO: define what should be returned if the end is the minimum float.
	return false
}

func (r FloatRange) Contains(ctx *Context, v Serializable) bool {
	f, ok := v.(Float)
	return ok && r.Includes(ctx, f)
}

func (r FloatRange) IsEmpty(ctx *Context) bool {
	if r.unknownStart {
		//TODO: define what should be returned if the end is the minimum float.
		return false
	}

	return r.KnownStart() == r.InclusiveEnd()
}

func (r RuneRange) Contains(ctx *Context, v Serializable) bool {
	i, ok := v.(Rune)
	return ok && r.Includes(ctx, i)
}

func (r RuneRange) IsEmpty(ctx *Context) bool {
	return r.Len() == 0
}

func (r QuantityRange) Contains(ctx *Context, v Serializable) bool {
	val := reflect.ValueOf(v)
	endReflVal := reflect.ValueOf(r.InclusiveEnd())

	if val.Type() != endReflVal.Type() {
		return false
	}

	switch endReflVal.Kind() {
	case reflect.Float64:
		if !r.unknownStart && quantityLessThan(val, reflect.ValueOf(r.start)) {
			return false
		}
		return quantityLessOrEqual(val, endReflVal)
	case reflect.Int64:
		if !r.unknownStart && quantityLessThan(val, reflect.ValueOf(r.start)) {
			return false
		}
		return quantityLessOrEqual(val, endReflVal)
	default:
		panic(ErrUnreachable)
	}
}

func (r QuantityRange) IsEmpty(ctx *Context) bool {
	if r.unknownStart {
		return false
	}
	return r.KnownStart().Equal(ctx, r.InclusiveEnd(), map[uintptr]uintptr{}, 0)
}
