package core

import (
	"bytes"
	"errors"
	"reflect"

	"github.com/inoxlang/inox/internal/utils"
)

// Core value types' implementations of Value.Equal

const (
	MAX_COMPARISON_DEPTH = 200
)

func (err Error) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherErr, ok := other.(Error)
	if !ok {
		return false
	}

	goErr := reflect.ValueOf(err.goError)
	otherGoErr := reflect.ValueOf(otherErr.goError)

	if goErr.IsValid() != otherGoErr.IsValid() {
		return false
	}

	if goErr.IsValid() {
		return goErr.Type() == otherGoErr.Type() && err.goError.Error() == otherErr.goError.Error()
	}
	return true
}

func (n AstNode) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherNode, ok := other.(AstNode)
	if !ok {
		return false
	}
	return n.Node == otherNode.Node
}

func (t Token) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherToken, ok := other.(Token)
	if !ok {
		return false
	}
	return t.value == otherToken.value
}

func (Nil NilT) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	switch other.(type) {
	case NilT:
		return true
	case nil:
		return false
	default:
		rval := reflect.ValueOf(other)
		if rval.IsValid() && rval.Kind() == reflect.Pointer {
			return rval.IsNil()
		}
		return false
	}
}

func (boolean Bool) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherBool, ok := other.(Bool)
	return ok && otherBool == boolean
}

func (r Rune) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherRune, ok := other.(Rune)
	return ok && otherRune == r
}

func (b Byte) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherByte, ok := other.(Byte)
	return ok && otherByte == b
}

func (i Int) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherInt, ok := other.(Int)
	return ok && otherInt == i
}

func (f Float) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherFloat, ok := other.(Float)
	return ok && otherFloat == f
}

func (s Str) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	strLike, ok := other.(StringLike)
	if !ok {
		return false
	}
	if strLike == nil {
		panic(errors.New("cannot compare string with nil StringLike"))
	}
	if otherStr, ok := strLike.(Str); ok {
		return s == otherStr
	}
	return ok && strLike.Equal(ctx, s, alreadyCompared, depth)
}

func (obj *Object) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {

	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	otherObject, ok := other.(*Object)
	if !ok {
		return false
	}

	addr := reflect.ValueOf(obj).Pointer()
	otherAddr := reflect.ValueOf(otherObject).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	closestState := ctx.GetClosestState()
	obj.Lock(closestState)
	defer obj.Unlock(closestState)
	if len(obj.keys) != len(otherObject.keys) {
		return false
	}

	if addr == otherAddr {
		return true
	}

	//check that all properties are equal
	for i, v := range obj.values {
		k := obj.keys[i]
		if !otherObject.HasProp(ctx, k) {
			return false
		}
		if !v.Equal(ctx, otherObject.Prop(ctx, k), alreadyCompared, depth+1) {
			return false
		}
	}

	return true
}

func (rec *Record) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	otherRec, ok := other.(*Record)
	if !ok {
		return false
	}

	//TODO: cache representation
	return GetRepresentation(rec, ctx).Equal(GetRepresentation(otherRec, ctx))
}

func (dict *Dictionary) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	otherDict, ok := other.(*Dictionary)
	if !ok {
		return false
	}

	addr := reflect.ValueOf(dict).Pointer()
	otherAddr := reflect.ValueOf(otherDict).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	if len(dict.entries) != len(otherDict.entries) {
		return false
	}

	if addr == otherAddr {
		return true
	}

	//check that all properties are equal
	for k, v := range dict.entries {
		if !v.Equal(ctx, otherDict.entries[k], alreadyCompared, depth+1) {
			return false
		}
	}

	return true
}

func (list KeyList) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	otherList, ok := other.(KeyList)
	if !ok {
		return false
	}

	addr := reflect.ValueOf(list).Pointer()
	otherAddr := reflect.ValueOf(otherList).Pointer()

	if len(list) != len(otherList) {
		return false
	}

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	if addr == otherAddr {
		return true
	}

	if len(list) != len(otherList) {
		return false
	}

	//check that all keys are equal
	for _, v := range list {
		if !utils.SliceContains(otherList, v) {
			return false
		}
	}

	return true
}

func (a *Array) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	otherArray, ok := other.(*Array)
	if !ok {
		return false
	}

	addr := reflect.ValueOf(a).Pointer()
	otherAddr := reflect.ValueOf(otherArray).Pointer()

	if a.Len() != otherArray.Len() {
		return false
	}

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	if addr == otherAddr {
		return true
	}

	for i, e := range *a {
		if !(*otherArray)[i].Equal(ctx, e, alreadyCompared, depth+1) {
			return false
		}
	}

	return true
}

func (list *List) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	return list.underlyingList.Equal(ctx, other, alreadyCompared, depth)
}

