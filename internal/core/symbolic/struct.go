package symbolic

import pprint "github.com/inoxlang/inox/internal/prettyprint"

var (
	ANY_STRUCT_TYPE = &StructType{}
	ANY_STRUCT      = &Struct{typ: ANY_STRUCT_TYPE}

	_ = Value((*Struct)(nil))
)

// A Struct represents a symbolic Struct.
type Struct struct {
	typ *StructType
}

func newStruct(t *StructType) *Struct {
	return &Struct{typ: t}
}

func (s *Struct) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherStruct, ok := v.(*Struct)
	if !ok {
		return false
	}
	return ok && s.typ.Equal(otherStruct.typ, state)
}

func (s *Struct) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteString("struct{")

	for _, field := range s.typ.fields {
		w.WriteString(field.Name)
		w.WriteByte(' ')
		field.Type.PrettyPrint(w.WithDepthIndent(w.Depth, 0), config)
		w.WriteString("; ")
	}

	w.WriteByte('}')
}

func (s *Struct) WidestOfType() Value {
	return ANY_STRUCT
}

// StructType represents a struct type, it implements CompileTimeType.
type StructType struct {
	name    string        //can be empty
	fields  []StructField //if nil any StructType is matched
	methods []StructMethod

	value *Struct
}

func NewStructType(name string, fields []StructField, methods []StructMethod) *StructType {
	t := &StructType{
		name:    name,
		fields:  fields,
		methods: methods,
	}

	t.value = newStruct(t)
	return t
}

type StructField struct {
	Name string
	Type CompileTimeType
}

type StructMethod struct {
	Name  string
	Value *InoxFunction
}

func (t *StructType) Name() (string, bool) {
	return t.name, t.name != ""
}

func (t *StructType) FieldCount() int {
	return len(t.fields)
}

// Field returns the field at index in the definition order.
func (t *StructType) Field(index int) StructField {
	return t.fields[index]
}

// Fields returns the underyling field slice, in definition order.
// The slice should NOT be modified.
func (t *StructType) Fields() []StructField {
	return t.fields
}

func (t *StructType) FieldByName(name string) (StructField, bool) {
	for _, field := range t.fields {
		if field.Name == name {
			return field, true
		}
	}
	return StructField{}, false
}

func (t *StructType) Method(index int) StructMethod {
	return t.methods[index]
}

func (t *StructType) MethodCount() int {
	return len(t.fields)
}

func (t *StructType) Equal(v CompileTimeType, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherStructType, ok := v.(*StructType)
	if !ok {
		return false
	}

	if t.fields == nil {
		return true
	}

	return otherStructType == t
}

func (t *StructType) TestValue(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	struct_, ok := v.(*Struct)
	if !ok {
		return false
	}
	return ok && struct_.typ == t
}

func (t *StructType) SymbolicValue() Value {
	return &Struct{typ: t}
}

func (t *StructType) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteString("struct-type{")
	w.WriteString("...")
	w.WriteByte('}')
}
