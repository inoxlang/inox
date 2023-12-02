package inoxprocess

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	nettypes "github.com/inoxlang/inox/internal/net_types"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"
)

const (
	CONTROL_SERVER_LOG_SRC_KEY = "/control-server"
	PROCESS_TOKEN_HEADER       = "Process-Token"
	PROCESS_TOKEN_BYTE_LENGTH  = 32

	CONTROLLED_SUBCMD = "controlled"

	//maximum time the control server will wait for the controlled process to connect.
	MAX_CONTROLLED_PROCESS_CONNECTION_WAITING_TIME = time.Second
)

var (
	PROCESS_TOKEN_ENCODED_BYTE_LENGTH = hex.EncodedLen(PROCESS_TOKEN_BYTE_LENGTH)
	ErrProcessDidNotConnect           = errors.New("controlled process did not connect to the control server")
	ErrProcessAlreadyConnected        = errors.New("controlled process is already connected to the control server")
)

// A ControlServer creates and controls Inox processes, each created process connects to the control server with a WebSocket.
type ControlServer struct {
	ctx             *core.Context
	config          ControlServerConfig
	port            string
	websocketServer *net_ns.WebsocketServer

	httpServer *http.Server
	logger     zerolog.Logger

	controlledProcesses     map[ControlledProcessToken]*ControlledProcess
	controlledProcessesLock sync.Mutex
}

type ControlServerConfig struct {
	Port           uint16
	InoxBinaryPath string
}

func (c ControlServerConfig) Check() error {
	if !strings.HasPrefix(c.InoxBinaryPath, "/") {
		return errors.New(".InoxBinaryPath should be an absolute path")
	}

	return nil
}

// NewControlServer creates a ControlServer, the Start method should be called to start it.
func NewControlServer(ctx *core.Context, config ControlServerConfig) (*ControlServer, error) {
	if err := config.Check(); err != nil {
		return nil, err
	}

	s := &ControlServer{
		config:              config,
		ctx:                 ctx,
		port:                strconv.Itoa(int(config.Port)),
		controlledProcesses: map[ControlledProcessToken]*ControlledProcess{},
	}

	s.logger = ctx.Logger().With().
		Str(core.SOURCE_LOG_FIELD_NAME, CONTROL_SERVER_LOG_SRC_KEY+"/"+s.port).Logger()

	websocketServer, err := net_ns.NewWebsocketServer(ctx)
	if err != nil {
		return nil, err
	}

	s.websocketServer = websocketServer
	return s, nil
}

func (s *ControlServer) host() core.Host {
	return core.Host("wss://localhost:" + s.port)
}

func (s *ControlServer) addr() string {
	return "localhost:" + s.port
}

// Start starts the HTTP server, it returns when the server is closed.
func (s *ControlServer) Start() error {
	httpServer, err := http_ns.NewGolangHttpServer(s.ctx, http_ns.GolangHttpServerConfig{
		Addr: s.addr(),

		//the configuration is very strict in order to quickly ignore connections from processes that are not controlled.
		ReadHeaderTimeout: 10 * time.Millisecond,
		MaxHeaderBytes:    200,
		ReadTimeout:       10 * time.Millisecond,
		WriteTimeout:      10 * time.Millisecond,

		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := ControlledProcessTokenFrom(r.Header.Get(PROCESS_TOKEN_HEADER))

			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			process, ok := s.gerControlledProcess(token)
			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			if process.isAlreadyConnected() {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			socket, err := s.websocketServer.UpgradeGoValues(w, r, s.allowConnection)
			if err != nil {
				s.logger.Err(err).Send()
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			process.setSocket(socket)
		}),
	})

	if err != nil {
		return err
	}

	s.httpServer = httpServer

	s.ctx.OnGracefulTearDown(func(ctx *core.Context) error {
		return s.httpServer.Shutdown(ctx)
	})

	s.ctx.OnDone(func(timeoutCtx context.Context) error {
		return s.httpServer.Close()
	})

	s.logger.Info().Msg("start HTTPS server")
	err = httpServer.ListenAndServeTLS("", "")
	if err != nil {
		return fmt.Errorf("failed to create HTTPS server: %w", err)
	}
	return nil
}

