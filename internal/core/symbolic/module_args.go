package symbolic

import (
	"errors"
	"fmt"
	"sort"

	"maps"

	"github.com/inoxlang/inox/internal/inoxconsts"
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
	return args.typ.paramNames
}

func (args *ModuleArgs) SetProp(state *State, node parse.Node, name string, value Value) (IProps, error) {
	fieldType, ok := args.typ.typeOfParam(name)
	if !ok {
		return nil, FormatErrPropertyDoesNotExist(name, args)
	}

	if !fieldType.TestValue(value, RecTestCallState{evalState: state.resetTestCallMsgBuffers()}) {
		msg := utils.Ret0(fmtNotAssignableToPropOfType(state.fmtHelper, value, fieldType, state.testCallMessageBuffer))
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
		if len(args.typ.paramNames) > 0 {
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
	parameters []ModuleParameter //[ positional ..., non positional ...] if nil any ModuleParamsPattern is matched
	paramNames []string
}

func NewModuleParamsPattern(params []ModuleParameter) *ModuleParamsPattern {
	patt := CreateModuleParamsPattern(params)
	return &patt
}

// CreateModuleParamsPattern does not return a pointer on purpose.
// Passed parameters are sorted.
func CreateModuleParamsPattern(params []ModuleParameter) ModuleParamsPattern {
	sort.Slice(params, func(i, j int) bool {
		if !params[i].Positional {

			if params[j].Positional {
				return false
			}
			return params[i].Name < params[j].Name
		}
		return params[i].Index < params[j].Index
	})

	paramNames := make([]string, len(params))
	for i, p := range params {
		paramNames[i] = p.Name
	}

	return ModuleParamsPattern{
		parameters: params,
		paramNames: paramNames,
	}
}

func (p *ModuleParamsPattern) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherStructPattern, ok := v.(*ModuleParamsPattern)
	if !ok {
		return false
	}

	if p.parameters == nil {
		return true
	}

	if otherStructPattern.parameters == nil || len(p.parameters) != len(otherStructPattern.parameters) {
		return false
	}

	for i, param := range p.parameters {
		if otherStructPattern.parameters[i].Name != param.Name {
			return false
		}
		if !deeplyMatch(otherStructPattern.parameters[i].Pattern, param.Pattern) {
			return false
		}
	}

	return true
}

func (p *ModuleParamsPattern) typeOfParam(name string) (Pattern, bool) {
	for _, param := range p.parameters {
		if param.Name == name {
			return param.Pattern, true
		}
	}
	return nil, false
}

func (p *ModuleParamsPattern) ArgumentsObject() *Object {
	entries := map[string]Serializable{}
	static := map[string]Pattern{}

	var positionalParams []Serializable

	firstNonPositionlParamIndex := len(p.parameters)

	for i, param := range p.parameters {
		if !param.Positional {
			firstNonPositionlParamIndex = i
			break
		}
		arg, ok := AsSerializable(param.Pattern.SymbolicValue()).(Serializable)
		if !ok {
			arg = ANY_SERIALIZABLE
		}
		positionalParams = append(positionalParams, arg)
	}

	for _, param := range p.parameters[firstNonPositionlParamIndex:] {
		arg, ok := AsSerializable(param.Pattern.SymbolicValue()).(Serializable)
		if !ok {
			arg = ANY_SERIALIZABLE
		}
		entries[param.Name] = arg
		static[param.Name] = param.Pattern
	}

	if len(positionalParams) > 0 {
		entries[inoxconsts.IMPLICIT_PROP_NAME] = NewList(positionalParams...)
		//We do not add the implicit property name if there is no positional parameters
		//because this would require the developer to add "": [] property.
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
