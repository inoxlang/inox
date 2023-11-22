package inoxd

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/containerd/cgroups/v3"
	"github.com/inoxlang/inox/internal/inoxd/cloudproxy"
	"github.com/inoxlang/inox/internal/utils"
)

const DAEMON_SUBCMD = "daemon"

type DaemonConfig struct {
	InoxCloud      bool                          `json:"inoxCloud"`
	CloudProxy     *CloudProxyConfig             `json:"cloudProxy,omitempty"` //ignored if inoxCloud is false
	Server         IndividualProjectServerConfig `json:"serverConfig"`
	InoxBinaryPath string
}

type IndividualProjectServerConfig struct {
	MaxWebSocketPerIp      int    `json:"maxWebsocketPerIp"`
	IgnoreInstalledBrowser bool   `json:"ignoreInstalledBrowser,omitempty"`
	ProjectsDir            string `json:"projectsDir,omitempty"` //if not set, defaults to filepath.Join(config.USER_HOME, "inox-projects")
	BehindCloudProxy       bool   `json:"behindCloudProxy,omitempty"`
	Port                   int    `json:"port,omitempty"`
}

type CloudProxyConfig struct {
	MaxWebSocketPerIp int `json:"maxWebsocketPerIp"`
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
		serverConfig.BehindCloudProxy = true

		if mode != cgroups.Unified {
			fmt.Fprintf(errW, "abort execution because current cgroup mode is not 'unified'\n")
			return
		}

		if !createInoxCgroup(outW, errW) {
			return
		}

		//launch proxy
		go func() {
			const MAX_TRY_COUNT = 3
			tryCount := 0
			var lastLaunchTime time.Time

			for {
				if tryCount >= MAX_TRY_COUNT {
					fmt.Fprintf(errW, "cloud proxy process exited unexpectedly %d or more times in a short timeframe; wait 5 minutes", MAX_TRY_COUNT)
					time.Sleep(5 * time.Minute)
					tryCount = 0
				}

				tryCount++
				lastLaunchTime = time.Now()

				err := launchCloudProxy(CloudProxyCmdParams{
					inoxBinaryPath: config.InoxBinaryPath,
					stderr:         errW,
					stdout:         outW,
				})

				fmt.Fprintf(errW, "cloud proxy process returned: %s\n", err.Error())

				if time.Since(lastLaunchTime) < 10*time.Second {
					tryCount++
				} else {
					tryCount = 1
				}
			}
		}()
	}

	// launchProjectServer(projectServerCmdParams{
	// 	config:         serverConfig,
	// 	inoxBinaryPath: config.InoxBinaryPath,
	// 	stderr:         errW,
	// 	stdout:         outW,
	// })

	time.Sleep(time.Hour)
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

	fmt.Fprintln(args.stdout, "create a new inox process (project server)")

	if err := cmd.Run(); err != nil {
		fmt.Fprintln(args.stderr, err.Error())
	}
}

type CloudProxyCmdParams struct {
	inoxBinaryPath string
	stderr, stdout io.Writer
}

func launchCloudProxy(args CloudProxyCmdParams) error {
	cmd := exec.Command(args.inoxBinaryPath, cloudproxy.CLOUD_PROXY_SUBCMD_NAME)
	cmd.Stderr = args.stderr
	cmd.Stdout = args.stdout

	fmt.Fprintln(args.stdout, "create a new inox process (cloud proxy)")
	return cmd.Run()
}
