package project_server

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/inoxlang/inox/internal/utils"
)

type ProjectServerCmdParams struct {
	Config         IndividualServerConfig
	InoxBinaryPath string
	Stderr, Stdout io.Writer
}

func ExecuteProjectServerCmd(args ProjectServerCmdParams) {
	projectServerConfig := "-config=" + string(utils.Must(json.Marshal(args.Config)))

	cmd := exec.Command(args.InoxBinaryPath, "project-server", projectServerConfig)
	cmd.Stderr = args.Stderr
	cmd.Stdout = args.Stdout

	fmt.Fprintln(args.Stdout, "create a new inox process (project server)")

	if err := cmd.Run(); err != nil {
		fmt.Fprintln(args.Stderr, err.Error())
	}
}