func (list *ValueList) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	var otherList underlyingList
	switch v := other.(type) {
	case *List:
		otherList = v.underlyingList
	case underlyingList:
		otherList = v
	default:
		return false
	}

	addr := reflect.ValueOf(list).Pointer()
	otherAddr := reflect.ValueOf(otherList).Pointer()

	if list.Len() != otherList.Len() {
		return false
	}

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	if addr == otherAddr {
		return true
	}

	switch other := otherList.(type) {
	case *ValueList:
		//check that all elements are equal
		for i, v := range list.elements {
			if !v.Equal(ctx, other.elements[i], alreadyCompared, depth+1) {
				return false
			}
		}
	default:
		it := other.Iterator(ctx, IteratorConfiguration{})

		//check that all elements are equal
		for _, v := range list.elements {
			it.Next(ctx)
			otherElem := it.Value(ctx)
			if !v.Equal(ctx, otherElem, alreadyCompared, depth+1) {
				return false
			}
		}

	}

	return true
}

func (list *IntList) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	var otherList underlyingList
	switch v := other.(type) {
	case *List:
		otherList = v.underlyingList
	case underlyingList:
		otherList = v
	default:
		return false
	}

	addr := reflect.ValueOf(list).Pointer()
	otherAddr := reflect.ValueOf(otherList).Pointer()

	if list.Len() != otherList.Len() {
		return false
	}

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	if addr == otherAddr {
		return true
	}

	switch other := otherList.(type) {
	case *IntList:
		//check that all elements are equal
		for i, v := range list.elements {
			if v != other.elements[i] {
				return false
			}
		}
	default:
		it := other.Iterator(ctx, IteratorConfiguration{})

		//check that all elements are equal
		for _, v := range list.elements {
			it.Next(ctx)
			otherElem := it.Value(ctx)
			if integer, ok := otherElem.(Int); !ok || v != integer {
				return false
			}
		}

	}

	return true
}

func (list *BoolList) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	var otherList underlyingList
	switch v := other.(type) {
	case *List:
		otherList = v.underlyingList
	case underlyingList:
		otherList = v
	default:
		return false
	}

	addr := reflect.ValueOf(list).Pointer()
	otherAddr := reflect.ValueOf(otherList).Pointer()

	if list.Len() != otherList.Len() {
		return false
	}

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	if addr == otherAddr {
		return true
	}

	switch other := otherList.(type) {
	case *BoolList:
		return list.elements.Equal(other.elements)
	default:
		it := other.Iterator(ctx, IteratorConfiguration{})
		i := 0
		//check that all elements are equal
		for it.Next(ctx) {
			if i >= list.Len() {
				return false
			}

			otherElem := it.Value(ctx)
			if boolean, ok := otherElem.(Bool); !ok || Bool(list.elements.Test(uint(i))) != boolean {
				return false
			}

			i++
		}

		if i != list.Len() {
			return false
		}
	}

	return true
}

func (list *StringList) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	var otherList underlyingList
	switch v := other.(type) {
	case *List:
		otherList = v.underlyingList
	case underlyingList:
		otherList = v
	default:
		return false
	}

	addr := reflect.ValueOf(list).Pointer()
	otherAddr := reflect.ValueOf(otherList).Pointer()

	if list.Len() != otherList.Len() {
		return false
	}

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	if addr == otherAddr {
		return true
	}

	switch other := otherList.(type) {
	case *StringList:
		//check that all elements are equal
		for i, v := range list.elements {
			if v != other.elements[i] {
				return false
			}
		}
	default:
		it := other.Iterator(ctx, IteratorConfiguration{})

		//check that all elements are equal
		for _, v := range list.elements {
			it.Next(ctx)
			otherElem := it.Value(ctx)
			if str, ok := otherElem.(StringLike); !ok || v.GetOrBuildString() != str.GetOrBuildString() {
				return false
			}
		}

	}

	return true
}

func (tuple *Tuple) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	otherTuple, ok := other.(*Tuple)
	if !ok {
		return false
	}

	addr := reflect.ValueOf(tuple).Pointer()
	otherAddr := reflect.ValueOf(otherTuple).Pointer()

	if len(tuple.elements) != len(otherTuple.elements) {
		return false
	}

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	if addr == otherAddr {
		return true
	}

	//check that all elements are equal
	for i, v := range tuple.elements {
		if !v.Equal(ctx, otherTuple.elements[i], alreadyCompared, depth+1) {
			return false
		}
	}

	return true
}

func (p *OrderedPair) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	otherPair, ok := other.(*OrderedPair)
	if !ok {
		return false
	}

	addr := reflect.ValueOf(p).Pointer()
	otherAddr := reflect.ValueOf(otherPair).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	if addr == otherAddr {
		return true
	}

	//check that all elements are equal
	for i, v := range p {
		if !v.Equal(ctx, otherPair[i], alreadyCompared, depth+1) {
			return false
		}
	}

	return true
}

func (s *Struct) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	otherStruct, ok := other.(*Struct)
	if !ok || s.structType != otherStruct.structType {
		return false
	}

	addr := reflect.ValueOf(s).Pointer()
	otherAddr := reflect.ValueOf(otherStruct).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	if addr == otherAddr {
		return true
	}

	for i, e := range s.values {
		if !otherStruct.values[i].Equal(ctx, e, alreadyCompared, depth+1) {
			return false
		}
	}

	return true
}

