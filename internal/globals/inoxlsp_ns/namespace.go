package inoxlsp_ns

import (
	"errors"
	"reflect"

	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	symbolic_inoxlsp "github.com/inoxlang/inox/internal/globals/inoxlsp_ns/symbolic"
	. "github.com/inoxlang/inox/internal/utils"

	"github.com/inoxlang/inox/internal/lsp/jsonrpc"

	lsp "github.com/inoxlang/inox/internal/lsp"
)

var (
	LSP_SESSION_PATTERN = &core.TypePattern{
		Name:          "inoxlsp.session",
		Type:          reflect.TypeOf((*LSPSession)(nil)),
		SymbolicValue: symbolic_inoxlsp.ANY_LSP_SESSION,
	}
)

func init() {
	core.RegisterSymbolicGoFunction(StartLspServer, func(ctx *symbolic.Context, config *symbolic.Object) *symbolic.Error {
		return nil
	})

	core.RegisterDefaultPatternNamespace("inoxlsp", &core.PatternNamespace{
		Patterns: map[string]core.Pattern{
			"session": LSP_SESSION_PATTERN,
		},
	})
}

func NewInoxLspNamespace() *core.Record {
	return core.NewRecordFromMap(core.ValMap{
		"start_websocket_server": core.WrapGoFunction(StartLspServer),
	})
}

func StartLspServer(ctx *core.Context, config *core.Object) error {
	state := ctx.GetClosestState()
	childCtx := ctx.BoundChild()

	var host core.Host
	var cert string
	var certKey string
	var onSessionHandler *core.InoxFunction

	err := config.ForEachEntry(func(k string, v core.Value) error {
		//TODO: add more checks + symbolic checks

		switch k {
		case "host":
			host = v.(core.Host)
		case "on-session":
			onSessionHandler = v.(*core.InoxFunction)
			if ok, msg := onSessionHandler.IsSharable(state); !ok {
				return errors.New("on-session handler function is not sharable " + msg)
			}
			onSessionHandler.Share(state)
		case "certificate":
			cert = v.(core.StringLike).GetOrBuildString()
		case "certiticate-key":
			certKey = v.(*core.Secret).StringValue().GetOrBuildString()
		}
		return nil
	})

	if err != nil {
		return err
	}

	if onSessionHandler == nil {
		return core.FmtMissingArgument("missing on-session handler function")
	}

	return lsp.StartLSPServer(childCtx, lsp.LSPServerOptions{
		Websocket: &lsp.WebsocketOptions{
			Addr:                  host.WithoutScheme(),
			Certificate:           cert,
			CertificatePrivateKey: certKey,
		},
		UseContextLogger: true,
		RemoteFS:         true,
		OnSession: func(rpcCtx *core.Context, s *jsonrpc.Session) error {
			mainFs := fs_ns.NewMemFilesystem(lsp.DEFAULT_MAX_IN_MEM_FS_STORAGE_SIZE)
			fls := lsp.NewFilesystem(mainFs, nil)

			file := Must(fls.Create("/main.ix"))
			Must(file.Write([]byte("manifest {\n\n}")))
			PanicIfErr(file.Close())

			sessionCtx := core.NewContext(core.ContextConfig{
				Permissions:          rpcCtx.GetGrantedPermissions(),
				ForbiddenPermissions: rpcCtx.GetForbiddenPermissions(),

				ParentContext: rpcCtx,
				Filesystem:    fls,
			})
			tempState := core.NewGlobalState(sessionCtx)
			tempState.Logger = state.Logger
			tempState.Out = state.Out
			s.SetContextOnce(sessionCtx)

			lspSession := NewLspSession(s)
			_, err := onSessionHandler.Call(tempState, core.Nil, []core.Value{lspSession}, nil)
			if err != nil {
				tempState.Logger.Err(err).Send()
			}
			return nil
		},
	})
}
