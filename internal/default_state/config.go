package default_state

import (
	"errors"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/project"

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
	NewDefaultGlobalState    NewDefaultGlobalStateFn
	NewDefaultContext        NewDefaultContextFn
	defaultScriptLimitations []core.Limitation
)

type DefaultGlobalStateConfig struct {
	//if set MODULE_DIRPATH_GLOBAL_NAME & MODULE_FILEPATH_GLOBAL_NAME should be defined.
	AbsoluteModulePath string

	EnvPattern          *core.ObjectPattern
	PreinitFiles        core.PreinitFiles
	Databases           map[string]*core.DatabaseIL
	AllowMissingEnvVars bool
	Project             *project.Project

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

func IsDefaultScriptLimitationsSet() bool {
	return defaultScriptLimitations != nil
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
