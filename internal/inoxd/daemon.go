package inoxd

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime/debug"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/containerd/cgroups/v3"
	"github.com/gorilla/websocket"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/inoxd/cloud/cloudproxy"
	"github.com/inoxlang/inox/internal/inoxd/cloud/cloudproxy/inoxdconn"
	"github.com/inoxlang/inox/internal/inoxd/consts"
	inoxdcrypto "github.com/inoxlang/inox/internal/inoxd/crypto"
	"github.com/inoxlang/inox/internal/project_server"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/inoxlang/inox/internal/utils/processutils"
	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"
)

const (
	DAEMON_SUBCMD = "daemon"
	INOXD_LOG_SRC = "/inoxd"
)

type DaemonConfig struct {
	InoxCloud        bool                                  `json:"inoxCloud,omitempty"`
	Server           project_server.IndividualServerConfig `json:"projectServerConfig"`
	ExposeWebServers bool                                  `json:"exposeWebServers,omitempty"`
	TunnelProvider   string                                `json:"tunnelProvider,omitempty"`
	InoxBinaryPath   string                                `json:"-"`
}

type InoxdArgs struct {
	Config DaemonConfig
	Logger zerolog.Logger
	GoCtx  context.Context

	DoNotUseCgroups     bool
	TestOnlyProxyConfig *cloudproxy.CloudProxyConfig
}

func Inoxd(args InoxdArgs) {
	config := args.Config
	goCtx := args.GoCtx
	logger := args.Logger.With().Str(core.SOURCE_LOG_FIELD_NAME, INOXD_LOG_SRC).Logger()

	serverConfig := config.Server
	mode, modeName := getCgroupMode()
	useCgroups := !args.DoNotUseCgroups

	logger.Info().Msgf("current cgroup mode is %q\n", modeName)

	masterKeySet, err := inoxdcrypto.LoadInoxdMasterKeysetFromEnv()
	if err != nil {
		logger.Error().Err(err).Msgf("failed to load inox master keyset")
		return
	}

	logger.Info().Msgf("master keyset successfully loaded, it contains %d key(s)", len(masterKeySet.KeysetInfo().KeyInfo))

	daemon := &Daemon{
		goCtx:  goCtx,
		logger: logger,
	}

	if !config.InoxCloud {
		//TODO: restart the process each time it crashes.
		project_server.ExecuteProjectServerCmd(project_server.ProjectServerCmdParams{
			GoCtx:          goCtx,
			Config:         serverConfig,
			InoxBinaryPath: config.InoxBinaryPath,
			Logger:         logger,
		})
		return
	}

	// ========== CLOUD MODE ==========

	serverConfig.BehindCloudProxy = true

	if useCgroups {
		if mode != cgroups.Unified {
			logger.Error().Msg("abort execution because current cgroup mode is not 'unified'")
			return
		}

		if !createInoxCgroup(logger) {
			return
		}
	}

	//get proxy configuration

	proxyConfig := cloudproxy.CloudProxyConfig{
		CloudDataDir: consts.CLOUD_DATA_DIR,
		Port:         project_server.DEFAULT_PROJECT_SERVER_PORT_INT,
	}

	if args.TestOnlyProxyConfig != nil {
		proxyConfig = *args.TestOnlyProxyConfig
	}

	daemon.proxyConfig = &proxyConfig

	//launch proxy

	proxyExitChan := make(chan struct{}, 1)
	var restartPaused atomic.Bool

	go processutils.AutoRestart(processutils.AutoRestartArgs{
		GoCtx: goCtx,
		MakeCommand: func() *exec.Cmd {
			return makeCloudProxyCommand(cloudProxyCmdParams{
				goCtx:          goCtx,
				inoxBinaryPath: config.InoxBinaryPath,
				config:         proxyConfig,
				logger:         logger,
			})
		},
		Logger:                      logger,
		ProcessNameInLogs:           "cloud proxy",
		MaxTryCount:                 3,
		PostStartBurstPause:         &restartPaused,
		PostStartBurstPauseDuration: 5 * time.Minute,
		ExitEventChan:               proxyExitChan,
	})

	//create a websocket connection with the proxy.

	inoxdconn.RegisterTypesInGob()
	daemon.cloudProxyConnLoop(proxyExitChan, &restartPaused)
}

