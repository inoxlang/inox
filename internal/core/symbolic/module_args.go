package symbolic

import (
	"bytes"
	"errors"

	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	_ IProps = (*ModuleArgs)(nil)

	ANY_MODULE_ARGS   = &ModuleArgs{}
	ANY_MODULE_PARAMS = &ModuleParamsPattern{}
)

// A ModuleArgs represents a symbolic ModuleArgs.
type ModuleArgs struct {
	typ         *ModuleParamsPattern //if nil matches any module args
	fieldValues map[string]Value
}

func NewModuleArgs(paramsPattern *ModuleParamsPattern, fieldValues map[string]Value) *ModuleArgs {
	return &ModuleArgs{typ: paramsPattern, fieldValues: fieldValues}
}

func (s *ModuleArgs) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherStruct, ok := v.(*ModuleArgs)
	if !ok {
		return false
	}

	if s.typ == nil {
		return true
	}

	return s.typ.Test(otherStruct.typ, state)
}

func (s *ModuleArgs) Prop(name string) Value {
	if s.typ == nil {
		return ANY
	}

	fieldValue, ok := s.fieldValues[name]
	if ok {
		return fieldValue
	}

	if fieldType, ok := s.typ.typeOfField(name); ok {
		return fieldType.SymbolicValue()
	} else {
		panic(FormatErrPropertyDoesNotExist(name, s))
	}
}

func (s *ModuleArgs) PropertyNames() []string {
	return s.typ.keys
}

func (s *ModuleArgs) SetProp(name string, value Value) (IProps, error) {
	fieldType, ok := s.typ.typeOfField(name)
	if !ok {
		return nil, FormatErrPropertyDoesNotExist(name, s)
	}
	if !fieldType.TestValue(value, RecTestCallState{}) {
		return nil, errors.New(fmtNotAssignableToPropOfType(value, fieldType))
	}

	return s, nil
}

func (s *ModuleArgs) WithExistingPropReplaced(name string, value Value) (IProps, error) {
	panic(ErrNotImplementedYet)
}

func (s *ModuleArgs) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("module-arguments")

	if w.Depth > config.MaxDepth {
		if len(s.typ.keys) > 0 {
			w.WriteString("{(...)}")
		} else {
			w.WriteString("{ }")
		}
		return
	}
	w.WriteString("{ ")

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

func (s *ModuleArgs) WidestOfType() Value {
	return ANY_MODULE_ARGS
}

// A ModuleParamsPattern represents a symbolic ModuleParamsPattern.
type ModuleParamsPattern struct {
	keys  []string //if nil matches any
	types []Pattern
}

func NewModuleParamsPattern(keys []string, types []Pattern) *ModuleParamsPattern {
	patt := CreateModuleParamsPattern(keys, types)
	return &patt
}

// CreateModuleParamsPattern does not return a pointer on purpose.
func CreateModuleParamsPattern(keys []string, types []Pattern) ModuleParamsPattern {
	if keys == nil {
		keys = []string{}
	}
	if types == nil {
		types = []Pattern{}
	}
	return ModuleParamsPattern{
		keys:  keys,
		types: types,
	}
}

func (p *ModuleParamsPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherStructPattern, ok := v.(*ModuleParamsPattern)
	if !ok {
		return false
	}

	if p.keys == nil {
		return true
	}

	if otherStructPattern.keys == nil || len(p.keys) != len(otherStructPattern.keys) {
		return false
	}

	for i, key := range p.keys {
		if otherStructPattern.keys[i] != key {
			return false
		}
	}

	for i, patt := range p.types {
		if !deeplyMatch(otherStructPattern.types[i], patt) {
			return false
		}
	}

	return true
}

func (s *ModuleParamsPattern) typeOfField(name string) (Pattern, bool) {
	ind, ok := s.indexOfField(name)
	if !ok {
		return nil, false
	}
	return s.types[ind], true
}

func (s *ModuleParamsPattern) indexOfField(name string) (int, bool) {
	for index, key := range s.keys {
		if key == name {
			return index, true
		}
	}
	return -1, false
}

func (s *ModuleParamsPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("module-parameters{")

	w.WriteString("...")
	w.WriteByte('}')
}

func (s *ModuleParamsPattern) WidestOfType() Value {
	return ANY_MODULE_PARAMS
}
