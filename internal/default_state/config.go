package default_state

import (
	"errors"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"

	"io"
)

const (
	PREINIT_DATA_GLOBAL_NAME = "preinit-data"
	DATABASES_GLOBAL_NAME    = "dbs"
)

var (
	NewDefaultGlobalState    NewDefaultGlobalStateFn
	NewDefaultContext        NewDefaultContextFn
	defaultScriptLimitations []core.Limitation
)

type DefaultGlobalStateConfig struct {
	EnvPattern          *core.ObjectPattern
	PreinitFiles        core.PreinitFiles
	Databases           map[string]*core.DatabaseIL
	AllowMissingEnvVars bool
	Out                 io.Writer
	LogOut              io.Writer
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

func SetDefaultScriptLimitations(limitations []core.Limitation) {
	if defaultScriptLimitations != nil {
		panic(errors.New("default script limitations already set"))
	}
	defaultScriptLimitations = limitations
}

func GetDefaultScriptLimitations() []core.Limitation {
	if defaultScriptLimitations == nil {
		panic(errors.New("default script limitations are not set"))
	}
	return defaultScriptLimitations
}

type DefaultContextConfig struct {
	Permissions          []core.Permission
	ForbiddenPermissions []core.Permission
	Limitations          []core.Limitation
	HostResolutions      map[core.Host]core.Value
	ParentContext        *core.Context  //optional
	Filesystem           afs.Filesystem //if nil the OS filesystem is used
}

type NewDefaultContextFn func(config DefaultContextConfig) (*core.Context, error)
