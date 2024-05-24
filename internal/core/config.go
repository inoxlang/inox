package core

import (
	"context"
	"errors"

	"github.com/inoxlang/inox/internal/core/slog"
	"github.com/rs/zerolog"

	"io"
)

var (
	NewDefaultGlobalState          NewDefaultGlobalStateFn //default state factory
	NewDefaultContext              NewDefaultContextFn     //default context factory
	defaultScriptLimits            []Limit
	defaultRequestHandlingLimits   []Limit
	defaultMaxRequestHandlerLimits []Limit

	ErrNoFilesystemProvided = errors.New("no filesystem provided")
)

// DefaultGlobalStateConfig is the configured passed to the default state factory.
type DefaultGlobalStateConfig struct {
	//if set MODULE_DIRPATH_GLOBAL_NAME & MODULE_FILEPATH_GLOBAL_NAME should be defined.
	AbsoluteModulePath string

	EnvPattern          *ObjectPattern
	PreinitFiles        PreinitFiles
	AllowMissingEnvVars bool

	Out io.Writer

	LogOut io.Writer //ignore if .Logger is set
	Logger zerolog.Logger

	LogLevels *slog.Levels
}

type NewDefaultGlobalStateFn func(ctx *Context, conf DefaultGlobalStateConfig) (*GlobalState, error)

// DefaultContextConfig is the configured passed to the default context factory.
type DefaultContextConfig struct {
	Permissions             []Permission
	ForbiddenPermissions    []Permission
	DoNotCheckDatabasePerms bool //used for the configuration of the created context.

	Limits              []Limit
	HostDefinitions     map[Host]Value
	ParentContext       *Context        //optional
	ParentStdLibContext context.Context //optional, should not be set if ParentContext is set

	////if nil the parent context's filesystem is used.
	//Filesystem              afs.Filesystem
	InitialWorkingDirectory Path //optional, should be passed without modification to NewContext.
}

type NewDefaultContextFn func(config DefaultContextConfig) (*Context, error)

// setter and getters

func SetNewDefaultGlobalStateFn(fn NewDefaultGlobalStateFn) {
	if NewDefaultGlobalState != nil {
		panic(errors.New("default global state fn already set"))
	}
	NewDefaultGlobalState = fn
}

func UnsetNewDefaultGlobalStateFn() {
	NewDefaultGlobalState = nil
}

func SetNewDefaultContext(fn NewDefaultContextFn) {
	if NewDefaultContext != nil {
		panic(errors.New("newDefaultContext is already set"))
	}
	NewDefaultContext = fn
}

func UnsetNewDefaultContext() {
	NewDefaultContext = nil
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
	if limits == nil {
		limits = make([]Limit, 0)
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
	if limits == nil {
		limits = make([]Limit, 0)
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
