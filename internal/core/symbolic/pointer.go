package symbolic

import (
	"github.com/inoxlang/inox/internal/prettyprint"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	ANY_POINTER = &Pointer{}

	_ = Value((*Pointer)(nil))
)

type Pointer struct {
	typ   *PointerType //if nil any pointer is matcher
	value Value        //only a few value types are allowed
}

func newPointer(ptrType *PointerType) *Pointer {
	return &Pointer{
		typ:   ptrType,
		value: ptrType.valueType.SymbolicValue(),
	}
}

func (p *Pointer) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherPtr, ok := v.(*Pointer)
	if !ok {
		return false
	}
	if p.typ == nil {
		return true
	}
	return p.typ.Equal(otherPtr.typ, state) && p.value.Test(otherPtr.value, state)
}

func (p *Pointer) Type() *PointerType {
	return p.typ
}

func (p *Pointer) ValueType() CompileTimeType {
	return p.typ.ValueType()
}

func (*Pointer) WidestOfType() Value {
	return ANY_POINTER
}

func (p *Pointer) PrettyPrint(w prettyprint.PrettyPrintWriter, config *prettyprint.PrettyPrintConfig) {
	w.WriteByte('*')
	p.value.PrettyPrint(w.ZeroIndent(), config)
}

type PointerType struct {
	valueType CompileTimeType

	pointer *Pointer
}

func newPointerType(valueType CompileTimeType) *PointerType {
	t := &PointerType{
		valueType: valueType,
	}
	t.pointer = newPointer(t)
	return t
}

func (t *PointerType) Equal(v CompileTimeType, state RecTestCallState) bool {
	otherPtrType, ok := v.(*PointerType)
	return ok && t.valueType.Equal(otherPtrType, RecTestCallState{})
}

func (t *PointerType) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	ptr, ok := v.(*Pointer)
	return ok && t.Equal(ptr.typ, state)
}

func (t *PointerType) ValueType() CompileTimeType {
	return t.valueType
}

func (t *PointerType) SymbolicValue() Value {
	return t.pointer
}

func (t *PointerType) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteString("pointer-type(")
	t.valueType.PrettyPrint(w.WithDepthIndent(w.Depth+1, 0), config)
	w.WriteByte(')')
}
