package deno

import (
	"encoding/hex"
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/ws_ns"
	"github.com/inoxlang/inox/internal/utils/processutils"
	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v3/process"
)

const (
	GRACEFUL_CANCELLATION_TIMEOUT = 5 * time.Second
)

// A DenoProcess is a process running the deno binary.
type DenoProcess struct {
	serviceName           string
	logger                zerolog.Logger
	token                 ControlledProcessToken
	pid                   atomic.Int32
	id                    ulid.ULID
	socket                *ws_ns.WebsocketConnection
	receivedResponses     map[ /*id*/ string]*message
	receivedResponseDates map[ /*id*/ string]time.Time

	lock                 sync.Mutex
	connected            chan struct{}
	connectedAtLeastOnce atomic.Bool

	stoppedOrBeingStopped atomic.Bool
	killedOrBeingKilled   atomic.Bool
}

func (p *DenoProcess) isAlreadyConnected() bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.socket != nil && !p.socket.IsClosedOrClosing()
}

func (p *DenoProcess) setPID(pid int32) {
	p.pid.Store(pid)
}

func (p *DenoProcess) setSocket(socket *ws_ns.WebsocketConnection) error {

	p.lock.Lock()
	defer p.lock.Unlock()

	if p.socket != nil {
		if p.socket.IsClosedOrClosing() {
			p.socket = nil
		} else {
			return ErrProcessAlreadyConnected
		}
	}
	p.socket = socket
	p.connectedAtLeastOnce.Store(true)

	select {
	case p.connected <- struct{}{}:
	default:
		//avoid blocking
	}

	return nil
}

func (p *DenoProcess) addResponse(resp *message) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if resp.Id == "" {
		return
	}

	p.receivedResponses[resp.Id] = resp
}

// Stop gracefully stops the process or directly kills it, Stop does not wait for the process to exit.
func (p *DenoProcess) Stop(ctx *core.Context) {
	if !p.stoppedOrBeingStopped.CompareAndSwap(false, true) {
		return
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.connectedAtLeastOnce.Load() || p.socket.IsClosedOrClosing() {
		p.kill()
		//note: don't use .Exited and ExitCode
		return
	}
	defer p.kill()

	gracefulCancellationDeadline := time.Now().Add(GRACEFUL_CANCELLATION_TIMEOUT)
	stopAllRequest := makeRequestMessage(STOP_ALL_METHOD, nil)

	ctx.OnGracefulTearDown(func(ctx *core.Context) error {
		p.socket.Close()
		p.kill()
		return nil
	})

	err := p.socket.WriteMessage(ctx, ws_ns.WebsocketTextMessage, encodeMessage(stopAllRequest))
	if err != nil {
		p.kill()
		return
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if time.Now().After(gracefulCancellationDeadline) {
			return
		}

		msgType, payload, err := p.socket.ReadMessage(ctx)
		if err != nil {
			return
		}

		if msgType != ws_ns.WebsocketTextMessage {
			continue
		}

		var msg message

		if err := json.Unmarshal(payload, &msg); err != nil {
			return
		}

		switch string(msg.Method) {
		case ALL_STOPPED_EVENT:
			return
		}

		if msg.Kind == RESPONSE_KIND && msg.Id == stopAllRequest.Id && string(msg.Payload) == ALREADY_STOPPED_RESPONSE {
			return
		}

	}

}

func (p *DenoProcess) kill() {
	if !p.killedOrBeingKilled.CompareAndSwap(false, true) {
		return
	}

	pid := p.pid.Load()
	if pid != 0 {
		if yes, err := process.PidExists(pid); yes || err != nil {
			return
		}
		processutils.KillHiearachy(int(pid), p.logger)
	}
}

type ControlledProcessToken string

func ControlledProcessTokenFrom(s string) (ControlledProcessToken, bool) {
	if len(s) != PROCESS_TOKEN_ENCODED_BYTE_LENGTH {
		return "", false
	}
	_, err := hex.DecodeString(s)
	if err != nil {
		return "", false
	}
	return ControlledProcessToken(s), true
}

func MakeControlledProcessToken() ControlledProcessToken {
	return ControlledProcessToken(core.CryptoRandSource.ReadNBytesAsHex(PROCESS_TOKEN_BYTE_LENGTH))
}
