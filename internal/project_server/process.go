package project_server

import (
	"context"
	"encoding/json"
	"os/exec"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

type ProjectServerCmdParams struct {
	GoCtx          context.Context
	Config         IndividualServerConfig
	InoxBinaryPath string
	Logger         zerolog.Logger
}

func ExecuteProjectServerCmd(args ProjectServerCmdParams) {
	projectServerConfig := "-config=" + string(utils.Must(json.Marshal(args.Config)))

	cmd := exec.CommandContext(args.GoCtx, args.InoxBinaryPath, "project-server", projectServerConfig)

	cmd.Stderr = utils.FnWriter{
		WriteFn: func(p []byte) (n int, err error) {
			args.Logger.Error().Msg(string(p))
			return len(p), nil
		},
	}
	cmd.Stdout = utils.FnWriter{
		WriteFn: args.Logger.Write,
	}

	args.Logger.Info().Msg("create a new inox process (project server)")

	if err := cmd.Run(); err != nil {
		args.Logger.Error().Err(err).Send()
	}
}
