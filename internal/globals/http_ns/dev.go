package http_ns

import (
	"encoding/hex"
	"fmt"
	"io/fs"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/inoxconsts"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	MAX_DEV_SERVER_STARTUP_PERIOD = time.Second
	DEV_SERVER_DIR_PERMS          = fs.FileMode(0o700)
	DEV_SESSION_KEY_BYTE_COUNT    = 16

	CTX_DATA_KEY_FOR_DEV_SESSION_KEY           = core.Path("/dev-session-key") //TODO: should the session have a random name in order to not be retrievable ?
	DEV_SERVER_EXPLANATION_MESSAGE_FOR_BROWSER = "This server is not expected to receive requests from a browser."
)

var (
	devServers     = map[ /*port*/ string]*DevServer{}
	devServersLock sync.Mutex

	//Session key used if the header is missing.
	fallbackDevSessionKey atomic.Value
)

type DevServer struct {
	lock          sync.Mutex
	ctx           *core.Context
	dir           string
	port          string
	server        *http.Server //Server receiving the requests.
	targetServers map[DevSessionKey]*HttpsServer
}

type DevServerConfig struct {
	DevServersDir       string //directory reserved for development servers.
	Port                string //Note: the permission is not checked on the context.
	IgnoreDevPortCheck  bool
	BindToAllInterfaces bool
}

// StartDevServer starts a multiplexed development server. The DEV_SESSION_KEY_HEADER of a request is used to determine the HttpsServer
// to send the request to. DEV_SERVER_STARTUP_PERIOD after the start of the call StartDevServer returns any early error returned by the
// server (or nil). StartDevServer can return way before DEV_SERVER_STARTUP_PERIOD for several reasons:
// - $port is not a development port
// - $ctx is done
// - The creation of the server failed (NewGolangHttpServer)
// - A development port is already listening on localhost:$port or :$port (all interfaces).
// The server closes when $ctx is done.
func StartDevServer(ctx *core.Context, config DevServerConfig) error {

	devServersDir := config.DevServersDir
	port := config.Port
	bindToAllInterfaces := config.BindToAllInterfaces

	//Check arguments.

	if !inoxconsts.IsDevPort(port) {
		return fmt.Errorf("%s is not a dev port", port)
	}

	if ctx.IsDoneSlowCheck() {
		return ctx.Err()
	}

	devServersLock.Lock()
	defer devServersLock.Unlock()

	if devServers[port] != nil {
		return fmt.Errorf("a dev server is already running on :%s", port)
	}

	//Create some directories if needed.

	fls := ctx.GetFileSystem()
	if err := fls.MkdirAll(string(devServersDir), DEV_SERVER_DIR_PERMS); err != nil {
		return fmt.Errorf("failed to create the directory reserved for development servers: %w", err)
	}

	serverDir := fls.Join(string(devServersDir), port)
	if err := fls.MkdirAll(string(serverDir), DEV_SERVER_DIR_PERMS); err != nil {
		return fmt.Errorf("failed to create the directory reserved for the development server of port %s: %w", port, err)
	}

	//Create the server.

	addr := ""
	if bindToAllInterfaces {
		addr = ":" + port
	} else {
		addr = "localhost:" + port
	}

	devServer := &DevServer{
		targetServers: make(map[DevSessionKey]*HttpsServer),
		ctx:           ctx,
		dir:           serverDir,
		port:          port,
	}

	server, err := NewGolangHttpServer(ctx, GolangHttpServerConfig{
		Addr:                                     addr,
		PersistCreatedLocalCert:                  true,
		AllowSelfSignedCertCreationEvenIfExposed: true,
		GeneratedCertDir:                         core.DirPathFrom(serverDir),
		Handler:                                  http.HandlerFunc(devServer.Handle),
	})

	if err != nil {
		return fmt.Errorf("failed to create dev server for dev port %s: %w", port, err)
	}

	devServer.server = server

	earlyErrorChan := make(chan error, 1)

	//Close the server when $ctx is done.
	go func() {
		defer utils.Recover()
		<-ctx.Done()
		server.Close()
	}()

	go func() {
		defer func() {
			close(earlyErrorChan)

			//Remove the server.

			devServersLock.Lock()
			defer devServersLock.Unlock()

			delete(devServers, port)
		}()

		earlyErrorChan <- server.ListenAndServeTLS("", "")
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-earlyErrorChan:
		return err
	case <-time.After(MAX_DEV_SERVER_STARTUP_PERIOD):
		devServers[port] = devServer
		return nil
	}
}