func (slice *RuneSlice) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	otherSlice, ok := other.(*RuneSlice)
	if !ok {
		return false
	}

	if len(slice.elements) != len(otherSlice.elements) {
		return false
	}

	addr := reflect.ValueOf(slice).Pointer()
	otherAddr := reflect.ValueOf(otherSlice).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	for i, e := range slice.elements {
		if otherSlice.elements[i] != e {
			return false
		}
	}
	return true
}

func (slice *ByteSlice) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	otherVal, ok := other.(*ByteSlice)
	if !ok {
		return false
	}

	if len(slice.bytes) != len(otherVal.bytes) {
		return false
	}

	addr := reflect.ValueOf(slice).Pointer()
	otherAddr := reflect.ValueOf(otherVal).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	return bytes.Equal(slice.bytes, otherVal.bytes)
}

func (goFunc *GoFunction) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherGoFunc, ok := other.(*GoFunction)
	if !ok {
		return false
	}

	return reflect.ValueOf(goFunc.fn) == reflect.ValueOf(otherGoFunc.fn)
}

func (opt Option) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	otherOption, ok := other.(Option)
	if !ok {
		return false
	}

	return opt.Name == otherOption.Name && opt.Value != nil && opt.Value.Equal(ctx, otherOption.Value, alreadyCompared, depth+1)
}

func (pth Path) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPath, ok := other.(Path)
	if !ok {
		return false
	}
	return pth == otherPath
}

func (patt PathPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	other, ok := other.(PathPattern)
	if !ok {
		return false
	}
	return patt == other
}

func (u URL) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherURL, ok := other.(URL)
	if !ok {
		return false
	}
	return u == otherURL
}

func (host Host) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherHost, ok := other.(Host)
	if !ok {
		return false
	}
	return host == otherHost
}

func (scheme Scheme) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherScheme, ok := other.(Scheme)
	if !ok {
		return false
	}
	return scheme == otherScheme
}

func (patt HostPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(HostPattern)
	if !ok {
		return false
	}
	return patt == otherPatt
}

func (addr EmailAddress) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherAddr, ok := other.(EmailAddress)
	if !ok {
		return false
	}
	return addr == otherAddr
}

func (patt URLPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(URLPattern)
	if !ok {
		return false
	}
	return patt == otherPatt
}

func (i Identifier) Equal(ctx *Context, otherIdent Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIdent, ok := otherIdent.(Identifier)
	if !ok {
		return false
	}
	return i == otherIdent
}

func (p PropertyName) Equal(ctx *Context, otherName Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherName, ok := otherName.(PropertyName)
	if !ok {
		return false
	}
	return p == otherName
}

func (str CheckedString) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherStr, ok := other.(CheckedString)
	if !ok {
		return false
	}
	return str.matchingPattern != nil &&
		str.matchingPattern.Equal(ctx, otherStr.matchingPattern, alreadyCompared, depth+1) &&
		str.str == otherStr.str
}

func (count ByteCount) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherCount, ok := other.(ByteCount)
	if !ok {
		return false
	}
	return count == otherCount
}

func (count LineCount) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherCount, ok := other.(LineCount)
	if !ok {
		return false
	}
	return count == otherCount
}

func (count RuneCount) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherCount, ok := other.(RuneCount)
	if !ok {
		return false
	}
	return count == otherCount
}

func (rate ByteRate) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherRate, ok := other.(ByteRate)
	if !ok {
		return false
	}
	return rate == otherRate
}

func (rate SimpleRate) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherRate, ok := other.(SimpleRate)
	if !ok {
		return false
	}
	return rate == otherRate
}

func (d Duration) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherDuration, ok := other.(Duration)
	if !ok {
		return false
	}
	return d == otherDuration
}

func (y Year) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherYear, ok := other.(Year)
	if !ok {
		return false
	}
	return y == otherYear
}

func (d Date) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherDate, ok := other.(Date)
	if !ok {
		return false
	}
	return d == otherDate
}

func (d DateTime) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherDate, ok := other.(DateTime)
	if !ok {
		return false
	}
	return d == otherDate
}

func (m FileMode) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherMode, ok := other.(FileMode)
	if !ok {
		return false
	}
	return m == otherMode
}

func (r RuneRange) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherRange, ok := other.(RuneRange)
	if !ok {
		return false
	}
	return r == otherRange
}

func (r QuantityRange) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherRange, ok := other.(QuantityRange)
	if !ok {
		return false
	}

	if r.inclusiveEnd != otherRange.inclusiveEnd || r.unknownStart != otherRange.unknownStart {
		return false
	}

	if !r.unknownStart && !r.start.Equal(ctx, otherRange.start, alreadyCompared, depth+1) {
		return false
	}

	return r.end.Equal(ctx, otherRange.end, alreadyCompared, depth+1)
}

func (r IntRange) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherRange, ok := other.(IntRange)
	if !ok {
		return false
	}
	return r == otherRange
}