func (s *ControlServer) allowConnection(
	remoteAddrPort nettypes.RemoteAddrWithPort,
	remoteAddr nettypes.RemoteIpAddr, currentConns []*net_ns.WebsocketConnection) error {

	return nil
}

// CreateClientProcess create an Inox process that connects to the server.
func (s *ControlServer) CreateControlledProcess(grantedPerms, forbiddenPerms []core.Permission) (*ControlledProcess, error) {
	//create arguments

	grantedPerms = slices.Clone(grantedPerms)
	grantedPerms = append(grantedPerms,
		core.WebsocketPermission{Kind_: permkind.Read, Endpoint: core.Host(s.host())},
		core.WebsocketPermission{Kind_: permkind.Write, Endpoint: core.Host(s.host())},
	)

	token := MakeControlledProcessToken()
	var grantedPermsArg string   //gob+base64
	var forbiddenPermsArg string //gob+base64
	{
		core.RegisterPermissionTypesInGob()
		core.RegisterSimpleValueTypesInGob()

		//encode granted permissions
		buf := bytes.Buffer{}
		encoder := gob.NewEncoder(&buf)
		if err := encoder.Encode(grantedPerms); err != nil {
			return nil, fmt.Errorf("failed to encode granted permissions: %w", err)
		}
		grantedPermsArg = hex.EncodeToString(buf.Bytes())

		//encode forbidden permissions
		buf.Reset()
		encoder = gob.NewEncoder(&buf)
		if err := encoder.Encode(forbiddenPerms); err != nil {
			return nil, fmt.Errorf("failed to encode forbidden permissions: %w", err)
		}
		forbiddenPermsArg = hex.EncodeToString(buf.Bytes())
	}

	//prepare the command

	cmd := exec.Command(s.config.InoxBinaryPath, CONTROLLED_SUBCMD, string(s.host()+"/"), string(token), grantedPermsArg, forbiddenPermsArg)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	process := &ControlledProcess{
		cmd:       cmd,
		token:     token,
		id:        ulid.Make().String(),
		connected: make(chan struct{}, 1),
	}

	s.addControlledProcess(process)

	//connect stdout & stderr to the logger
	logger := s.logger.With().Str("controlled process", process.id).Logger()

	go func() {
		defer utils.Recover()
		io.Copy(logger, stderr)
	}()

	go func() {
		defer utils.Recover()
		io.Copy(logger, stdout)
	}()

	//start the command

	if err := cmd.Start(); err != nil {
		s.removeControlledProcess(process)
		return nil, fmt.Errorf("failed to start controller process: %w", err)
	}

	//wait for the process to connect to the server
	t := time.NewTimer(MAX_CONTROLLED_PROCESS_CONNECTION_WAITING_TIME)
	defer t.Stop()

	select {
	case <-process.connected:
	case <-t.C:
		s.removeControlledProcess(process)
		return nil, ErrProcessDidNotConnect
	}

	return process, nil
}

func (s *ControlServer) addControlledProcess(process *ControlledProcess) {
	s.controlledProcessesLock.Lock()
	s.controlledProcesses[process.token] = process
	s.controlledProcessesLock.Unlock()
}

func (s *ControlServer) gerControlledProcess(token ControlledProcessToken) (*ControlledProcess, bool) {
	s.controlledProcessesLock.Lock()
	defer s.controlledProcessesLock.Unlock()
	process, ok := s.controlledProcesses[token]
	return process, ok
}

func (s *ControlServer) removeControlledProcess(process *ControlledProcess) {
	s.controlledProcessesLock.Lock()
	delete(s.controlledProcesses, process.token)
	s.controlledProcessesLock.Unlock()
	if process.cmd.Process != nil {
		process.cmd.Process.Kill()
	}
}