func GetDevServer(port string) (*DevServer, bool) {
	devServersLock.Lock()
	defer devServersLock.Unlock()

	server, ok := devServers[port]
	return server, ok
}

func (s *DevServer) Handle(rw http.ResponseWriter, req *http.Request) {

	const ERROR_MSG_PREFIX_FMT = "[Dev server on port %s]\n" + DEV_SERVER_EXPLANATION_MESSAGE_FOR_BROWSER + "\n\n"

	keys := req.Header.Values(inoxconsts.DEV_SESSION_KEY_HEADER)
	var devSessionKey DevSessionKey

	switch len(keys) {
	case 0:
		key, ok := fallbackDevSessionKey.Load().(DevSessionKey)
		if ok && key != "" {
			devSessionKey = key
			break
		}
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, ERROR_MSG_PREFIX_FMT+"Missing %s header.", s.port, inoxconsts.DEV_SESSION_KEY_HEADER)
		return
	case 1:
		key, err := DevSessionKeyFrom(keys[0])
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(rw, ERROR_MSG_PREFIX_FMT+"Invalid %s header.", s.port, inoxconsts.DEV_SESSION_KEY_HEADER)
			return
		}
		devSessionKey = key
	default:
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, ERROR_MSG_PREFIX_FMT+"Duplicate %s header.", s.port, inoxconsts.DEV_SESSION_KEY_HEADER)
		return
	}

	s.lock.Lock()
	s.removeDeadTargetServersNoLock()
	targetServer, ok := s.targetServers[devSessionKey]
	s.lock.Unlock()

	if !ok {
		rw.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(rw, ERROR_MSG_PREFIX_FMT+"No target server found. Have you started it ?", s.port)
		return
	}

	targetServer.wrappedServer.Handler.ServeHTTP(rw, req)
}

func (s *DevServer) SetTargetServer(devSessionKey DevSessionKey, server *HttpsServer) bool {
	s.lock.Lock()
	defer s.lock.Unlock()

	//Do no add (or remove) the server if its context is done.
	if server.state.Ctx.IsDoneSlowCheck() {
		delete(s.targetServers, devSessionKey)
		return false
	}

	if s.targetServers[devSessionKey] != nil {
		return false
	}

	s.targetServers[devSessionKey] = server
	return true
}

func (s *DevServer) UnsetTargetServer(devSessionKey DevSessionKey) {
	s.lock.Lock()
	defer s.lock.Unlock()

	delete(s.targetServers, devSessionKey)
}

func (s *DevServer) GetTargetServer(devSessionKey DevSessionKey) (*HttpsServer, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()

	targetServer, ok := s.targetServers[devSessionKey]
	return targetServer, ok
}

func (s *DevServer) removeDeadTargetServersNoLock() {
	for devSessionKey, server := range s.targetServers {
		if server.state.Ctx.IsDoneSlowCheck() {
			delete(s.targetServers, devSessionKey)
		}
	}
}

// The primary use of a development session key is informing the developement server receiving the request what is the target server
// the request should be forwarded to. The key is stored as context data and is added in the inoxconsts.DEV_SESSION_KEY_HEADER header
// by HTTP clients.
type DevSessionKey string

func RandomDevSessionKey() DevSessionKey {
	return DevSessionKey(core.CryptoRandSource.ReadNBytesAsHex(DEV_SESSION_KEY_BYTE_COUNT))
}

func DevSessionKeyFrom(s string) (DevSessionKey, error) {
	if hex.DecodedLen(len(s)) != DEV_SESSION_KEY_BYTE_COUNT {
		return "", fmt.Errorf("invalid development session key: invalid length")
	}
	_, err := hex.DecodeString(s)
	if err != nil {
		return "", fmt.Errorf("invalid development session key")
	}

	return DevSessionKey(s), nil
}

func SetFallbackDevSessionKey(key DevSessionKey) {
	fallbackDevSessionKey.Store(key)
}

func RemoveFallbackDevSessionKey() {
	fallbackDevSessionKey.Store(DevSessionKey(""))
}
