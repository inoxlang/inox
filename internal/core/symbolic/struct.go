package symbolic

import (
	"bufio"
	"bytes"
	"errors"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
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
	fieldValues map[string]SymbolicValue
}

func NewStruct(structType *StructPattern, fieldValues map[string]SymbolicValue) *Struct {
	return &Struct{structType: structType, fieldValues: fieldValues}
}

func (s *Struct) Test(v SymbolicValue) bool {
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

func (s *Struct) Prop(name string) SymbolicValue {
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

func (s *Struct) SetProp(name string, value SymbolicValue) (IProps, error) {
	fieldType, ok := s.structType.typeOfField(name)
	if !ok {
		return nil, FormatErrPropertyDoesNotExist(name, s)
	}
	if !fieldType.TestValue(value) {
		return nil, errors.New(fmtNotAssignableToPropOfType(value, fieldType))
	}

	return s, nil
}

func (s *Struct) WithExistingPropReplaced(name string, value SymbolicValue) (IProps, error) {
	panic(ErrNotImplementedYet)
}

func (s *Struct) Widen() (SymbolicValue, bool) {
	if s.IsWidenable() {
		return ANY_STRUCT, true
	}
	return nil, false
}

func (s *Struct) IsWidenable() bool {
	return s.structType != nil
}

func (s *Struct) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	if s.structType != nil {
		utils.Must(w.Write(utils.StringAsBytes("struct")))
		utils.Must(w.Write(utils.StringAsBytes(s.structType.name)))
		utils.Must(w.Write(utils.StringAsBytes(" {")))
	} else {
		utils.Must(w.Write(utils.StringAsBytes("struct {")))
	}

	if depth > config.MaxDepth {
		if len(s.structType.keys) > 0 {
			utils.Must(w.Write(utils.StringAsBytes("{(...)}")))
		} else {
			utils.Must(w.Write(utils.StringAsBytes(" }")))
		}
		return
	}

	indentCount := parentIndentCount + 1
	indent := bytes.Repeat(config.Indent, indentCount)

	propertyNames := s.PropertyNames()
	for i, name := range propertyNames {

		if !config.Compact {
			utils.Must(w.Write(LF_CR))
			utils.Must(w.Write(indent))
		}

		if config.Colorize {
			utils.Must(w.Write(config.Colors.IdentifierLiteral))
		}

		utils.Must(w.Write(utils.StringAsBytes(name)))

		if config.Colorize {
			utils.Must(w.Write(ANSI_RESET_SEQUENCE))
		}

		//colon
		utils.Must(w.Write(COLON_SPACE))

		//value
		v := s.Prop(name)
		v.PrettyPrint(w, config, depth+1, indentCount)

		//comma & indent
		isLastEntry := i == len(propertyNames)-1

		if !isLastEntry {
			utils.Must(w.Write(COMMA_SPACE))
		}
	}

	if !config.Compact && len(propertyNames) > 0 {
		utils.Must(w.Write(LF_CR))
	}

	utils.MustWriteMany(w, bytes.Repeat(config.Indent, depth), []byte{'}'})
}

func (s *Struct) WidestOfType() SymbolicValue {
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

func (p *StructPattern) Test(v SymbolicValue) bool {
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

func (s *StructPattern) Widen() (SymbolicValue, bool) {
	return nil, false
}

func (s *StructPattern) IsWidenable() bool {
	return false
}

func (s *StructPattern) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("struct-type ")))
	utils.Must(w.Write(utils.StringAsBytes(s.name)))
	utils.Must(w.Write(utils.StringAsBytes(" {")))

	utils.Must(w.Write(utils.StringAsBytes("...")))
	w.WriteByte('}')
}

func (s *StructPattern) WidestOfType() SymbolicValue {
	return ANY_STRUCT_PATTERN
}
