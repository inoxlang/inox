package devtools

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5/util"
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

var (
	//go:embed tools/**
	DEFAULT_TOOLS_DIR embed.FS
)

// StartWebApp starts an Inox web application that provides development tools.
// The application servers listens on a development port on localhost, therefore it is virtual and do not directly bind.
func (inst *Instance) StartWebApp() error {

	const DEFAULT_DIR_FPERMS = 0600
	const DEFAULT_FILE_FPERMS = 0600

	_, ok := http_ns.GetDevServer(inst.toolsServerPort)
	if !ok {
		return fmt.Errorf("failed to start dev tools server: dev server on port %s is not listening", inst.toolsServerPort)
	}

	_, err := inst.developerWorkingFS.Stat(layout.DEV_TOOLS_DIR_PATH)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			//If the directory does not exist, create it and includes ./tools.
			err = inst.developerWorkingFS.MkdirAll(layout.DEV_TOOLS_DIR_PATH, DEFAULT_DIR_FPERMS)
			if err != nil {
				return err
			}

			err = fs.WalkDir(DEFAULT_TOOLS_DIR, "tools", func(path string, d fs.DirEntry, err error) error {
				actualPath := filepath.Join(layout.DEV_TOOLS_DIR_PATH, strings.TrimPrefix(path, "tools"))

				if d.IsDir() {
					return inst.developerWorkingFS.MkdirAll(actualPath, DEFAULT_DIR_FPERMS)
				} else {
					content := utils.Must(DEFAULT_TOOLS_DIR.ReadFile(path))
					return util.WriteFile(inst.developerWorkingFS, actualPath, content, DEFAULT_FILE_FPERMS)
				}
			})
		}
		if err != nil {
			return err
		}
	}

	//Check that the entry point file is present.

	entryPoint := layout.DEV_TOOLS_SERVER_ENTRY_POINT
	stat, err := inst.developerWorkingFS.Stat(entryPoint)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			//We do not create the entrypoint because custom dev tools may have been installed.
			return ErrInexistingOrInvalidDevToolsEntryPoint
		}
		return err
	}
	if stat.IsDir() {
		return ErrInexistingOrInvalidDevToolsEntryPoint
	}

	//Launch the program.

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