func (r FloatRange) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherRange, ok := other.(FloatRange)
	if !ok {
		return false
	}
	return r == otherRange
}

//patterns

func (pattern *ExactValuePattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPattern, ok := other.(*ExactValuePattern)
	if !ok {
		return false
	}

	return pattern.value.Equal(ctx, otherPattern.value, alreadyCompared, depth+1)
}

func (pattern *ExactStringPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPattern, ok := other.(*ExactValuePattern)
	if !ok {
		return false
	}

	return pattern.value.Equal(ctx, otherPattern.value, alreadyCompared, depth+1)
}

func (pattern *TypePattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPattern, ok := other.(*TypePattern)
	if !ok {
		return false
	}

	return pattern.Type == otherPattern.Type
}

func (pattern *DifferencePattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPattern, ok := other.(*DifferencePattern)
	if !ok {
		return false
	}

	return pattern.base.Equal(ctx, otherPattern.base, alreadyCompared, depth+1) && pattern.removed.Equal(ctx, otherPattern.removed, alreadyCompared, depth+1)
}

func (pattern *OptionalPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPattern, ok := other.(*OptionalPattern)
	if !ok {
		return false
	}

	return pattern.Pattern.Equal(ctx, otherPattern.Pattern, map[uintptr]uintptr{}, 0)
}

func (pattern *FunctionPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPattern, ok := other.(*FunctionPattern)
	if !ok {
		return false
	}

	return pattern.node == otherPattern.node
}

func (patt *RegexPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*RegexPattern)
	if !ok {
		return false
	}

	return patt.syntaxRegep.Equal(otherPatt.syntaxRegep)
	//return patt.regexp.String() == otherPatt.regexp.String()
}

func (patt *UnionPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*UnionPattern)
	if !ok {
		return false
	}

	if len(patt.cases) != len(otherPatt.cases) {
		return false
	}

	addr := reflect.ValueOf(patt).Pointer()
	otherAddr := reflect.ValueOf(otherPatt).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	for i, case_ := range patt.cases {
		if !case_.Equal(ctx, otherPatt.cases[i], alreadyCompared, depth+1) {
			return false
		}
	}
	return true
}

func (patt *IntersectionPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*IntersectionPattern)
	if !ok {
		return false
	}

	if len(patt.cases) != len(otherPatt.cases) {
		return false
	}

	addr := reflect.ValueOf(patt).Pointer()
	otherAddr := reflect.ValueOf(otherPatt).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	for i, case_ := range patt.cases {
		if !case_.Equal(ctx, otherPatt.cases[i], alreadyCompared, depth+1) {
			return false
		}
	}
	return true
}

func (patt *LengthCheckingStringPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*LengthCheckingStringPattern)
	if !ok {
		return false
	}

	return *patt == *otherPatt
}

func (patt *SequenceStringPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*SequenceStringPattern)
	if !ok {
		return false
	}

	if patt.HasRegex() && otherPatt.HasRegex() && patt.Regex() == otherPatt.Regex() {
		return true
	}

	if len(patt.elements) != len(otherPatt.elements) {
		return false
	}

	addr := reflect.ValueOf(patt).Pointer()
	otherAddr := reflect.ValueOf(otherPatt).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	for i, e := range patt.elements {
		if !e.Equal(ctx, otherPatt.elements[i], alreadyCompared, depth+1) {
			return false
		}
	}
	return true
}

func (patt *UnionStringPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*UnionStringPattern)
	if !ok {
		return false
	}

	if len(patt.cases) != len(otherPatt.cases) {
		return false
	}

	addr := reflect.ValueOf(patt).Pointer()
	otherAddr := reflect.ValueOf(otherPatt).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	for i, case_ := range patt.cases {
		if !case_.Equal(ctx, otherPatt.cases[i], alreadyCompared, depth+1) {
			return false
		}
	}
	return true
}

func (patt *RuneRangeStringPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*RuneRangeStringPattern)
	if !ok {
		return false
	}

	return patt.runes.Equal(ctx, otherPatt.runes, alreadyCompared, depth+1)
}

func (patt *IntRangePattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*IntRangePattern)
	if !ok {
		return false
	}

	return patt.intRange.Equal(ctx, otherPatt.intRange, alreadyCompared, depth+1)
}

func (patt *FloatRangePattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*FloatRangePattern)
	if !ok {
		return false
	}

	return patt.floatRange.Equal(ctx, otherPatt.floatRange, alreadyCompared, depth+1)
}

func (patt DynamicStringPatternElement) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*DynamicStringPatternElement)
	if !ok {
		return false
	}

	return patt.ctx == otherPatt.ctx && patt.name == otherPatt.name
}

func (patt *RepeatedPatternElement) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*RepeatedPatternElement)
	if !ok {
		return false
	}

	addr := reflect.ValueOf(patt).Pointer()
	otherAddr := reflect.ValueOf(otherPatt).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	if patt.HasRegex() && otherPatt.HasRegex() && patt.Regex() == otherPatt.Regex() {
		return true
	}

	return patt.exactCount == otherPatt.exactCount &&
		patt.ocurrenceModifier == otherPatt.ocurrenceModifier &&
		patt.element.Equal(ctx, otherPatt.element, alreadyCompared, depth+1)

}

