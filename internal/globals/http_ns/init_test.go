package http_ns

import (
	"errors"
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/permbase"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/html_ns"
	http_ns "github.com/inoxlang/inox/internal/globals/http_ns/symbolic"
)

func init() {
	port.Store(10_000)
	if !core.AreDefaultScriptLimitsSet() {
		core.SetDefaultScriptLimits([]core.Limit{})
	}

	if core.NewDefaultContext == nil {
		core.SetNewDefaultContext(func(config core.DefaultContextConfig) (*core.Context, error) {

			if len(config.OwnedDatabases) != 0 {
				panic(errors.New("not supported"))
			}

			permissions := []core.Permission{
				core.GlobalVarPermission{Kind_: permbase.Use, Name: "*"},
				core.GlobalVarPermission{Kind_: permbase.Create, Name: "*"},
				core.GlobalVarPermission{Kind_: permbase.Read, Name: "*"},
				core.LThreadPermission{Kind_: permbase.Create},
				core.FilesystemPermission{Kind_: permbase.Read, Entity: core.PathPattern("/...")},
			}

			permissions = append(permissions, config.Permissions...)

			ctx := core.NewContext(core.ContextConfig{
				Permissions:          permissions,
				ForbiddenPermissions: config.ForbiddenPermissions,
				HostDefinitions:      config.HostDefinitions,
				ParentContext:        config.ParentContext,
			})

			for k, v := range core.DEFAULT_NAMED_PATTERNS {
				ctx.AddNamedPattern(k, v)
			}

			for k, v := range core.DEFAULT_PATTERN_NAMESPACES {
				ctx.AddPatternNamespace(k, v)
			}

			return ctx, nil
		})

		core.SetNewDefaultGlobalStateFn(func(ctx *core.Context, conf core.DefaultGlobalStateConfig) (*core.GlobalState, error) {
			state := core.NewGlobalState(ctx, map[string]core.Value{
				"html":              core.ValOf(html_ns.NewHTMLNamespace()),
				"sleep":             core.WrapGoFunction(core.Sleep),
				"torstream":         core.WrapGoFunction(toRstream),
				"mkbytes":           core.WrapGoFunction(mkBytes),
				"tostr":             core.WrapGoFunction(toStr),
				"cancel_exec":       core.WrapGoFunction(cancelExec),
				"do_cpu_bound_work": core.WrapGoFunction(doCpuBoundWork),
				"add_effect":        core.WrapGoFunction(addEffect),
				"EmailAddress":      core.WrapGoFunction(makeEmailAddress),
				"statuses":          STATUS_NAMESPACE,
				"Status":            core.WrapGoFunction(makeStatus),
				"Result":            core.WrapGoFunction(NewResult),
				"ctx_data":          core.WrapGoFunction(_ctx_data),
			})

			return state, nil
		})
	}

	core.RegisterSymbolicGoFunction(toStr, func(ctx *symbolic.Context, arg symbolic.Value) symbolic.StringLike {
		return symbolic.ANY_STR_LIKE
	})

	core.RegisterSymbolicGoFunction(cancelExec, func(ctx *symbolic.Context) {})
	core.RegisterSymbolicGoFunction(doCpuBoundWork, func(ctx *symbolic.Context, _ *symbolic.Duration) {})
	core.RegisterSymbolicGoFunction(addEffect, func(ctx *symbolic.Context) *symbolic.Error { return nil })

	core.RegisterSymbolicGoFunction(mkBytes, func(ctx *symbolic.Context, i *symbolic.Int) *symbolic.ByteSlice {
		return symbolic.ANY_BYTE_SLICE
	})
	core.RegisterSymbolicGoFunction(makeEmailAddress, func(ctx *symbolic.Context, s symbolic.StringLike) *symbolic.EmailAddress {
		return symbolic.ANY_EMAIL_ADDR
	})
	core.RegisterSymbolicGoFunction(makeStatus, func(ctx *symbolic.Context, s *http_ns.StatusCode) *http_ns.Status {
		return http_ns.ANY_STATUS
	})

	core.RegisterSymbolicGoFunction(toRstream, func(ctx *symbolic.Context, v symbolic.Value) *symbolic.ReadableStream {
		return symbolic.NewReadableStream(symbolic.ANY)
	})

	core.RegisterSymbolicGoFunction(_ctx_data, func(ctx *symbolic.Context, path *symbolic.Path) symbolic.Value {
		return symbolic.ANY
	})

	if !core.IsSymbolicEquivalentOfGoFunctionRegistered(core.Sleep) {
		core.RegisterSymbolicGoFunction(core.Sleep, func(ctx *symbolic.Context, _ *symbolic.Duration) {

		})
	}
}

func registerDefaultRequestLimits(t *testing.T, limits ...core.Limit) {
	if core.AreDefaultRequestHandlingLimitsSet() {
		save := core.GetDefaultRequestHandlingLimits()
		core.UnsetDefaultRequestHandlingLimits()
		core.SetDefaultRequestHandlingLimits(limits)
		t.Cleanup(func() {
			core.UnsetDefaultRequestHandlingLimits()
			core.SetDefaultRequestHandlingLimits(save)
		})

	} else {
		core.SetDefaultRequestHandlingLimits(limits)
		t.Cleanup(func() {
			core.UnsetDefaultRequestHandlingLimits()
		})
	}
}

func registerDefaultMaxRequestHandlerLimits(t *testing.T, limits ...core.Limit) {
	if core.AreDefaultMaxRequestHandlerLimitsSet() {
		save := core.GetDefaultMaxRequestHandlerLimits()
		core.UnsetDefaultMaxRequestHandlerLimits()
		core.SetDefaultMaxRequestHandlerLimits(limits)
		t.Cleanup(func() {
			core.UnsetDefaultMaxRequestHandlerLimits()
			core.SetDefaultMaxRequestHandlerLimits(save)
		})

	} else {
		core.SetDefaultMaxRequestHandlerLimits(limits)
		t.Cleanup(func() {
			core.UnsetDefaultMaxRequestHandlerLimits()
		})
	}

}
