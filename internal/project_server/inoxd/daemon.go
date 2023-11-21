package inoxd

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/inoxlang/inox/internal/utils"
)

const DAEMON_SUBCMD = "daemon"

type DaemonConfig struct {
	InoxCloud      bool                          `json:"inoxCloud"`
	Server         IndividualProjectServerConfig `json:"serverConfig"`
	InoxBinaryPath string
}

type IndividualProjectServerConfig struct {
	MaxWebSocketPerIp      int  `json:"maxWebsocketPerIp"`
	IgnoreInstalledBrowser bool `json:"ignoreInstalledBrowser,omitempty"`
	ProjectsDir            bool `json:"projectsDir,omitempty"` //if not set, defaults to filepath.Join(config.USER_HOME, "inox-projects")
}

func Inoxd(config DaemonConfig, errW, outW io.Writer) {
	serverConfig := config.Server

	launchProjectServer(projectServerCmdParams{
		config:         serverConfig,
		inoxBinaryPath: config.InoxBinaryPath,
		stderr:         errW,
		stdout:         outW,
	})
}

type projectServerCmdParams struct {
	config         IndividualProjectServerConfig
	inoxBinaryPath string
	stderr, stdout io.Writer
}

func launchProjectServer(args projectServerCmdParams) {
	projectServerConfig := "-config=" + string(utils.Must(json.Marshal(args.config)))

	cmd := exec.Command(args.inoxBinaryPath, "project-server", projectServerConfig)
	cmd.Stderr = args.stderr
	cmd.Stdout = args.stdout

	if err := cmd.Run(); err != nil {
		fmt.Fprintln(args.stderr, err.Error())
	}
}
