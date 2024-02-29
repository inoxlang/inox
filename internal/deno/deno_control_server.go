package deno

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/http_ns"
	"github.com/inoxlang/inox/internal/globals/ws_ns"
	netaddr "github.com/inoxlang/inox/internal/netaddr"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/rs/zerolog"
)

const (
	CONTROL_SERVER_LOG_SRC    = "control-server"
	PROCESS_TOKEN_QUERY_PARAM = "token"
	PROCESS_TOKEN_BYTE_LENGTH = 32

	CONTROLLED_SUBCMD = "controlled"

	//maximum time the control server will wait for the controlled process to connect.
	MAX_CONTROLLED_PROCESS_CONNECTION_WAITING_TIME = time.Second
)

var (
	PROCESS_TOKEN_ENCODED_BYTE_LENGTH = hex.EncodedLen(PROCESS_TOKEN_BYTE_LENGTH)
	ErrProcessDidNotConnect           = errors.New("controlled process did not connect to the control server")
	ErrProcessNotCurrentlyConnect     = errors.New("controlled process is not currently connected to the control server")
	ErrProcessAlreadyConnected        = errors.New("controlled process is already connected to the control server")
)

// A ControlServer creates and controls Deno processes, each created process connects to the control server with a WebSocket (HTTP, no encrytion).
type ControlServer struct {
	ctx             *core.Context
	config          ControlServerConfig
	port            string
	websocketServer *ws_ns.WebsocketServer

	httpServer *http.Server
	logger     zerolog.Logger

	controlledProcesses     map[ControlledProcessToken]*DenoProcess
	controlledProcessesLock sync.Mutex
}

type ControlServerConfig struct {
	Port uint16
}

func (c ControlServerConfig) Check() error {

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
		controlledProcesses: map[ControlledProcessToken]*DenoProcess{},
	}

	s.logger = ctx.Logger().With().
		Str(core.SOURCE_LOG_FIELD_NAME, CONTROL_SERVER_LOG_SRC+"/"+s.port).Logger()

	websocketServer, err := ws_ns.NewWebsocketServer(ctx)
	if err != nil {
		return nil, err
	}

	s.websocketServer = websocketServer
	return s, nil
}

func (s *ControlServer) host() core.Host {
	return core.Host("ws://localhost:" + s.port)
}

func (s *ControlServer) addr() string {
	return "localhost:" + s.port
}

// Start starts the HTTP server, it returns when the server is closed.
func (s *ControlServer) Start() error {
	httpServer, err := http_ns.NewGolangHttpServer(s.ctx, http_ns.GolangHttpServerConfig{
		Addr: s.addr(),

		//the configuration is very strict in order to quickly ignore connections from processes that are not controlled.
		ReadHeaderTimeout: 100 * time.Millisecond,
		MaxHeaderBytes:    200,
		ReadTimeout:       100 * time.Millisecond,
		WriteTimeout:      100 * time.Millisecond,

		//Upgrade connection to WebSocket.
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			token, ok := ControlledProcessTokenFrom(r.URL.Query().Get(PROCESS_TOKEN_QUERY_PARAM))

			if !ok {
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			process, ok := s.getControlledProcess(token)
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

			//Read the messages.
			go func() {
				defer utils.Recover()

				for !s.ctx.IsDoneSlowCheck() {
					msgType, payload, err := socket.ReadMessage(s.ctx)
					if socket.IsClosedOrClosing() {
						return
					}
					if err != nil {
						s.logger.Err(err).Send()
						continue
					}
					if msgType != ws_ns.WebsocketTextMessage {
						continue
					}

					var message message
					err = json.Unmarshal(payload, &message)

					if err != nil {
						s.logger.Err(err).Send()
						continue
					}

					process.addResponse(&message)
				}

			}()
		}),
	})

	if err != nil {
		return err
	}

	s.httpServer = httpServer

	s.ctx.OnGracefulTearDown(func(ctx *core.Context) error {
		return s.httpServer.Shutdown(ctx)
	})

	s.ctx.OnDone(func(timeoutCtx context.Context, teardownStatus core.GracefulTeardownStatus) error {
		return s.httpServer.Close()
	})

	s.logger.Info().Msg("start HTTP server")
	err = httpServer.ListenAndServe()
	if err != nil {
		return fmt.Errorf("failed to create HTTP server or HTTP server was closed: %w", err)
	}
	return nil
}

func (s *ControlServer) allowConnection(
	remoteAddrPort netaddr.RemoteAddrWithPort,
	remoteAddr netaddr.RemoteIpAddr, currentConns []*ws_ns.WebsocketConnection) error {

	return nil
}

func (s *ControlServer) addControlledProcess(process *DenoProcess) {
	s.controlledProcessesLock.Lock()
	s.controlledProcesses[process.token] = process
	s.controlledProcessesLock.Unlock()
}

func (s *ControlServer) getControlledProcess(token ControlledProcessToken) (*DenoProcess, bool) {
	s.controlledProcessesLock.Lock()
	defer s.controlledProcessesLock.Unlock()
	process, ok := s.controlledProcesses[token]
	return process, ok
}

func (s *ControlServer) removeControlledProcess(process *DenoProcess) {
	s.controlledProcessesLock.Lock()
	delete(s.controlledProcesses, process.token)
	s.controlledProcessesLock.Unlock()
	process.kill()
}
