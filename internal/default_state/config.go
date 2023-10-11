package default_state

import (
	"context"
	"errors"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"

	"io"
)

const (
	PREINIT_DATA_GLOBAL_NAME    = "preinit-data"
	DATABASES_GLOBAL_NAME       = "dbs"
	PROJECT_SECRETS_GLOBAL_NAME = "project-secrets"
	MODULE_DIRPATH_GLOBAL_NAME  = "__mod-dir"
	MODULE_FILEPATH_GLOBAL_NAME = "__mod-file"
)

var (
	NewDefaultGlobalState          NewDefaultGlobalStateFn
	NewDefaultContext              NewDefaultContextFn
	defaultScriptLimits            []core.Limit
	defaultRequestHandlingLimits   []core.Limit
	defaultMaxRequestHandlerLimits []core.Limit
)

type DefaultGlobalStateConfig struct {
	//if set MODULE_DIRPATH_GLOBAL_NAME & MODULE_FILEPATH_GLOBAL_NAME should be defined.
	AbsoluteModulePath string

	EnvPattern          *core.ObjectPattern
	PreinitFiles        core.PreinitFiles
	AllowMissingEnvVars bool

	Out    io.Writer
	LogOut io.Writer
}

type NewDefaultGlobalStateFn func(ctx *core.Context, conf DefaultGlobalStateConfig) (*core.GlobalState, error)

func SetNewDefaultGlobalStateFn(fn NewDefaultGlobalStateFn) {
	if NewDefaultGlobalState != nil {
		panic(errors.New("default global state fn already set"))
	}
	NewDefaultGlobalState = fn
}

func SetNewDefaultContext(fn NewDefaultContextFn) {
	if NewDefaultContext != nil {
		panic(errors.New("newDefaultContext is already set"))
	}
	NewDefaultContext = fn
}

func SetDefaultScriptLimits(limits []core.Limit) {
	if defaultScriptLimits != nil {
		panic(errors.New("default script limits already set"))
	}
	defaultScriptLimits = limits
}

func GetDefaultScriptLimits() []core.Limit {
	if defaultScriptLimits == nil {
		panic(errors.New("default script limits are not set"))
	}
	return defaultScriptLimits
}

func AreDefaultScriptLimitsSet() bool {
	return defaultScriptLimits != nil
}

func UnsetDefaultScriptLimits() {
	defaultScriptLimits = nil
}

func SetDefaultRequestHandlingLimits(limits []core.Limit) {
	if defaultRequestHandlingLimits != nil {
		panic(errors.New("default request handling limits already set"))
	}
	defaultRequestHandlingLimits = limits
}

func GetDefaultRequestHandlingLimits() []core.Limit {
	if defaultRequestHandlingLimits == nil {
		panic(errors.New("default request handling limits are not set"))
	}
	return defaultRequestHandlingLimits
}

func AreDefaultRequestHandlingLimitsSet() bool {
	return defaultRequestHandlingLimits != nil
}

func UnsetDefaultRequestHandlingLimits() {
	defaultRequestHandlingLimits = nil
}

func SetDefaultMaxRequestHandlerLimits(limits []core.Limit) {
	if defaultMaxRequestHandlerLimits != nil {
		panic(errors.New("default max request handler limits already set"))
	}
	defaultMaxRequestHandlerLimits = limits
}

func GetDefaultMaxRequestHandlerLimits() []core.Limit {
	if defaultMaxRequestHandlerLimits == nil {
		panic(errors.New("default max request handler limits are not set"))
	}
	return defaultMaxRequestHandlerLimits
}

func AreDefaultMaxRequestHandlerLimitsSet() bool {
	return defaultMaxRequestHandlerLimits != nil
}

func UnsetDefaultMaxRequestHandlerLimits() {
	defaultMaxRequestHandlerLimits = nil
}

type DefaultContextConfig struct {
	Permissions          []core.Permission
	ForbiddenPermissions []core.Permission
	Limits               []core.Limit
	HostResolutions      map[core.Host]core.Value
	OwnedDatabases       []core.DatabaseConfig
	ParentContext        *core.Context   //optional
	ParentStdLibContext  context.Context //optional, should not be set if ParentContext is set
	Filesystem           afs.Filesystem  //if nil the OS filesystem is used
}

type NewDefaultContextFn func(config DefaultContextConfig) (*core.Context, error)
