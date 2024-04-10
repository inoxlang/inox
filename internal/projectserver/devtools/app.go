package devtools

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/slog"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/mod"
	"github.com/inoxlang/inox/internal/project/layout"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	EARLY_DEV_TOOLS_ERROR_PERIOD = 2 * time.Second
)

var (
	ErrInexistingOrInvalidDevToolsEntryPoint = errors.New("entry point for the dev tools server does not exist or is invalid")
)

// StartWebApp starts an Inox web application that provides development tools.
// The application servers listens on a development port on localhost, therefore it is virtual and do not directly bind.
func (inst *Instance) StartWebApp() error {

	_, ok := http_ns.GetDevServer(inst.toolsServerPort)
	if !ok {
		return fmt.Errorf("failed to start dev tools server: dev server on port %s is not listening", inst.toolsServerPort)
	}

	entryPoint := layout.DEV_TOOLS_SERVER_ENTRY_POINT
	stat, err := inst.developerWorkingFS.Stat(entryPoint)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return ErrInexistingOrInvalidDevToolsEntryPoint
		}
		return err
	}
	if stat.IsDir() {
		return ErrInexistingOrInvalidDevToolsEntryPoint
	}

	earlyErrChan := make(chan error, 1)
	go func() {
		defer utils.Recover()

		_, _, _, _, err = mod.RunLocalModule(mod.RunLocalModuleArgs{
			Fpath:                    entryPoint,
			SingleFileParsingTimeout: SINGLE_FILE_PARSING_TIMEOUT,

			ParsingCompilationContext:      inst.context,
			ParentContext:                  inst.context,
			ParentContextRequired:          true,
			PreinitFilesystem:              inst.developerWorkingFS,
			AllowMissingEnvVars:            false,
			IgnoreHighRiskScore:            true,
			FullAccessToDatabases:          true,
			Project:                        inst.project,
			MemberAuthToken:                inst.memberAuthToken,
			ListeningPort:                  inoxconsts.Uint16DevPort(inst.toolsServerPort),
			ForceLocalhostListeningAddress: true,

			Out:    io.Discard,
			Logger: zerolog.Nop(),
			LogLevels: slog.NewLevels(slog.LevelsInitialization{
				DefaultLevel:            zerolog.WarnLevel,
				EnableInternalDebugLogs: false,
			}),

			OnPrepared: func(state *core.GlobalState) error {
				//Give access to the dev API to the dev tools app.
				state.Ctx.PutUserData(inoxconsts.DEV_CTX_DATA_ENTRY, inst.api)
				return nil
			},
		})

		earlyErrChan <- err
	}()

	select {
	case err := <-earlyErrChan:
		return err
	case <-time.After(EARLY_DEV_TOOLS_ERROR_PERIOD):
		return nil
	}
}
