package inoxd

import (
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/containerd/cgroups/v3"
	"github.com/inoxlang/inox/internal/inoxd/cloud/cloudproxy"
	"github.com/inoxlang/inox/internal/inoxd/consts"
	inoxdcrypto "github.com/inoxlang/inox/internal/inoxd/crypto"
	"github.com/inoxlang/inox/internal/project_server"
	"github.com/inoxlang/inox/internal/utils"
)

const DAEMON_SUBCMD = "daemon"

type DaemonConfig struct {
	InoxCloud        bool                                  `json:"inoxCloud,omitempty"`
	Server           project_server.IndividualServerConfig `json:"projectServerConfig"`
	ExposeWebServers bool                                  `json:"exposeWebServers,omitempty"`
	TunnelProvider   string                                `json:"tunnelProvider,omitempty"`
	InoxBinaryPath   string                                `json:"-"`
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
	proxyConfig := cloudproxy.CloudProxyConfig{
		CloudDataDir: consts.CLOUD_DATA_DIR,
		Port:         project_server.DEFAULT_PROJECT_SERVER_PORT_INT,
	}

	go func() {
		const MAX_TRY_COUNT = 3
		tryCount := 0
		var lastLaunchTime time.Time

		for {
			if tryCount >= MAX_TRY_COUNT {
				fmt.Fprintf(errW, "cloud proxy process exited unexpectedly %d or more times in a short timeframe; wait 5 minutes\n", MAX_TRY_COUNT)
				time.Sleep(5 * time.Minute)
				tryCount = 0
			}

			tryCount++
			lastLaunchTime = time.Now()

			err := launchCloudProxy(cloudProxyCmdParams{
				inoxBinaryPath: config.InoxBinaryPath,
				stderr:         errW,
				stdout:         outW,
				config:         proxyConfig,
			})

			fmt.Fprintf(errW, "cloud proxy process returned: %s\n", err.Error())

			if time.Since(lastLaunchTime) < 10*time.Second {
				tryCount++
			} else {
				tryCount = 1
			}
		}
	}()

	for {
		time.Sleep(time.Minute)
	}
}

type cloudProxyCmdParams struct {
	inoxBinaryPath string
	stderr, stdout io.Writer
	config         cloudproxy.CloudProxyConfig
}

func launchCloudProxy(args cloudProxyCmdParams) error {
	config := "-config=" + string(utils.Must(json.Marshal(args.config)))

	cmd := exec.Command(args.inoxBinaryPath, cloudproxy.CLOUD_PROXY_SUBCMD_NAME, config)
	cmd.Stderr = args.stderr
	cmd.Stdout = args.stdout

	fmt.Fprintln(args.stdout, "create a new inox process (cloud proxy)")
	return cmd.Run()
}
