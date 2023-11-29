package fs_ns

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/prettyprint"

	pprint "github.com/inoxlang/inox/internal/prettyprint"
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

func (s *LSPSession) Test(v symbolic.Value, state symbolic.RecTestCallState) bool {
	state.StartCall()
	defer state.FinishCall()

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

func (s *LSPSession) Prop(name string) symbolic.Value {
	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(symbolic.FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*LSPSession) PropertyNames() []string {
	return LSP_SESSION_PROPNAMES
}

func (r *LSPSession) PrettyPrint(w prettyprint.PrettyPrintWriter, config *pprint.PrettyPrintConfig) {
	w.WriteName("lsp-session")
}

func (r *LSPSession) WidestOfType() symbolic.Value {
	return ANY_LSP_SESSION
}

func (s *LSPSession) IsMutable() bool {
	return true
}
