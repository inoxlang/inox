package inoxd

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"

	"github.com/containerd/cgroups/v3"
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

	mode := cgroups.Mode()
	modeName := "unavailable"
	switch mode {
	case cgroups.Legacy:
		modeName = "legacy"
	case cgroups.Hybrid:
		modeName = "hybrid"
	case cgroups.Unified:
		modeName = "unified"
	}

	fmt.Fprintf(outW, "current cgroup mode is %q\n", modeName)

	if config.InoxCloud {
		if mode != cgroups.Unified {
			fmt.Fprintf(errW, "abort execution because current cgroup mode is not 'unified'\n")
			return
		}

		if !createInoxCgroup(outW, errW) {
			return
		}
	}

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
