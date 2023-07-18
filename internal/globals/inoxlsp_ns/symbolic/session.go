package fs_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"

	pprint "github.com/inoxlang/inox/internal/pretty_print"

	"github.com/inoxlang/inox/internal/utils"
)

var (
	LSP_SESSION_PROPNAMES = []string{}

	ANY_LSP_SESSION = &LSPSession{}

	_true  = core.Bool(true)
	_false = core.Bool(false)

	_ symbolic.PotentiallySharable = (*LSPSession)(nil)
)

type LSPSession struct {
	symbolic.UnassignablePropsMixin
	shared *core.Bool
	_      int
}

func (s *LSPSession) Test(v symbolic.SymbolicValue) bool {
	other, ok := v.(*LSPSession)
	if !ok {
		return false
	}
	if s.shared == nil {
		return true
	}
	if other.shared == nil {
		return false
	}

	return *s.shared == *other.shared
}

func (s *LSPSession) IsSharable() (bool, string) {
	return true, ""
}

func (s *LSPSession) Share(originState *symbolic.State) symbolic.PotentiallySharable {
	return &LSPSession{
		shared: &_true,
	}
}

func (s *LSPSession) IsShared() bool {
	return s.shared != nil && bool(*s.shared)
}

func (s *LSPSession) GetGoMethod(name string) (*symbolic.GoFunction, bool) {
	return nil, false
}

func (s *LSPSession) Prop(name string) symbolic.SymbolicValue {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(symbolic.FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*LSPSession) PropertyNames() []string {
	return LSP_SESSION_PROPNAMES
}

func (r *LSPSession) Widen() (symbolic.SymbolicValue, bool) {
	return nil, false
}

func (s *LSPSession) IsWidenable() bool {
	return false
}

func (r *LSPSession) PrettyPrint(w *bufio.Writer, config *pprint.PrettyPrintConfig, depth int, parentIndentCount int) {
	utils.Must(w.Write(utils.StringAsBytes("%lsp-session")))
}

func (r *LSPSession) WidestOfType() symbolic.SymbolicValue {
	return ANY_LSP_SESSION
}

func (s *LSPSession) IsMutable() bool {
	return true
}
