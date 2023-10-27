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

// The Container interface should be implemented by data structures
// able to tell if they contain a specific value. The data structure
// can contain a infinite number of values.
type Container interface {
	Serializable
	Iterable

	//Contains should return true:
	// - if the value has a URL AND there is an element such as Same(element, value) is true.
	// - if the value has not a URL AND there is an element equal to value.
	Contains(ctx *Context, value Serializable) bool
}

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

func (obj *Object) Contains(ctx *Context, value Serializable) bool {
	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)

	if urlHolder, ok := value.(UrlHolder); ok {
		_, ok := urlHolder.URL()
		if ok {
			for i := 0; i < obj.Len(); i++ {
				e := obj.values[i]
				if Same(e, value) {
					return true
				}
			}
			return false
		}
	}

	for i := 0; i < obj.Len(); i++ {
		e := obj.values[i]
		if value.Equal(ctx, e, map[uintptr]uintptr{}, 0) {
			return true
		}
	}
	return false
}

func (rec *Record) Contains(ctx *Context, value Serializable) bool {
	if urlHolder, ok := value.(UrlHolder); ok {
		_, ok := urlHolder.URL()
		if ok {
			for i := 0; i < rec.Len(); i++ {
				e := rec.values[i]
				if Same(e, value) {
					return true
				}
			}
			return false
		}
	}

	for i := 0; i < rec.Len(); i++ {
		e := rec.values[i]
		if value.Equal(ctx, e, map[uintptr]uintptr{}, 0) {
			return true
		}
	}
	return false
}

func (r IntRange) Contains(ctx *Context, v Serializable) bool {
	i, ok := v.(Int)
	return ok && r.Includes(ctx, i)
}

func (r FloatRange) Contains(ctx *Context, v Serializable) bool {
	f, ok := v.(Float)
	return ok && r.Includes(ctx, f)
}

func (r RuneRange) Contains(ctx *Context, v Serializable) bool {
	i, ok := v.(Rune)
	return ok && r.Includes(ctx, i)
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