func (patt *NamedSegmentPathPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*NamedSegmentPathPattern)

	if !ok {
		return false
	}

	return patt.node == otherPatt.node
}

func (patt *ObjectPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*ObjectPattern)
	if !ok {
		return false
	}

	if len(patt.entryPatterns) != len(otherPatt.entryPatterns) || patt.inexact != otherPatt.inexact {
		return false
	}

	addr := reflect.ValueOf(patt).Pointer()
	otherAddr := reflect.ValueOf(otherPatt).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	for i, v := range patt.entryPatterns {
		if !v.Equal(ctx, otherPatt.entryPatterns[i], alreadyCompared, depth+1) {
			return false
		}
	}
	return true
}

func (patt *RecordPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*RecordPattern)
	if !ok {
		return false
	}

	if len(patt.entryPatterns) != len(otherPatt.entryPatterns) || patt.inexact != otherPatt.inexact {
		return false
	}

	addr := reflect.ValueOf(patt).Pointer()
	otherAddr := reflect.ValueOf(otherPatt).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	for i, v := range patt.entryPatterns {
		if !v.Equal(ctx, otherPatt.entryPatterns[i], alreadyCompared, depth+1) {
			return false
		}
	}
	return true
}

func (patt *ListPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*ListPattern)
	if !ok {
		return false
	}

	if len(patt.elementPatterns) != len(otherPatt.elementPatterns) ||
		(patt.generalElementPattern == nil) != (otherPatt.generalElementPattern == nil) {
		return false
	}

	addr := reflect.ValueOf(patt).Pointer()
	otherAddr := reflect.ValueOf(otherPatt).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	if patt.elementPatterns != nil {
		for i, v := range patt.elementPatterns {
			if !v.Equal(ctx, otherPatt.elementPatterns[i], alreadyCompared, depth+1) {
				return false
			}
		}
		return true
	}

	return patt.generalElementPattern.Equal(ctx, otherPatt.generalElementPattern, alreadyCompared, depth+1)
}

func (patt *TuplePattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*TuplePattern)
	if !ok {
		return false
	}

	if len(patt.elementPatterns) != len(otherPatt.elementPatterns) ||
		(patt.generalElementPattern == nil) != (otherPatt.generalElementPattern == nil) {
		return false
	}

	addr := reflect.ValueOf(patt).Pointer()
	otherAddr := reflect.ValueOf(otherPatt).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	if patt.elementPatterns != nil {
		for i, v := range patt.elementPatterns {
			if !v.Equal(ctx, otherPatt.elementPatterns[i], alreadyCompared, depth+1) {
				return false
			}
		}
		return true
	}

	return patt.generalElementPattern.Equal(ctx, otherPatt.generalElementPattern, alreadyCompared, depth+1)
}

func (patt *OptionPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*OptionPattern)
	if !ok {
		return false
	}

	addr := reflect.ValueOf(patt).Pointer()
	otherAddr := reflect.ValueOf(otherPatt).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	return patt.name == otherPatt.name &&
		patt.value.Equal(ctx, otherPatt.value, alreadyCompared, depth+1)
}

func (patt *EventPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*EventPattern)
	if !ok {
		return false
	}

	addr := reflect.ValueOf(patt).Pointer()
	otherAddr := reflect.ValueOf(otherPatt).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	if (patt.ValuePattern == nil) != (otherPatt.ValuePattern == nil) {
		return false
	}
	if patt.ValuePattern == nil {
		return true
	}

	return patt.ValuePattern.Equal(ctx, otherPatt.ValuePattern, alreadyCompared, depth+1)
}

func (patt *MutationPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*MutationPattern)
	if !ok {
		return false
	}

	addr := reflect.ValueOf(patt).Pointer()
	otherAddr := reflect.ValueOf(otherPatt).Pointer()

	if alreadyCompared[addr] == otherAddr || alreadyCompared[otherAddr] == addr {
		//we return true to prevent cycling
		return true
	}

	alreadyCompared[addr] = otherAddr
	alreadyCompared[otherAddr] = addr

	return patt.kind == otherPatt.kind && patt.data0.Equal(ctx, otherPatt.data0, alreadyCompared, depth+1)
}

func (patt *ParserBasedPseudoPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*ParserBasedPseudoPattern)
	if !ok {
		return false
	}

	return patt == otherPatt
}

func (patt *IntRangeStringPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*IntRangeStringPattern)
	if !ok {
		return false
	}

	return patt.intRange.Equal(ctx, otherPatt.intRange, alreadyCompared, depth+1)
}

func (patt *FloatRangeStringPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*FloatRangeStringPattern)
	if !ok {
		return false
	}

	return patt.floatRange.Equal(ctx, otherPatt.floatRange, alreadyCompared, depth+1)
}

