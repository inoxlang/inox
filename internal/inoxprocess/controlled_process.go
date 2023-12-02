package inoxprocess

import (
	"encoding/hex"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	"github.com/inoxlang/inox/internal/inoxd/cloud/cloudproxy/inoxdconn"
	"github.com/inoxlang/inox/internal/utils/processutils"
	"github.com/oklog/ulid/v2"
	"github.com/rs/zerolog"
	"github.com/shirou/gopsutil/v3/process"
)

const (
	GRACEFUL_CANCELLATION_TIMEOUT = 5 * time.Second
)

// A ControlledProcess is an Inox process controlled by the control server,
// it is not necessarily running on the same machine.
type ControlledProcess struct {
	cmd    *exec.Cmd
	logger zerolog.Logger
	token  ControlledProcessToken
	id     string //TODO: make this ID globally unique
	socket *net_ns.WebsocketConnection

	lock                 sync.Mutex
	connected            chan struct{}
	connectedAtLeastOnce atomic.Bool
	killedOrBeingKilled  atomic.Bool
}

func (p *ControlledProcess) isAlreadyConnected() bool {
	p.lock.Lock()
	defer p.lock.Unlock()

	return p.socket != nil && !p.socket.IsClosedOrClosing()
}

func (p *ControlledProcess) setSocket(socket *net_ns.WebsocketConnection) error {

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

// Stop gracefully stops the process if process or kills it, Stop does not wait for the process to exit.
func (p *ControlledProcess) Stop(ctx *core.Context) {
	p.lock.Lock()
	defer p.lock.Unlock()

	if !p.connectedAtLeastOnce.Load() || p.socket.IsClosedOrClosing() {
		p.kill()
		//note: don't use .Exited and ExitCode
		return
	}
	defer p.kill()

	stopAllRequest := Message{
		ULID:  ulid.Make(),
		Inner: StopAllRequest{},
	}

	err := p.socket.WriteMessage(ctx, net_ns.WebsocketBinaryMessage, MustEncodeMessage(stopAllRequest))
	if err != nil {
		p.kill()
		return
	}

	gracefulCancellationDeadline := time.Now().Add(GRACEFUL_CANCELLATION_TIMEOUT)

	for {
		if time.Now().After(gracefulCancellationDeadline) {
			return
		}

		msgType, payload, err := p.socket.ReadMessage(ctx)
		if err != nil {
			return
		}

		if msgType != net_ns.WebsocketBinaryMessage {
			continue
		}

		var msg Message
		if err := DecodeMessage(payload, &msg); err != nil {
			return
		}

		switch m := msg.Inner.(type) {
		case AllStoppedEvent:
			return
		case StopAllResponse:
			if m.AlreadyStopped {
				return
			}
		}

	}

}

func (p *ControlledProcess) sendAck(ctx *core.Context, msgULID ulid.ULID) error {
	//send Ack message to control server
	ack := inoxdconn.Message{
		ULID:  ulid.Make(),
		Inner: AckMsg{AcknowledgedMessage: msgULID},
	}

	err := p.socket.WriteMessage(ctx, net_ns.WebsocketBinaryMessage, inoxdconn.MustEncodeMessage(ack))
	if err != nil {
		//TODO: log errors
		return err
	}
	return nil
}

func (p *ControlledProcess) kill() {
	if !p.killedOrBeingKilled.CompareAndSwap(false, true) {
		return
	}
	if yes, err := process.PidExists(int32(p.cmd.Process.Pid)); yes || err != nil {
		return
	}
	processutils.KillHiearachy(p.cmd.Process.Pid, p.logger)
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
