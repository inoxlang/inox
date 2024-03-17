package inoxlsp_ns

import (
	"bufio"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"

	"github.com/inoxlang/inox/internal/core/symbolic"

	symbolic_inox_ns "github.com/inoxlang/inox/internal/globals/inoxlsp_ns/symbolic"
)

var (
	LSP_SESSION_PROPNAMES = []string{}

	_ core.PotentiallySharable = (*LSPSession)(nil)
)

type LSPSession struct {
	rpcSession *jsonrpc.Session
	lock       core.SmartLock
}

func NewLspSession(rpcSession *jsonrpc.Session) *LSPSession {
	return &LSPSession{
		rpcSession: rpcSession,
	}
}

func (s *LSPSession) IsSharable(originState *core.GlobalState) (bool, string) {
	return true, ""
}

func (s *LSPSession) Share(originState *core.GlobalState) {
	s.lock.Share(originState, func() {

	})
}

func (s *LSPSession) IsShared() bool {
	return s.lock.IsValueShared()
}

func (s *LSPSession) SmartLock(state *core.GlobalState) {
	s.lock.Lock(state, s, true)
}

func (s *LSPSession) SmartUnlock(state *core.GlobalState) {
	s.lock.Unlock(state, s, true)
}

func (s *LSPSession) Prop(ctx *core.Context, name string) core.Value {
	state := ctx.MustGetClosestState()
	s.lock.Lock(state, s)
	defer s.lock.Unlock(state, s)

	method, ok := s.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, s))
	}
	return method
}

func (*LSPSession) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*LSPSession) PropertyNames(ctx *core.Context) []string {
	return LSP_SESSION_PROPNAMES
}

func (s *LSPSession) GetGoMethod(name string) (*core.GoFunction, bool) {
	return nil, false
}

func (evs *LSPSession) ToSymbolicValue(ctx *core.Context, encountered map[uintptr]symbolic.Value) (symbolic.Value, error) {
	return symbolic_inox_ns.ANY_LSP_SESSION, nil
}

func (s *LSPSession) IsMutable() bool {
	return true
}

func (s *LSPSession) Equal(ctx *core.Context, other core.Value, alreadyCompared map[uintptr]uintptr, depth int) bool {
	otherSession, ok := other.(*LSPSession)
	if !ok {
		return false
	}
	return s == otherSession
}

func (s *LSPSession) PrettyPrint(w *bufio.Writer, config *core.PrettyPrintConfig, depth int, parentIndentCount int) {
	state := config.Context.MustGetClosestState()
	s.lock.Lock(state, s)
	defer s.lock.Unlock(state, s)

	core.PrintType(w, s)
}
