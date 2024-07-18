package globals

import "github.com/inoxlang/inox/internal/core"

// NewDefaultState creates a new Context with the default patterns.
func NewDefaultContext(config core.DefaultContextConfig) (*core.Context, error) {

	ctxConfig := core.ContextConfig{
		Permissions:             config.Permissions,
		ForbiddenPermissions:    config.ForbiddenPermissions,
		DoNotCheckDatabasePerms: config.DoNotCheckDatabasePerms,

		Limits:              config.Limits,
		HostDefinitions:     config.HostDefinitions,
		ParentContext:       config.ParentContext,
		ParentStdLibContext: config.ParentStdLibContext,
	}

	if ctxConfig.ParentContext != nil {
		if err, _ := ctxConfig.Check(); err != nil {
			return nil, err
		}
	}

	ctx := core.NewContext(ctxConfig)

	for k, v := range core.DEFAULT_NAMED_PATTERNS {
		ctx.AddNamedPattern(k, v)
	}

	for k, v := range core.DEFAULT_PATTERN_NAMESPACES {
		ctx.AddPatternNamespace(k, v)
	}

	return ctx, nil
}
