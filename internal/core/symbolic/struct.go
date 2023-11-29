package symbolic

import (
	"bytes"
	"errors"

	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/oklog/ulid/v2"
)

var (
	_ IProps = (*Struct)(nil)

	ANY_STRUCT         = &Struct{}
	ANY_STRUCT_PATTERN = &StructPattern{}
)

// A Struct represents a symbolic Struct.
type Struct struct {
	structType  *StructPattern //if nil matches any struct
	fieldValues map[string]Value
}

func NewStruct(structType *StructPattern, fieldValues map[string]Value) *Struct {
	return &Struct{structType: structType, fieldValues: fieldValues}
}

func (s *Struct) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherStruct, ok := v.(*Struct)
	if !ok {
		return false
	}

	if s.structType == nil {
		return true
	}

	if s.structType.tempId == otherStruct.structType.tempId {
		return false
	}

	return true
}

func (s *Struct) Prop(name string) Value {
	if s.structType == nil {
		return ANY
	}

	fieldValue, ok := s.fieldValues[name]
	if ok {
		return fieldValue
	}

	if fieldType, ok := s.structType.typeOfField(name); ok {
		return fieldType.SymbolicValue()
	} else {
		panic(FormatErrPropertyDoesNotExist(name, s))
	}
}

func (s *Struct) PropertyNames() []string {
	return s.structType.keys
}

func (s *Struct) SetProp(name string, value Value) (IProps, error) {
	fieldType, ok := s.structType.typeOfField(name)
	if !ok {
		return nil, FormatErrPropertyDoesNotExist(name, s)
	}
	if !fieldType.TestValue(value, RecTestCallState{}) {
		return nil, errors.New(fmtNotAssignableToPropOfType(value, fieldType))
	}

	return s, nil
}

func (s *Struct) WithExistingPropReplaced(name string, value Value) (IProps, error) {
	panic(ErrNotImplementedYet)
}

func (s *Struct) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	if s.structType != nil {
		w.WriteName("struct")
		w.WriteString(s.structType.name)
		w.WriteString(" {")
	} else {
		w.WriteName("struct {")
	}

	if w.Depth > config.MaxDepth {
		if len(s.structType.keys) > 0 {
			w.WriteString("{(...)}")
		} else {
			w.WriteString(" }")
		}
		return
	}

	indentCount := w.ParentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	propertyNames := s.PropertyNames()
	for i, name := range propertyNames {

		if !config.Compact {
			w.WriteLFCR()
			w.WriteBytes(indent)
		}

		if config.Colorize {
			w.WriteBytes(config.Colors.IdentifierLiteral)
		}

		w.WriteString(name)

		if config.Colorize {
			w.WriteAnsiReset()
		}

		//colon
		w.WriteColonSpace()

		//value
		v := s.Prop(name)
		v.PrettyPrint(w.IncrDepthWithIndent(indentCount), config)

		//comma & indent
		isLastEntry := i == len(propertyNames)-1

		if !isLastEntry {
			w.WriteCommaSpace()
		}
	}

	if !config.Compact && len(propertyNames) > 0 {
		w.WriteLFCR()
	}

	w.WriteManyBytes(bytes.Repeat(config.Indent, w.Depth), []byte{'}'})
}

func (s *Struct) WidestOfType() Value {
	return ANY_STRUCT
}

// A StructPattern represents a symbolic StructPattern.
type StructPattern struct {
	name   string //empty if anonymous
	tempId ulid.ULID
	keys   []string //if nil matches any
	types  []Pattern
}

func NewStructPattern(name string, id ulid.ULID, keys []string, types []Pattern) *StructPattern {
	patt := CreateStructPattern(name, id, keys, types)
	return &patt
}

// CreateStructPattern does not return a pointer on purpose.
func CreateStructPattern(name string, id ulid.ULID, keys []string, types []Pattern) StructPattern {
	//it's okay if severals StructPattern are created with the same id since
	//they should logically have the same name, keys & types.

	if keys == nil {
		keys = []string{}
	}
	if types == nil {
		types = []Pattern{}
	}
	return StructPattern{
		name:   name,
		tempId: id,
		keys:   keys,
		types:  types,
	}
}

func (p *StructPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherStructPattern, ok := v.(*StructPattern)
	if !ok {
		return false
	}

	if p.keys == nil {
		return true
	}

	return p.tempId == otherStructPattern.tempId
}

func (s *StructPattern) typeOfField(name string) (Pattern, bool) {
	ind, ok := s.indexOfField(name)
	if !ok {
		return nil, false
	}
	return s.types[ind], true
}

func (s *StructPattern) indexOfField(name string) (int, bool) {
	for index, key := range s.keys {
		if key == name {
			return index, true
		}
	}
	return -1, false
}

func (s *StructPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("struct-type ")
	w.WriteString(s.name)
	w.WriteString(" {")

	w.WriteString("...")
	w.WriteByte('}')
}

func (s *StructPattern) WidestOfType() Value {
	return ANY_STRUCT_PATTERN
}
