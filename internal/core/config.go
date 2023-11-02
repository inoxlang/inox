package core

import (
	"context"
	"errors"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/rs/zerolog"

	"io"
)

var (
	NewDefaultGlobalState          NewDefaultGlobalStateFn
	NewDefaultContext              NewDefaultContextFn
	defaultScriptLimits            []Limit
	defaultRequestHandlingLimits   []Limit
	defaultMaxRequestHandlerLimits []Limit
)

type DefaultGlobalStateConfig struct {
	//if set MODULE_DIRPATH_GLOBAL_NAME & MODULE_FILEPATH_GLOBAL_NAME should be defined.
	AbsoluteModulePath string

	EnvPattern          *ObjectPattern
	PreinitFiles        PreinitFiles
	AllowMissingEnvVars bool

	Out io.Writer

	LogOut io.Writer //ignore if .Logger is set
	Logger zerolog.Logger
}

type NewDefaultGlobalStateFn func(ctx *Context, conf DefaultGlobalStateConfig) (*GlobalState, error)

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

func SetDefaultScriptLimits(limits []Limit) {
	if defaultScriptLimits != nil {
		panic(errors.New("default script limits already set"))
	}
	defaultScriptLimits = limits
}

func GetDefaultScriptLimits() []Limit {
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

func SetDefaultRequestHandlingLimits(limits []Limit) {
	if defaultRequestHandlingLimits != nil {
		panic(errors.New("default request handling limits already set"))
	}
	defaultRequestHandlingLimits = limits
}

func GetDefaultRequestHandlingLimits() []Limit {
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

func SetDefaultMaxRequestHandlerLimits(limits []Limit) {
	if defaultMaxRequestHandlerLimits != nil {
		panic(errors.New("default max request handler limits already set"))
	}
	defaultMaxRequestHandlerLimits = limits
}

func GetDefaultMaxRequestHandlerLimits() []Limit {
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
	Permissions          []Permission
	ForbiddenPermissions []Permission
	Limits               []Limit
	HostResolutions      map[Host]Value
	OwnedDatabases       []DatabaseConfig
	ParentContext        *Context        //optional
	ParentStdLibContext  context.Context //optional, should not be set if ParentContext is set
	Filesystem           afs.Filesystem  //if nil the OS filesystem is used
}

type NewDefaultContextFn func(config DefaultContextConfig) (*Context, error)
