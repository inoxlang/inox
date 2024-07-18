package core

import (
	"bufio"
	"fmt"
	"io/fs"
	"reflect"
	"time"

	"github.com/inoxlang/inox/internal/ast"
	pprint "github.com/inoxlang/inox/internal/prettyprint"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core/symbolic"
)

var (
	_ = []IProps{(*Object)(nil), (*Record)(nil), (*Namespace)(nil), (*Dictionary)(nil), (*List)(nil)}
)

func init() {
	RegisterSymbolicGoFunction(NewArray, func(ctx *symbolic.Context, elements ...symbolic.Value) *symbolic.Array {
		return symbolic.NewArray(elements...)
	})
}

// Value is the interface implemented by all values accessible to Inox code.
// A value should either be definitively mutable or definitively immutable.
type Value interface {
	// IsMutable should return true if the value is definitively mutable and false if it is definitively immutable.
	IsMutable() bool

	Equal(ctx *Context, other Value, alreadyCompared map[uintptr]uintptr, depth int) bool

	//human readable representation
	PrettyPrint(ctx *Context, w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int)

	//ToSymbolicValue should return a symbolic value that represents the value.
	ToSymbolicValue(ctx *Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error)
}

// NilT implements Value.
type NilT int

const Nil = NilT(0)

func (n NilT) String() string {
	return "nil"
}

// FileMode implements Value.
type FileMode fs.FileMode

func FileModeFrom(ctx *Context, firstArg Value) FileMode {
	integer, ok := firstArg.(Int)
	if !ok {
		panic(commonfmt.FmtErrInvalidArgumentAtPos(0, "should be an integer"))
	}

	return FileMode(integer)
}

func (m FileMode) FileMode() fs.FileMode {
	return fs.FileMode(m)
}

func (m FileMode) Executable() bool {
	return m&0o111 != 0
}

// ---------------------------

func SamePointer(a, b interface{}) bool {
	return reflect.ValueOf(a).Pointer() == reflect.ValueOf(b).Pointer()
}

func IsSimpleInoxVal(v Value) bool {
	switch v.(type) {
	case NilT, Rune, Byte, String, Bool, Int, Float, GoString, Port:
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
		return String(val)
	case ast.Node:
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

func ToValueAsserted(v any) Value {
	return v.(Value)
}

func ToSerializableAsserted(v any) Serializable {
	return v.(Serializable)
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

// Port implements Value. Inox's port literals (e.g. `:80`, `:80/http`) evaluate to a Port.
type Port struct {
	Number uint16
	Scheme Scheme //set to NO_SCHEME_SCHEME_NAME if no scheme is specified.
}

type Type struct {
	reflect.Type
}