type cloudProxyCmdParams struct {
	goCtx          context.Context
	inoxBinaryPath string
	config         cloudproxy.CloudProxyConfig
	logger         zerolog.Logger
}

func makeCloudProxyCommand(args cloudProxyCmdParams) *exec.Cmd {
	config := "-config=" + string(utils.Must(json.Marshal(args.config)))

	cmd := exec.CommandContext(args.goCtx, args.inoxBinaryPath, cloudproxy.CLOUD_PROXY_SUBCMD_NAME, config)

	cmd.Stderr = utils.FnWriter{
		WriteFn: func(p []byte) (n int, err error) {
			args.logger.Error().Msg(string(p))
			return len(p), nil
		},
	}
	cmd.Stdout = utils.FnWriter{
		WriteFn: args.logger.Write,
	}
	return cmd
}

type Daemon struct {
	goCtx  context.Context
	logger zerolog.Logger

	//only set in cloud mode
	proxyConfig *cloudproxy.CloudProxyConfig
}

func (d *Daemon) cloudProxyConnLoop(proxyProcessedFinished <-chan struct{}, restartPaused *atomic.Bool) {
	for !utils.IsContextDone(d.goCtx) {
		func() {
			defer func() {
				e := recover()
				if e != nil {
					err := utils.ConvertPanicValueToError(e)
					err = fmt.Errorf("%w: %s", err, debug.Stack())
					d.logger.Err(err).Send()
				}
			}()

			select {
			case <-proxyProcessedFinished:
				//wait for the proxy to restart or for the creation loop to tell if a long wait is needed.
				time.Sleep(100 * time.Millisecond)

				for restartPaused.Load() {
					time.Sleep(time.Second)
				}
			default:
			}

			//create websocket connection to the proxy.
			dialer := *websocket.DefaultDialer
			dialer.Proxy = nil
			dialer.HandshakeTimeout = 10 * time.Millisecond
			dialer.TLSClientConfig = &tls.Config{
				InsecureSkipVerify: true,
			}
			socket, _, err := dialer.Dial("wss://localhost:"+strconv.Itoa(d.proxyConfig.Port)+consts.PROXY__INOXD_WEBSOCKET_ENDPOINT, nil)
			if err != nil {
				d.logger.Err(err).Send()
				return
			}
			defer socket.Close()

			d.handleSingleConnection(socket)
		}()
	}
}

func (d *Daemon) handleSingleConnection(socket *websocket.Conn) {

	//send hello message.
	helloMsg := inoxdconn.Message{
		ULID:  ulid.Make(),
		Inner: inoxdconn.Hello{},
	}

	err := socket.WriteMessage(websocket.BinaryMessage, inoxdconn.MustEncodeMessage(helloMsg))
	if err != nil {
		d.logger.Err(err).Send()
		return
	}

	//message handling loop.
	for !utils.IsContextDone(d.goCtx) {

		msgType, payload, err := socket.ReadMessage()

		if err != nil {
			d.logger.Err(err).Send()
			return
		}

		if msgType != websocket.BinaryMessage {
			continue
		}

		var msg inoxdconn.Message
		err = inoxdconn.DecodeMessage(payload, &msg)
		if err != nil {
			d.logger.Err(err).Send()
			return
		}

		switch m := msg.Inner.(type) {
		case inoxdconn.Ack:
			d.logger.Debug().Msg("ack received on connection to cloud-proxy for message " + m.AcknowledgedMessage.String())
		}
		//TODO
	}
}
