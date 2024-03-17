package inoxprocess

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/ws_ns"
	"github.com/inoxlang/inox/internal/inoxd/cloud/cloudproxy/inoxdconn"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
)

const (
	HEARTBEAT_INTERVAL          = 100 * time.Millisecond
	MAX_RECONNECT_ATTEMPT_COUNT = 10
	LOCAL_WS_HANDSHAKE_TIMEOUT  = time.Second
	INIT_PROCESS_PID            = 1
)

var (
	ErrControlLoopEnd           = errors.New("control loop end")
	ErrTooManyReconnectAttempts = errors.New("too many reconnect attemps")
	ErrOrphanProcess            = errors.New("orphan process")
)

type ControlClient struct {
	lock sync.Mutex

	conn             *ws_ns.WebsocketConnection
	reconnectAttemps atomic.Int32

	ctx              *core.Context
	token            ControlledProcessToken
	controlServerURL core.URL

	executionContext context.Context
	executing        atomic.Bool
}

func ConnectToProcessControlServer(ctx *core.Context, u *url.URL, token ControlledProcessToken) (*ControlClient, error) {

	if u.Scheme != "wss" {
		return nil, errors.New("url's scheme should be wss")
	}

	if u.Hostname() != "localhost" {
		return nil, errors.New("url's hostname should be localhost")
	}

	client := &ControlClient{
		ctx:              ctx,
		token:            token,
		controlServerURL: core.URL(u.String()),
	}

	if err := client.connect(); err != nil {
		return nil, err
	}

	return client, nil
}

func (c *ControlClient) Conn() *ws_ns.WebsocketConnection {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.conn
}

func (c *ControlClient) connect() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.conn != nil && !c.conn.IsClosedOrClosing() {
		c.ctx.LoggerPrint("close connection with control server")
		c.conn.Close()
	}

	dialer := *websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	c.ctx.LoggerPrint("(re)connect to control server")

	insecure := true
	conn, err := ws_ns.WebsocketConnect(ws_ns.WebsocketConnectParams{
		Ctx:      c.ctx,
		URL:      c.controlServerURL,
		Insecure: insecure,
		RequestHeader: http.Header{
			PROCESS_TOKEN_HEADER: []string{string(c.token)},
		},
		HandshakeTimeout: LOCAL_WS_HANDSHAKE_TIMEOUT,
	})

	if err != nil {
		c.reconnectAttemps.Add(1)
		return err
	}

	c.conn = conn
	if !conn.IsClosedOrClosing() {
		c.reconnectAttemps.Store(0)
	}

	return nil
}

func (c *ControlClient) StartControl() error {
	defer func() { //teardown
		if conn := c.Conn(); conn != nil {
			conn.Close()
		}
	}()

	heartbeatCtx := c.ctx.BoundChild()
	defer heartbeatCtx.CancelGracefully()
	defer c.stopApplication()

	//send hearbeats
	go func() {
		defer utils.Recover()

		ticker := time.NewTicker(HEARTBEAT_INTERVAL)
		defer ticker.Stop()

		for t := range ticker.C {

			select {
			case <-heartbeatCtx.Done():
				return
			default:
			}

			conn := c.Conn()

			if conn == nil || conn.IsClosedOrClosing() {
				time.Sleep(10 * time.Millisecond)
				continue
			}

			heartbeat := heartbeat{
				Time: t,
			}

			data, err := json.Marshal(heartbeat)
			if err != nil {
				continue
			}

			logger := heartbeatCtx.Logger()
			logger.Print("send hearbeat to control server ", t)
			err = conn.WriteMessage(heartbeatCtx, websocket.PingMessage, data)
			if err != nil {
				logger.Err(err).Send()
			}
		}
	}()

	//handle messages from the control server
	for {
		select {
		case <-c.ctx.Done():
			if c.conn != nil {
				c.conn.Close()
			}
			return c.ctx.Err()
		default:
		}

		if c.reconnectAttemps.Load() > MAX_RECONNECT_ATTEMPT_COUNT {
			return ErrTooManyReconnectAttempts
		}

		if os.Getppid() == INIT_PROCESS_PID {
			return ErrOrphanProcess
		}

		conn := c.Conn()
		if conn == nil || conn.IsClosedOrClosing() {
			time.Sleep(10 * time.Millisecond)
			c.connect()
			continue
		}

		msgType, p, err := conn.ReadMessage(c.ctx)

		if err != nil {
			//TODO: investigate exit status -13 after parent process is done.
			c.connect()
			continue
		}

		resp, sendResp, endLoop := c.handleMessage(msgType, p, err)
		if sendResp {
			msg := Message{
				ULID:  ulid.Make(),
				Inner: resp,
			}
			conn.WriteMessage(c.ctx, ws_ns.WebsocketBinaryMessage, MustEncodeMessage(msg))
		}
		if endLoop {
			return ErrControlLoopEnd
		}
	}
}

func (c *ControlClient) handleMessage(messageType ws_ns.WebsocketMessageType, p []byte, err error) (response any, sendResp bool, endLoop bool) {
	//TODO: log errors

	switch messageType {
	case ws_ns.WebsocketBinaryMessage:
		if err != nil {
			return
		}

		var msg Message
		if err := DecodeMessage(p, &msg); err != nil {
			return
		}

		if err = c.sendAck(msg.ULID); err != nil {
			return
		}

		switch m := msg.Inner.(type) {
		case LaunchApplicationRequest:
			if c.executing.Load() {
				response = LaunchAppResponse{
					Request: msg.ULID,
					Error:   ErrAlreadyExecuting,
				}
				sendResp = true
				endLoop = true
				return
			}
			go c.executeApplication(m.AppDir)
		case StopAllRequest:
			if !c.executing.Load() {
				response = StopAllResponse{
					AlreadyStopped: true,
				}
				sendResp = true
				endLoop = true
				return
			}
			//TODO
		}

	case ws_ns.WebsocketTextMessage:
	case ws_ns.WebsocketPingMessage:
		c.conn.WriteMessage(c.ctx, ws_ns.WebsocketPongMessage, nil)
	case ws_ns.WebsocketPongMessage:
	}

	return
}

func (c *ControlClient) sendAck(msgULID ulid.ULID) error {
	//send Ack message to control server
	ack := inoxdconn.Message{
		ULID:  ulid.Make(),
		Inner: AckMsg{AcknowledgedMessage: msgULID},
	}

	err := c.conn.WriteMessage(c.ctx, ws_ns.WebsocketBinaryMessage, inoxdconn.MustEncodeMessage(ack))
	if err != nil {
		//TODO: log errors
		return err
	}
	return nil
}

func (c *ControlClient) executeApplication(appDir string) {
	defer utils.Recover()

	//TODO
}

func (c *ControlClient) stopApplication() {

	//TODO
}

type heartbeat struct {
	Time time.Time `json:"time"`
}
