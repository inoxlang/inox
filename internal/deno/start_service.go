package deno

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/deno/binary"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"
)

const (
	SERVICE_FILENAME = "service.ts"
	CACHE_DIRNAME    = ".cache"
)

type ServiceConfiguration struct {
	RequiresPersistendWorkdir bool
	Name                      string
	DenoBinaryLocation        string
	ServiceProgram            string //This program is written to SERVICE_FILENAME in the service's directory.
	SrcDir                    fs.FS  //Written to src/ in the service's directory. (TODO, make sure no files are written outside of the service's directory)

	AllowNetwork         bool
	AllowLocalhostAccess bool
}

// StartServiceProcess starts a Deno service in a goroutine.
func (s *ControlServer) StartServiceProcess(ctx *core.Context, config ServiceConfiguration) (_ ulid.ULID, earlyErr error) {

	if config.RequiresPersistendWorkdir {
		earlyErr = fmt.Errorf("services requiring a persistent working directory are not supported for now")
		return
	}

	err := binary.Install(config.DenoBinaryLocation)
	if err != nil {
		earlyErr = err
		return
	}

	dir := fs_ns.CreateDirInProcessTempDir("internal-service-" + config.Name)
	serviceFilePath := dir.JoinEntry(SERVICE_FILENAME)
	denoDir := dir.JoinEntry(CACHE_DIRNAME)

	err = fs_ns.Mkfile(ctx, serviceFilePath, core.String(config.ServiceProgram))
	if err != nil {
		earlyErr = err
		return
	}

	errChan := make(chan error)

	token := MakeControlledProcessToken()
	configJSON := string(utils.Must(json.Marshal(map[string]string{
		"controlServerPort": s.port,
		"token":             string(token),
	})))

	process := &DenoProcess{
		serviceName:           config.Name,
		token:                 token,
		id:                    ulid.Make(),
		connected:             make(chan struct{}, 1),
		logger:                zerolog.Nop(),
		receivedResponses:     map[string]*message{},
		receivedResponseDates: map[string]time.Time{},
	}

	s.addControlledProcess(process)

	startEventChan := make(chan int32, 1)

	//Start a goroutine setting the PID of $process.
	go func() {
		defer utils.Recover()

		for {
			select {
			case pid := <-startEventChan:
				process.setPID(pid)
			case <-ctx.Done():
				s.removeControlledProcess(process)
			}
		}
	}()

	//Start process.
	go func() {
		defer utils.Recover()
		defer s.removeControlledProcess(process)

		errChan <- binary.ExecuteWithAutoRestart(ctx, binary.Execution{
			Location:            config.DenoBinaryLocation,
			AbsoluteWorkDir:     dir.UnderlyingString(),
			AbsoluteDenoDir:     denoDir.UnderlyingString(),
			RelativeProgramPath: SERVICE_FILENAME,

			Logger:         process.logger,
			StartEventChan: startEventChan,

			AllowNetwork:         config.AllowNetwork,
			AllowLocalhostAccess: config.AllowLocalhostAccess,
			CLIArguments: []string{
				//The first argument is the service configuration.
				configJSON,
			},
		})
	}()

	select {
	case earlyErr = <-errChan:
		s.removeControlledProcess(process)
		return
	case <-process.connected:
		return process.id, nil
	case <-time.After(time.Second):
		s.removeControlledProcess(process)
		earlyErr = ErrProcessDidNotConnect
		return
	}
}
