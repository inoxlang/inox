package symbolic

import (
	"bufio"
	"errors"
	"reflect"

	parse "github.com/inoxlang/inox/internal/parse"
	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	MODULE_PROP_NAMES      = []string{"parsing_errors", "main_chunk_node"}
	ANY_MODULE             = &Module{}
	SOURCE_POSITION_RECORD = NewRecord(map[string]Serializable{
		"source": ANY_STR_LIKE,
		"line":   ANY_INT,
		"column": ANY_INT,
	})
)

// A Module represents a symbolic Module.
type Module struct {
	mainChunk               *parse.ParsedChunk // if nil, any module is matched
	inclusionStatementMap   map[*parse.InclusionImportStatement]*IncludedChunk
	directlyImportedModules map[*parse.ImportStatement]*Module
}

func NewModule(
	chunk *parse.ParsedChunk,
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

func (m *Module) Test(v SymbolicValue) bool {
	otherMod, ok := v.(*Module)

	if !ok {
		return false
	}
	if m.mainChunk == nil {
		return true
	}

	return m.mainChunk == otherMod.mainChunk && reflect.ValueOf(m.inclusionStatementMap).Pointer() == reflect.ValueOf(otherMod.inclusionStatementMap).Pointer()
}

func (m *Module) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%module")))
}

func (m *Module) WidestOfType() SymbolicValue {
	return ANY_MODULE
}

func (m *Module) GetGoMethod(name string) (*GoFunction, bool) {
	return nil, false
}

func (m *Module) Prop(name string) SymbolicValue {
	switch name {
	case "parsing_errors":
		return NewTupleOf(NewError(SOURCE_POSITION_RECORD))
	case "main_chunk_node":
		return &AstNode{}
	}
	return GetGoMethodOrPanic(name, m)
}

func (m *Module) SetProp(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(m))
}

func (m *Module) WithExistingPropReplaced(name string, value SymbolicValue) (IProps, error) {
	return nil, errors.New(FmtCannotAssignPropertyOf(m))
}

func (*Module) PropertyNames() []string {
	return MODULE_PROP_NAMES
}

type IncludedChunk struct {
	*parse.ParsedChunk
}
