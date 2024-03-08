package dev

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/mod"
	"github.com/inoxlang/inox/internal/project"
	"github.com/rs/zerolog"
)

const (
	SINGLE_FILE_PARSING_TIMEOUT = 100 * time.Millisecond
)

var (
	ErrDevSessionAlreadyRunningProgram = errors.New("development session is already running a program")
)

type Session struct {
	lock sync.Mutex

	isRunningAProgram atomic.Bool
}

func NewDevSession() *Session {
	return &Session{}
}

type RunProgramParams struct {
	Path            string
	ParentContext   *core.Context //also used as the parsing context
	Project         *project.Project
	MemberAuthToken string

	PreinitFilesystem afs.Filesystem
	Debugger          *core.Debugger

	ProgramOut io.Writer
	Logger     zerolog.Logger
	LogLevels  *core.LogLevels

	ProgramPreparedOrFailedToChan chan error
}

func (s *Session) RunProgram(args RunProgramParams) (preparationOk bool, _ error) {

	if !s.isRunningAProgram.CompareAndSwap(false, true) {
		return false, ErrDevSessionAlreadyRunningProgram
	}

	defer s.isRunningAProgram.Store(false)

	_, _, _, preparationOk, err := mod.RunLocalModule(mod.RunLocalModuleArgs{
		Fpath:                    args.Path,
		SingleFileParsingTimeout: SINGLE_FILE_PARSING_TIMEOUT,

		ParsingCompilationContext: args.ParentContext,
		ParentContext:             args.ParentContext,
		ParentContextRequired:     true,
		PreinitFilesystem:         args.PreinitFilesystem,
		AllowMissingEnvVars:       false,
		IgnoreHighRiskScore:       true,
		FullAccessToDatabases:     true,
		Project:                   args.Project,
		MemberAuthToken:           args.MemberAuthToken,

		Out:       args.ProgramOut,
		Logger:    args.Logger,
		LogLevels: args.LogLevels,

		Debugger:     args.Debugger,
		PreparedChan: args.ProgramPreparedOrFailedToChan,
	})

	return preparationOk, err
}
