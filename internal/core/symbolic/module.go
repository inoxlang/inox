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
	MainChunk             *parse.ParsedChunk // if nil, any module is matched
	InclusionStatementMap map[*parse.InclusionImportStatement]*IncludedChunk
}

func NewModule(chunk *parse.ParsedChunk, inclusionStatementMap map[*parse.InclusionImportStatement]*IncludedChunk) *Module {
	return &Module{
		MainChunk:             chunk,
		InclusionStatementMap: inclusionStatementMap,
	}
}

func (mod *Module) Name() string {
	return mod.MainChunk.Name()
}

func (mod *Module) GetLineColumn(node parse.Node) (int32, int32) {
	return mod.MainChunk.GetLineColumn(node)
}

func (m *Module) Test(v SymbolicValue) bool {
	otherMod, ok := v.(*Module)

	if !ok {
		return false
	}
	if m.MainChunk == nil {
		return true
	}

	return m.MainChunk == otherMod.MainChunk && reflect.ValueOf(m.InclusionStatementMap).Pointer() == reflect.ValueOf(otherMod.InclusionStatementMap).Pointer()
}

func (m *Module) Widen() (SymbolicValue, bool) {
	return ANY_MODULE, false
}

func (m *Module) IsWidenable() bool {
	return m.MainChunk != nil
}

func (m *Module) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%module")))
	return
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
