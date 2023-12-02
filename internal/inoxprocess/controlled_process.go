package inoxprocess

import (
	"encoding/hex"
	"os/exec"
	"sync"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/net_ns"
)

// A ControlledProcess is an Inox process controlled by the control server,
// it is not necessarily running on the same machine.
type ControlledProcess struct {
	cmd    *exec.Cmd
	token  ControlledProcessToken
	id     string //TODO: make this ID globally unique
	socket *net_ns.WebsocketConnection

	lock      sync.Mutex
	connected chan struct{}
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

	select {
	case p.connected <- struct{}{}:
	default:
		//avoid blocking
	}

	return nil
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
