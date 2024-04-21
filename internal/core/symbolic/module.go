package symbolic

import (
	"errors"
	"reflect"

	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/prettyprint"
)

var (
	MODULE_PROP_NAMES      = []string{"parsing-errors", "main-chunk-node"}
	ANY_MODULE             = &Module{}
	SOURCE_POSITION_RECORD = NewInexactRecord(map[string]Serializable{
		"source": ANY_STR_LIKE,
		"line":   ANY_INT,
		"column": ANY_INT,
	}, nil)
)

// A Module represents a symbolic Module.
type Module struct {
	mainChunk               *parse.ParsedChunkSource // if nil, any module is matched
	inclusionStatementMap   map[*parse.InclusionImportStatement]*IncludedChunk
	directlyImportedModules map[*parse.ImportStatement]*Module
}

func NewModule(
	chunk *parse.ParsedChunkSource,
	inclusionStatementMap map[*parse.InclusionImportStatement]*IncludedChunk,
	importedModuleMap map[*parse.ImportStatement]*Module,
) *Module {
	return &Module{
		mainChunk:               chunk,
		inclusionStatementMap:   inclusionStatementMap,
		directlyImportedModules: importedModuleMap,
	}
}

func (mod *Module) Name() string {
	return mod.mainChunk.Name()
}

func (mod *Module) GetLineColumn(node parse.Node) (int32, int32) {
	return mod.mainChunk.GetLineColumn(node)
}

func (m *Module) Test(v Value, state RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

	otherMod, ok := v.(*Module)

	if !ok {
		return false
	}
	if m.mainChunk == nil {
		return true
	}

	return m.mainChunk == otherMod.mainChunk && reflect.ValueOf(m.inclusionStatementMap).Pointer() == reflect.ValueOf(otherMod.inclusionStatementMap).Pointer()
}

func (m *Module) PrettyPrint(w pprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("module")
}

func (m *Module) WidestOfType() Value {
	return ANY_MODULE
}

func (m *Module) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (m *Module) Prop(name string) Value {
	switch name {
	case "parsing-errors":
		return NewTupleOf(NewError(SOURCE_POSITION_RECORD))
	case "main-chunk-node":
		return ANY_AST_NODE
	}
	return GetGoMethodOrPanic(name, m)
}

func (m *Module) SetProp(state *State, node parse.Node, name string, value Value) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(m))
}

func (m *Module) WithExistingPropReplaced(state *State, name string, value Value) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(m))
}

func (*Module) PropertyNames() []string {
	return MODULE_PROP_NAMES
}

type IncludedChunk struct {
	*parse.ParsedChunkSource
}

type ModuleParameter struct {
	Name       string
	Pattern    Pattern
	Positional bool
	Index      int //set if positional
}

func getModuleParameters(manifestObject *Object, manifestObjectLiteral *parse.ObjectLiteral) []ModuleParameter {
	parametersDesc, _, ok := manifestObject.GetProperty(extData.MANIFEST_PARAMS_SECTION_NAME)
	if !ok {
		return nil
	}

	obj, ok := parametersDesc.(*Object)
	if !ok {
		return nil
	}

	moduleParams := []ModuleParameter{}

	parametersNode, _ := manifestObjectLiteral.PropValue(extData.MANIFEST_PARAMS_SECTION_NAME)
	parametersObjectNode, ok := parametersNode.(*parse.ObjectLiteral)

	if !ok {
		return nil
	}

	var noKeyProperties *List
	if obj.hasProperty(inoxconsts.IMPLICIT_PROP_NAME) {
		noKeyProperties = obj.Prop(inoxconsts.IMPLICIT_PROP_NAME).(*List)
	}

	var positionalParamIndex = 0

	for _, prop := range parametersObjectNode.Properties {
		if prop.HasNoKey() { //positional parameter
			paramDesc, ok := noKeyProperties.ElementAt(positionalParamIndex).(*Object)
			index := positionalParamIndex
			positionalParamIndex++

			if !ok {
				return nil
			}

			paramNameVal, _, _ := paramDesc.GetProperty(extData.MANIFEST_POSITIONAL_PARAM_NAME_FIELD)
			paramName, ok := paramNameVal.(*Identifier)
			if !ok || !paramName.HasConcreteName() {
				return nil
			}

			paramPatternVal, _, _ := paramDesc.GetProperty(extData.MANIFEST_POSITIONAL_PARAM_PATTERN_FIELD)

			paramPattern, ok := paramPatternVal.(Pattern)
			if !ok {
				return nil
			}

			moduleParams = append(moduleParams, ModuleParameter{
				Name:       paramName.Name(),
				Pattern:    paramPattern,
				Positional: true,
				Index:      index,
			})
		} else { //non-positional parameter
			paramName := prop.Name()
			propValue := obj.Prop(paramName)

			switch val := propValue.(type) {
			case *OptionPattern:
				moduleParams = append(moduleParams, ModuleParameter{
					Name:    paramName,
					Pattern: val.pattern,
				})
			case Pattern:
				moduleParams = append(moduleParams, ModuleParameter{
					Name:    paramName,
					Pattern: val,
				})
			case *Object:
				paramDesc := val

				paramDesc.ForEachEntry(func(k string, v Value) error {
					switch k {
					case extData.MANIFEST_POSITIONAL_PARAM_PATTERN_FIELD:
						moduleParams = append(moduleParams, ModuleParameter{
							Name:    paramName,
							Pattern: v.(Pattern),
						})
					}
					return nil
				})
			}
		}
	}

	return moduleParams
}
