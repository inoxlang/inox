package internal

import (
	"reflect"
	"unsafe"
)

type FastId [4]uintptr

const (
	INT_TYPE_ID = iota + 1
	FLOAT_TYPE_ID
	BOOL_TYPE_ID
	STR_TYPE_ID
	URL_TYPE_ID
	HOST_TYPE_ID
	CHECKEDSTR_TYPE_ID
	LIST_TYPE_ID
	OBJECT_TYPE_ID
	TUPLE_TYPE_ID
	RECORD_TYPE_ID
	RUNE_SLICE_TYPE_ID
	OBJECTPATTERN_TYPE_ID
	LISTPATTERN_TYPE_ID
	DIFFPATTERN_TYPE_ID
	OPTPATTERN_TYPE_ID
	NAMED_SEGMENT_PATH_PATTERN_TYPE_ID
	SEQ_STR_PATTERN_TYPE_ID
	UNION_STR_PATTERN_TYPE_ID
	UNION_PATTERN_TYPE_ID
	INOX_FN_TYPE_ID
	GO_FN_TYPE_ID

	//TODO: support more types
)

func FastIdOf(ctx *Context, v Value) (FastId, bool) {
	switch val := v.(type) {
	case Int:
		return FastId{INT_TYPE_ID, uintptr(val), 0}, true
	case Float:
		return FastId{FLOAT_TYPE_ID, uintptr(val), 0}, true
	case Bool:
		if val {
			return FastId{BOOL_TYPE_ID, 1, 0}, true
		}
		return FastId{BOOL_TYPE_ID, 0, 0}, true
	case Str:
		header := (*reflect.StringHeader)(unsafe.Pointer(&val))
		return FastId{STR_TYPE_ID, header.Data, uintptr(header.Len)}, true
	case URL:
		header := (*reflect.StringHeader)(unsafe.Pointer(&val))
		return FastId{URL_TYPE_ID, header.Data, uintptr(header.Len)}, true
	case Host:
		header := (*reflect.StringHeader)(unsafe.Pointer(&val))
		return FastId{HOST_TYPE_ID, header.Data, uintptr(header.Len)}, true
	case CheckedString:
		header := (*reflect.StringHeader)(unsafe.Pointer(&val.str))
		return FastId{CHECKEDSTR_TYPE_ID, header.Data, uintptr(header.Len)}, true
	case *List:
		return FastId{LIST_TYPE_ID, reflect.ValueOf(v).Pointer()}, true
	case *Object:
		return FastId{OBJECT_TYPE_ID, reflect.ValueOf(v).Pointer()}, true
	case *Tuple:
		return FastId{TUPLE_TYPE_ID, reflect.ValueOf(v).Pointer()}, true
	case *Record:
		return FastId{RECORD_TYPE_ID, reflect.ValueOf(v).Pointer()}, true
	case *RuneSlice:
		return FastId{RUNE_SLICE_TYPE_ID, reflect.ValueOf(v).Pointer()}, true
	case *ObjectPattern:
		return FastId{OBJECTPATTERN_TYPE_ID, reflect.ValueOf(v).Pointer()}, true
	case *ListPattern:
		return FastId{LISTPATTERN_TYPE_ID, reflect.ValueOf(v).Pointer()}, true
	case *DifferencePattern:
		return FastId{DIFFPATTERN_TYPE_ID, reflect.ValueOf(v).Pointer()}, true
	case *OptionPattern:
		return FastId{OPTPATTERN_TYPE_ID, reflect.ValueOf(v).Pointer()}, true
	case *NamedSegmentPathPattern:
		return FastId{NAMED_SEGMENT_PATH_PATTERN_TYPE_ID, reflect.ValueOf(v).Pointer()}, true
	case *SequenceStringPattern:
		return FastId{SEQ_STR_PATTERN_TYPE_ID, reflect.ValueOf(v).Pointer()}, true
	case *UnionStringPattern:
		return FastId{UNION_STR_PATTERN_TYPE_ID, reflect.ValueOf(v).Pointer()}, true
	case *UnionPattern:
		return FastId{UNION_PATTERN_TYPE_ID, reflect.ValueOf(v).Pointer()}, true
	case *InoxFunction:
		return FastId{INOX_FN_TYPE_ID, reflect.ValueOf(v).Pointer()}, true
	case *GoFunction:
		return FastId{GO_FN_TYPE_ID, reflect.ValueOf(v).Pointer()}, true
	}
	return FastId{}, false
}