func (patt *PathStringPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPatt, ok := other.(*PathStringPattern)
	if !ok {
		return false
	}

	return patt.optionalPathPattern == otherPatt.optionalPathPattern
}

func (reader *Reader) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherReader, ok := other.(*Reader)
	if !ok {
		return false
	}

	return reader == otherReader
}

func (w *Writer) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherWriter, ok := other.(*Writer)
	if !ok {
		return false
	}
	return w == otherWriter
}

func (mt Mimetype) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherMimetype, ok := other.(Mimetype)
	if !ok {
		return false
	}

	return mt == otherMimetype
}
func (i FileInfo) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherInfo, ok := other.(FileInfo)
	if !ok {
		return false
	}

	return i.BaseName_ == otherInfo.BaseName_ &&
		i.AbsPath_ == otherInfo.AbsPath_ &&
		i.ModTime_.Equal(ctx, otherInfo.ModTime_, alreadyCompared, depth+1) &&
		i.Mode_ == otherInfo.Mode_ &&
		i.Size_ == otherInfo.Size_
}

func (r *LThread) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherLThread, ok := other.(*LThread)
	if !ok {
		return false
	}

	return r == otherLThread
}

func (g *LThreadGroup) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherGroup, ok := other.(*LThreadGroup)
	if !ok {
		return false
	}

	return g == otherGroup
}

func (fn *InoxFunction) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherFn, ok := other.(*InoxFunction)
	if !ok {
		return false
	}

	return fn == otherFn
}

func (b *Bytecode) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherBytecode, ok := other.(*Bytecode)
	if !ok {
		return false
	}

	return b == otherBytecode
}

func (it *KeyFilteredIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIterator, ok := other.(*KeyFilteredIterator)
	if !ok {
		return false
	}

	return it == otherIterator
}

func (it *ValueFilteredIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIterator, ok := other.(*ValueFilteredIterator)
	if !ok {
		return false
	}

	return it == otherIterator
}

func (it *KeyValueFilteredIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIterator, ok := other.(*KeyValueFilteredIterator)
	if !ok {
		return false
	}

	return it == otherIterator
}

func (it *indexableIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIterator, ok := other.(*indexableIterator)
	if !ok {
		return false
	}

	return it == otherIterator
}

func (it *ArrayIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIterator, ok := other.(*ArrayIterator)
	if !ok {
		return false
	}

	return it == otherIterator
}

func (it *immutableSliceIterator[T]) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIterator, ok := other.(*immutableSliceIterator[T])
	if !ok {
		return false
	}

	return it == otherIterator
}

func (it IntRangeIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIterator, ok := other.(IntRangeIterator)
	if !ok {
		return false
	}

	return it == otherIterator
}

func (it FloatRangeIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIterator, ok := other.(FloatRangeIterator)
	if !ok {
		return false
	}

	return it == otherIterator
}

func (it RuneRangeIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIterator, ok := other.(RuneRangeIterator)
	if !ok {
		return false
	}

	return it == otherIterator
}

func (it QuantityRangeIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIterator, ok := other.(QuantityRangeIterator)
	if !ok {
		return false
	}

	return it == otherIterator
}

func (it *PatternIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIterator, ok := other.(*PatternIterator)
	if !ok {
		return false
	}

	return it == otherIterator
}

func (it *indexedEntryIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIterator, ok := other.(*indexedEntryIterator)
	if !ok {
		return false
	}

	return it.i == otherIterator.i && it.len == otherIterator.len && SamePointer(it.entries, otherIterator.entries)
}

func (it *IpropsIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIterator, ok := other.(*IpropsIterator)
	if !ok {
		return false
	}

	return it == otherIterator
}

func (it *EventSourceIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIterator, ok := other.(*EventSourceIterator)
	if !ok {
		return false
	}

	return it.i == otherIterator.i
}

func (w *DirWalker) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherWalker, ok := other.(*DirWalker)
	if !ok {
		return false
	}
	return otherWalker == w
}

func (w *TreedataWalker) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherWalker, ok := other.(*TreedataWalker)
	if !ok {
		return false
	}
	return otherWalker == w
}

func (it *ValueListIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIt, ok := other.(*ValueListIterator)
	if !ok {
		return false
	}
	return otherIt == it
}

func (it *IntListIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIt, ok := other.(*IntListIterator)
	if !ok {
		return false
	}
	return otherIt == it
}

func (it *BitSetIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIt, ok := other.(*BitSetIterator)
	if !ok {
		return false
	}
	return otherIt == it
}

func (it *StrListIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIt, ok := other.(*StrListIterator)
	if !ok {
		return false
	}
	return otherIt == it
}

func (it *TupleIterator) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherIt, ok := other.(*TupleIterator)
	if !ok {
		return false
	}
	return otherIt == it
}

func (t Type) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherType, ok := other.(Type)
	if !ok {
		return false
	}
	return otherType == t
}

func (tx *Transaction) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherTx, ok := other.(*Transaction)
	if !ok {
		return false
	}
	return otherTx == tx
}

