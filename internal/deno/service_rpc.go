package deno

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/ws_ns"
	"github.com/oklog/ulid/v2"
)

const (
	METHOD_CALL_TIMEOUT        = 5 * time.Second
	RESPONSE_DISCARD_THRESHOLD = 10 * time.Second
)

var (
	ErrRemoteCallTimeout = errors.New("remote call timeout")
)

func (s *ControlServer) GetServiceProcessByID(ulid ulid.ULID) (*DenoProcess, bool) {
	s.controlledProcessesLock.Lock()
	defer s.controlledProcessesLock.Unlock()

	for _, process := range s.controlledProcesses {
		if process.id == ulid {
			return process, true
		}
	}

	return nil, false
}

func (p *DenoProcess) CallMethod(ctx *core.Context, method string, payload any) (json.RawMessage, error) {
	if p.killedOrBeingKilled.Load() {
		return nil, ErrProcessKilledOrBeingKilled
	}

	start := time.Now()

	p.lock.Lock()
	socket := p.socket
	p.lock.Unlock()

	if socket == nil {
		return nil, ErrProcessNotCurrentlyConnect
	}

	msg := makeRequestMessage(method, payload)

	err := socket.WriteMessage(ctx, ws_ns.WebsocketTextMessage, encodeMessage(msg))
	if err != nil {
		return nil, err
	}

	if time.Since(start) > METHOD_CALL_TIMEOUT {
		return nil, fmt.Errorf("%w: sending the request has taken too long", ErrRemoteCallTimeout)
	}

	if socket.IsClosedOrClosing() {
		return nil, errors.New("socket is closed or closing")
	}

	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			if time.Since(start) > METHOD_CALL_TIMEOUT {
				return nil, ErrRemoteCallTimeout
			}
			if p.killedOrBeingKilled.Load() {
				return nil, ErrProcessKilledOrBeingKilled
			}
		}

		now := time.Now()

		resp := func() *message {
			p.lock.Lock()
			defer p.lock.Unlock()

			//Remove ignored responses.
			for id, reponseTime := range p.receivedResponseDates {
				if now.Sub(reponseTime) > RESPONSE_DISCARD_THRESHOLD {
					delete(p.receivedResponseDates, id)
					delete(p.receivedResponses, id)
				}
			}

			//May be nil.
			return p.receivedResponses[msg.Id]
		}()

		if resp != nil {
			return resp.Payload, nil
		}
	}
}
