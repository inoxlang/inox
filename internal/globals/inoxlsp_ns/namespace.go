package inoxlsp_ns

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/inoxlang/inox/internal/commonfmt"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	symbolic_inoxlsp "github.com/inoxlang/inox/internal/globals/inoxlsp_ns/symbolic"
	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"

	"github.com/inoxlang/inox/internal/projectserver"
)

const (
	NAMESPACE_NAME = "inoxlsp"
)

var (
	LSP_SESSION_PATTERN = &core.TypePattern{
		Name:          NAMESPACE_NAME + ".session",
		Type:          reflect.TypeOf((*LSPSession)(nil)),
		SymbolicValue: symbolic_inoxlsp.ANY_LSP_SESSION,
	}
)

func init() {
	core.RegisterSymbolicGoFunction(StartLspServer, func(ctx *symbolic.Context, config *symbolic.Object) *symbolic.Error {
		return nil
	})

	core.RegisterDefaultPatternNamespace(NAMESPACE_NAME, &core.PatternNamespace{
		Patterns: map[string]core.Pattern{
			"session": LSP_SESSION_PATTERN,
		},
	})
}

func NewInoxLspNamespace() *core.Namespace {
	return core.NewNamespace(NAMESPACE_NAME, map[string]core.Value{
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
	var projectsDir core.Path

	err := config.ForEachEntry(func(k string, v core.Serializable) error {
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
		case "projects-dir":
			projectsDir = v.(core.Path)
			if !projectsDir.IsDirPath() {
				return fmt.Errorf("%s should be a directory path", k)
			}
		}
		return nil
	})

	if err != nil {
		return err
	}

	if onSessionHandler == nil {
		return commonfmt.FmtMissingArgument("missing on-session handler function")
	}

	return projectserver.StartLSPServer(childCtx, projectserver.LSPServerConfiguration{
		Websocket: &projectserver.WebsocketServerConfiguration{
			Addr:                  host.WithoutScheme(),
			Certificate:           cert,
			CertificatePrivateKey: certKey,
		},
		UseContextLogger:      true,
		ProjectMode:           true,
		ProjectsDir:           projectsDir,
		ProjectsDirFilesystem: ctx.GetFileSystem(),
		OnSession: func(rpcCtx *core.Context, s *jsonrpc.Session) error {
			sessionCtx := core.NewContext(core.ContextConfig{
				Permissions:          rpcCtx.GetGrantedPermissions(),
				ForbiddenPermissions: rpcCtx.GetForbiddenPermissions(),

				ParentContext: rpcCtx,
			})
			tempState := core.NewGlobalState(sessionCtx)
			tempState.Logger = state.Logger
			tempState.Out = state.Out
			tempState.OutputFieldsInitialized.Store(true)

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