func (r *RandomnessSource) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherRandom, ok := other.(*RandomnessSource)
	if !ok {
		return false
	}
	return otherRandom == r
}

func (m *Mapping) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherMapping, ok := other.(*Mapping)
	if !ok {
		return false
	}
	return otherMapping == m
}

func (ns *PatternNamespace) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherNamespace, ok := other.(*PatternNamespace)
	if !ok {
		return false
	}
	return otherNamespace == ns
}

func (port Port) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPort, ok := other.(Port)
	if !ok {
		return false
	}
	return otherPort == port
}

func (u *Treedata) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherData, ok := other.(*Treedata)
	if !ok {
		return false
	}

	if (otherData.Root == nil) != (u.Root == nil) || !otherData.Root.Equal(ctx, u.Root, map[uintptr]uintptr{}, depth+1) {
		return false
	}

	if len(u.HiearchyEntries) != len(otherData.HiearchyEntries) {
		return false
	}

	for i, e := range u.HiearchyEntries {
		if !u.HiearchyEntries[i].Equal(ctx, e, map[uintptr]uintptr{}, depth+1) {
			return false
		}
	}

	return true
}

func (e TreedataHiearchyEntry) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherEntry, ok := other.(TreedataHiearchyEntry)
	if !ok {
		return false
	}

	if !otherEntry.Value.Equal(ctx, e.Value, map[uintptr]uintptr{}, depth+1) || len(e.Children) != len(otherEntry.Children) {
		return false
	}

	for i, e := range e.Children {
		if !otherEntry.Children[i].Equal(ctx, e, map[uintptr]uintptr{}, depth+1) {
			return false
		}
	}

	return true
}

func (c *StringConcatenation) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	strLike, ok := other.(StringLike)
	if !ok {
		return false
	}

	if strLike == nil || strLike.Len() != c.Len() {
		return false
	}

	switch val := strLike.(type) {
	case Str:
		i := 0
		for _, elem := range c.elements {
			substring := val[i : i+elem.Len()]
			if !elem.Equal(ctx, Str(substring), alreadyCompared, depth+1) {
				return false
			}
			i += elem.Len()
		}
	default:
		s := strLike.GetOrBuildString()
		i := 0
		for _, elem := range c.elements {
			substring := s[i : i+elem.Len()]
			if !elem.Equal(ctx, Str(substring), alreadyCompared, depth+1) {
				return false
			}
			i += elem.Len()
		}
	}

	return true
}

func (c *BytesConcatenation) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	bytesLike, ok := other.(BytesLike)
	if !ok {
		return false
	}

	if bytesLike == nil || bytesLike.Len() != c.Len() {
		return false
	}

	switch val := bytesLike.(type) {
	case *ByteSlice:
		i := 0
		for _, elem := range c.elements {
			subSlice := val.bytes[i : i+elem.Len()]
			if !elem.Equal(ctx, Str(subSlice), alreadyCompared, depth+1) {
				return false
			}
			i += elem.Len()
		}
	default:
		s := bytesLike.GetOrBuildBytes()
		i := 0
		for _, elem := range c.elements {
			subSlice := s.bytes[i : i+elem.Len()]
			if !elem.Equal(ctx, Str(subSlice), alreadyCompared, depth+1) {
				return false
			}
			i += elem.Len()
		}
	}

	return true
}

func (s *TestSuite) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otheSuite, ok := other.(*TestSuite)
	if !ok {
		return false
	}
	return Same(s, otheSuite)
}

func (c *TestCase) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherCase, ok := other.(*TestCase)
	if !ok {
		return false
	}
	return Same(c, otherCase)
}

func (r *TestCaseResult) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherResult, ok := other.(*TestCaseResult)
	if !ok {
		return false
	}
	//TODO: update
	return Same(r, otherResult)
}

func (c *DynamicValue) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	return c.Resolve(ctx).Equal(ctx, other, alreadyCompared, depth)
}

func (e *Event) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherEvent, ok := other.(*Event)
	if !ok {
		return false
	}

	//TODO: also  affected resource list ?
	return e.time == otherEvent.time && e.value.Equal(ctx, otherEvent.value, alreadyCompared, depth+1)
}

func (s *ExecutedStep) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherStep, ok := other.(*ExecutedStep)
	if !ok {
		return false
	}

	//TODO: also  affected resource list ?
	return s == otherStep
}

func (j *LifetimeJob) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherJob, ok := other.(*LifetimeJob)
	if !ok {
		return false
	}

	//TODO: also  affected resource list ?
	return j == otherJob
}

func Same(a, b Value) bool {
	aVal := reflect.ValueOf(a)
	bVal := reflect.ValueOf(b)

	if aVal.Type() != bVal.Type() {
		return false
	}

	switch aVal.Kind() {
	case reflect.Pointer, reflect.Map:
		return aVal.Pointer() == bVal.Pointer()
	case reflect.Slice:
		return aVal.Pointer() == bVal.Pointer() && aVal.Len() == bVal.Len()
	case reflect.Bool:
		return aVal.Bool() == bVal.Bool()
	default:
		if aVal.CanInt() {
			return aVal.Int() == bVal.Int()
		}
		if aVal.CanFloat() {
			return aVal.Float() == bVal.Float()
		}
	}

	return false
}

