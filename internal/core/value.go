package core

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strconv"
	"time"

	"github.com/inoxlang/inox/internal/core/symbolic"
	parse "github.com/inoxlang/inox/internal/parse"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	NO_SCHEME_SCHEME_NAME = "noscheme"
	NO_SCHEME_SCHEME      = NO_SCHEME_SCHEME_NAME + "://"
)

var (
	ErrNotResourceName = errors.New("not a resource name")
)

// Value is the interface implemented by all values accessible to Inox code.
type Value interface {
	// IsMutable should return true if the value is definitively mutable and false if it is definitively immutable.
	IsMutable() bool

	Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool

	//human readable representation
	PrettyPrint(w *bufio.Writer, config *PrettyPrintConfig, depth int, parentIndentCount int)

	//ToSymbolicValue should return a symbolic value that represents the value.
	ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.SymbolicValue) (symbolic.SymbolicValue, error)
}

// A resource name is a string value that designates a resource, examples: URL, Path & Host are resource names.
// The meaning of resource is broad and should not be confused with HTTP Resources.
type ResourceName interface {
	WrappedString
	ResourceName() string
}

func ResourceNameFrom(s string) ResourceName {
	n, _ := parse.ParseExpression(s)

	switch n.(type) {
	case *parse.HostLiteral, *parse.AbsolutePathLiteral, *parse.RelativePathLiteral, *parse.URLLiteral:
		return utils.Must(evalSimpleValueLiteral(n.(parse.SimpleValueLiteral), nil)).(ResourceName)
	}

	panic(fmt.Errorf("%q is not a valid resource name", s))
}

type NilT int

const Nil = NilT(0)

func (n NilT) String() string {
	return "nil"
}

type Bool bool

const (
	True  = Bool(true)
	False = Bool(false)
)

type FileMode fs.FileMode

func (m FileMode) FileMode() fs.FileMode {
	return fs.FileMode(m)
}

func (m FileMode) Executable() bool {
	return m&0o111 != 0
}

// ---------------------------

func IsIndexKey(key string) bool {
	//TODO: number of implicit keys will be soon limited so this function should be refactored to only check for integers
	// with a small number of digits.
	_, err := strconv.ParseUint(key, 10, 32)
	return err == nil
}

func SamePointer(a, b interface{}) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

func IsSimpleInoxVal(v Value) bool {
	switch v.(type) {
	case NilT, Rune, Byte, Str, Bool, Int, Float, WrappedString, Port:
		return true
	default:
		return false
	}
}

func IsSimpleInoxValOrOption(v Value) bool {
	if IsSimpleInoxVal(v) {
		return true
	}
	_, ok := v.(Option)
	return ok
}

// ValOf any reflect.Value that wraps a Inox value.
// Wraps its argument in a reflect.Value if it is not a Inox value.
func ValOf(v interface{}) Value {
	if val, ok := v.(Value); ok {
		return val
	}
	switch val := v.(type) {
	case bool:
		return Bool(val)
	case rune:
		return Rune(val)
	case byte:
		return Byte(val)
	case int:
		return Int(val)
	case int64:
		return Int(val)
	case float64:
		return Float(val)
	case string:
		return Str(val)
	case parse.Node:
		return AstNode{Node: val}
	case time.Duration:
		return Duration(val)
	case GoValue:
		return val
	default:
		rval := reflect.ValueOf(val)

		switch rval.Kind() {
		case reflect.Func:
			return &GoFunction{fn: rval.Interface(), kind: GoFunc}
		case reflect.Pointer:
			if rval.Type().Implements(ERROR_INTERFACE_TYPE) {
				return NewError(rval.Interface().(error), Nil)
			}

			rval = rval.Elem()
			fallthrough
		default:
			if !rval.IsValid() {
				return Nil
			}

			if rval.Type().Implements(ERROR_INTERFACE_TYPE) {
				return NewError(rval.Interface().(error), Nil)
			}
		}
		panic(fmt.Errorf("cannot convert a value of type %T to a Inox value", val))
	}
}

func ToValueList[T Value](arg []T) []Value {
	values := make([]Value, len(arg))
	for i, val := range arg {
		values[i] = val
	}
	return values
}

func ToSerializableSlice(values []Value) []Serializable {
	serializable := make([]Serializable, len(values))
	for i, val := range values {
		values[i] = val.(Serializable)
	}
	return serializable
}

func ToSerializableValueMap(valMap map[string]Value) map[string]Serializable {
	serializable := make(map[string]Serializable, len(valMap))
	for k, val := range valMap {
		serializable[k] = val.(Serializable)
	}
	return serializable
}

func coerceToBool(val Value) bool {
	reflVal := reflect.ValueOf(val)

	if !reflVal.IsValid() {
		return false
	}

	switch v := val.(type) {
	case Indexable:
		return v.Len() > 0
	}

	if reflVal.Type() == NIL_TYPE {
		return false
	}

	switch reflVal.Kind() {
	case reflect.String:
		return reflVal.Len() != 0
	case reflect.Slice:
		return reflVal.Len() != 0
	case reflect.Chan, reflect.Map:
		return !reflVal.IsNil() && reflVal.Len() != 0
	case reflect.Func, reflect.Pointer, reflect.UnsafePointer, reflect.Interface:
		return !reflVal.IsNil()
	default:
		return true
	}
}

type Port struct {
	Number uint16
	Scheme Scheme
}

type Type struct {
	reflect.Type
}
