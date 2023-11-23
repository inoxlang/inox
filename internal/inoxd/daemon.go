package inoxd

import (
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/containerd/cgroups/v3"
	"github.com/inoxlang/inox/internal/inoxd/cloud/cloudproxy"
	inoxdcrypto "github.com/inoxlang/inox/internal/inoxd/crypto"
	"github.com/inoxlang/inox/internal/project_server"
)

const DAEMON_SUBCMD = "daemon"

type DaemonConfig struct {
	InoxCloud      bool                                  `json:"inoxCloud"`
	CloudProxy     *CloudProxyConfig                     `json:"cloudProxy,omitempty"` //ignored if inoxCloud is false
	Server         project_server.IndividualServerConfig `json:"serverConfig"`
	InoxBinaryPath string
}

type CloudProxyConfig struct {
	MaxWebSocketPerIp int `json:"maxWebsocketPerIp"`
}

func Inoxd(config DaemonConfig, errW, outW io.Writer) {
	serverConfig := config.Server

	mode, modeName := getCgroupMode()
	fmt.Fprintf(outW, "current cgroup mode is %q\n", modeName)

	masterKeySet, err := inoxdcrypto.LoadInoxdMasterKeysetFromEnv()
	if err != nil {
		fmt.Fprintf(errW, "failed to load inox master keyset: %s", err.Error())
		return
	}

	fmt.Fprintf(outW, "master keyset successfully loaded, it contains %d key(s)\n", len(masterKeySet.KeysetInfo().KeyInfo))

	if !config.InoxCloud {
		project_server.ExecuteProjectServerCmd(project_server.ProjectServerCmdParams{
			Config:         serverConfig,
			InoxBinaryPath: config.InoxBinaryPath,
			Stderr:         errW,
			Stdout:         outW,
		})
		return
	}

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
