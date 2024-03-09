package dev

import (
	"io"
	"maps"

	"github.com/inoxlang/inox/internal/afs"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/mod"
	"github.com/inoxlang/inox/internal/project"
	"github.com/inoxlang/inox/internal/project/layout"
	"github.com/rs/zerolog"
)

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
	defer func() {
		s.lock.Lock()
		defer s.lock.Unlock()
		clear(s.runningProgramDatabases)
	}()

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

		OnPrepared: func(state *core.GlobalState) error {
			if args.Path != layout.MAIN_PROGRAM_PATH {
				return nil
			}

			state.Ctx.PutUserData(inoxconsts.DEV_CTX_DATA_ENTRY, s.devAPI)

			s.lock.Lock()
			defer s.lock.Unlock()

			clear(s.runningProgramDatabases)
			maps.Copy(s.runningProgramDatabases, state.Databases)
			return nil
		},
	})

	return preparationOk, err
}