func (w *GenericWatcher) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherWatcher, ok := other.(*GenericWatcher)
	if !ok {
		return false
	}

	return w == otherWatcher
}

func (w *PeriodicWatcher) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherWatcher, ok := other.(*PeriodicWatcher)
	if !ok {
		return false
	}

	return w == otherWatcher
}

func (m Mutation) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherMutation, ok := other.(Mutation)
	if !ok {
		return false
	}

	return m.Kind == otherMutation.Kind &&
		m.Complete == otherMutation.Complete &&
		m.DataElementLengths == otherMutation.DataElementLengths &&
		bytes.Equal(m.Data, otherMutation.Data)
}

func (w *joinedWatchers) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherWatcher, ok := other.(*joinedWatchers)
	if !ok {
		return false
	}

	return w == otherWatcher
}

func (w stoppedWatcher) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	_, ok := other.(stoppedWatcher)
	return ok
}

func (s *wrappedWatcherStream) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherStream, ok := other.(*wrappedWatcherStream)
	return ok && s == otherStream
}

func (s *ElementsStream) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherStream, ok := other.(*ElementsStream)
	return ok && s == otherStream
}

func (s *ReadableByteStream) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherStream, ok := other.(*ReadableByteStream)
	return ok && s == otherStream
}

func (s *WritableByteStream) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherStream, ok := other.(*WritableByteStream)
	return ok && s == otherStream
}

func (s *ConfluenceStream) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherStream, ok := other.(*ConfluenceStream)
	return ok && s == otherStream
}

func (c Color) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherColor, ok := other.(*Color)
	if !ok {
		return false
	}

	//TODO: return true if equivalent colors ?

	return c.data == otherColor.data && c.encodingId == otherColor.encodingId
}

func (r *RingBuffer) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherBuf, ok := other.(*RingBuffer)
	return ok && r == otherBuf
}

func (c *DataChunk) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherChunk, ok := other.(*DataChunk)
	return ok && c == otherChunk
}

func (d *StaticCheckData) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherData, ok := other.(*StaticCheckData)
	return ok && d == otherData
}

func (d *SymbolicData) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherData, ok := other.(*SymbolicData)
	return ok && d == otherData
}

func (m *Module) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherMod, ok := other.(*Module)
	return ok && m == otherMod
}

func (s *GlobalState) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherState, ok := other.(*GlobalState)
	return ok && s == otherState
}

func (msg Message) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherMessage, ok := other.(Message)
	return ok && msg == otherMessage
}

func (s *Subscription) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherSub, ok := other.(*Subscription)

	return ok && s == otherSub
}

func (p *Publication) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPublication, ok := other.(*Publication)

	return ok && p == otherPublication
}

func (h *ValueHistory) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherHistory, ok := other.(*ValueHistory)

	return ok && h == otherHistory
}

func (h *SynchronousMessageHandler) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherHandler, ok := other.(*SynchronousMessageHandler)

	return ok && h == otherHandler
}

func (g *SystemGraph) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	other, ok := other.(*SystemGraph)

	return ok && g == other
}

func (n *SystemGraphNodes) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherNodes, ok := other.(*SystemGraphNodes)

	return ok && n == otherNodes
}

func (g *SystemGraphNode) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherGraph, ok := other.(*SystemGraphNode)

	return ok && g == otherGraph
}

func (e SystemGraphEvent) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherEvent, ok := other.(SystemGraphEvent)

	return ok && e == otherEvent
}

func (e SystemGraphEdge) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherEdge, ok := other.(SystemGraphEdge)

	return ok && e == otherEdge
}

func (s *Secret) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	return false
}

func (s *SecretPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherPattern, ok := other.(*SecretPattern)
	if !ok {
		return false
	}

	return s == otherPattern
}

func (e *XMLElement) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	//TODO: implement
	otherElem, ok := other.(*XMLElement)
	if !ok {
		return false
	}

	return e == otherElem
}

func (db *DatabaseIL) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherDB, ok := other.(*DatabaseIL)
	if !ok {
		return false
	}

	return db == otherDB
}

func (api *ApiIL) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherAPI, ok := other.(*ApiIL)
	if !ok {
		return false
	}

	return api == otherAPI
}

func (ns *Namespace) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	otherNS, ok := other.(*Namespace)
	return ok && ns == otherNS
}

func (p *StructPattern) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	otherPatt, ok := other.(*StructPattern)
	return ok && p == otherPatt
}

func (s *FilesystemSnapshotIL) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	if depth > MAX_COMPARISON_DEPTH {
		return false
	}

	otherPatt, ok := other.(*FilesystemSnapshotIL)
	return ok && s == otherPatt
}

func (t *CurrentTest) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	other, ok := other.(*CurrentTest)

	return ok && t == other
}

func (p *TestedProgram) Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	other, ok := other.(*TestedProgram)

	return ok && p == other
}
