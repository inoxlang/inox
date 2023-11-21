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
	InoxCloud      bool                   `json:"inoxCloud"`
	Server         IndividualServerConfig `json:"serverConfig"`
	InoxBinaryPath string
}

type IndividualServerConfig struct {
	MaxWebSocketPerIp      int  `json:"maxWebsocketPerIp"`
	IgnoreInstalledBrowser bool `json:"ignoreInstalledBrowser,omitempty"`
}

func Inoxd(config DaemonConfig, errW, outW io.Writer) {
	serverConfig := config.Server

	projectServerConfig := "-config=" + string(utils.Must(json.Marshal(serverConfig)))

	cmd := exec.Command(config.InoxBinaryPath, "project-server", projectServerConfig)
	cmd.Stderr = errW
	cmd.Stdout = outW

	if err := cmd.Run(); err != nil {
		fmt.Fprintln(errW, err.Error())
	}
}
