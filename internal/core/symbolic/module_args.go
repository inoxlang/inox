package symbolic

import (
	"errors"
	"fmt"

	"maps"

	"github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	_ IProps = (*ModuleArgs)(nil)

	ANY_MODULE_ARGS   = &ModuleArgs{}
	ANY_MODULE_PARAMS = &ModuleParamsPattern{}
)

// A ModuleArgs represents a symbolic ModuleArgs.
type ModuleArgs struct {
	typ    *ModuleParamsPattern //if nil matches any module args
	values map[string]Value
}

func NewModuleArgs(paramsPattern *ModuleParamsPattern, values map[string]Value) *ModuleArgs {
	return &ModuleArgs{typ: paramsPattern, values: values}
}

func (args *ModuleArgs) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherStruct, ok := v.(*ModuleArgs)
	if !ok {
		return false
	}

	if args.typ == nil {
		return true
	}

	return args.typ.Test(otherStruct.typ, state)
}

func (args *ModuleArgs) Prop(name string) Value {
	if args.typ == nil {
		return ANY
	}

	fieldValue, ok := args.values[name]
	if ok {
		return fieldValue
	}

	if paramType, ok := args.typ.typeOfParam(name); ok {
		return paramType.SymbolicValue()
	} else {
		panic(FormatErrPropertyDoesNotExist(name, args))
	}
}

func (args *ModuleArgs) PropertyNames() []string {
	return args.typ.keys
}

func (args *ModuleArgs) SetProp(state *State, node parse.Node, name string, value Value) (IProps, error) {
	fieldType, ok := args.typ.typeOfParam(name)
	if !ok {
		return nil, FormatErrPropertyDoesNotExist(name, args)
	}
	if !fieldType.TestValue(value, RecTestCallState{}) {
		msg := utils.Ret0(fmtNotAssignableToPropOfType(state.fmtHelper, value, fieldType))
		return nil, errors.New(msg)
	}

	return args, nil
}

func (args *ModuleArgs) WithExistingPropReplaced(state *State, name string, value Value) (IProps, error) {
	fields := maps.Clone(args.values)
	fields[name] = value

	result := &ModuleArgs{
		values: fields,
		typ:    args.typ,
	}

	if args.typ == nil {
		return result, nil
	}

	pattern, ok := args.typ.typeOfParam(name)
	if !ok {
		return nil, fmt.Errorf("cannot replace value inexisting property %s", name)
	}
	if !pattern.TestValue(value, RecTestCallState{}) {
		return nil, fmt.Errorf("cannot update property %s with a non matching value", name)
	}

	return result, nil
}

func (args *ModuleArgs) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w = w.IncrDepth()
	w.WriteName("module-arguments")

	if w.Depth > config.MaxDepth {
		if len(args.typ.keys) > 0 {
			w.WriteString("{(...)}")
		} else {
			w.WriteString("{ }")
		}
		return
	}
	w.WriteString("{ ")

	propertyNames := args.PropertyNames()
	for i, name := range propertyNames {

		if !config.Compact {
			w.WriteEndOfLine()
			w.WriteInnerIndent()
		}

		if config.Colorize {
			w.WriteBytes(config.Colors.IdentifierLiteral)
		}

		w.WriteString(name)

		if config.Colorize {
			w.WriteAnsiReset()
		}

		//colon
		w.WriteString(": ")

		//value
		v := args.Prop(name)
		v.PrettyPrint(w.IncrIndent(), config)

		//comma & indent
		isLastEntry := i == len(propertyNames)-1

		if !isLastEntry {
			w.WriteString(", ")
		}
	}

	if !config.Compact && len(propertyNames) > 0 {
		w.WriteEndOfLine()
	}

	w.WriteOuterIndent()
	w.WriteByte('}')
}

func (args *ModuleArgs) WidestOfType() Value {
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

func (p *ModuleParamsPattern) typeOfParam(name string) (Pattern, bool) {
	ind, ok := p.indexOfParam(name)
	if !ok {
		return nil, false
	}
	return p.types[ind], true
}

func (p *ModuleParamsPattern) indexOfParam(name string) (int, bool) {
	for index, key := range p.keys {
		if key == name {
			return index, true
		}
	}
	return -1, false
}

func (p *ModuleParamsPattern) ArgumentsObject() *Object {
	entries := map[string]Serializable{}
	static := map[string]Pattern{}

	for _, paramName := range p.keys {
		pattern := utils.MustGet(p.typeOfParam(paramName))

		arg, ok := AsSerializable(pattern.SymbolicValue()).(Serializable)
		if !ok {
			arg = ANY_SERIALIZABLE
		}

		entries[paramName] = arg
		static[paramName] = pattern
	}

	return NewInexactObject(entries, nil, static)
	//Note: returning an exact object would be better but the arguments object passed in the imported configuration
	//may not be exact.
}

func (p *ModuleParamsPattern) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("module-parameters{")

	w.WriteString("...")
	w.WriteByte('}')
}

func (p *ModuleParamsPattern) WidestOfType() Value {
	return ANY_MODULE_PARAMS
}
