package inoxprocess

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/globals/net_ns"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	HEARTBEAT_INTERVAL = 100 * time.Millisecond
)

type ControlClient struct {
	conn             *net_ns.WebsocketConnection
	ctx              *core.Context
	token            ControlledProcessToken
	controlServerURL core.URL

	lock sync.Mutex
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

func (c *ControlClient) Conn() *net_ns.WebsocketConnection {
	c.lock.Lock()
	defer c.lock.Unlock()
	return c.conn
}

func (c *ControlClient) connect() error {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.conn != nil && !c.conn.IsClosedOrClosing() {
		c.ctx.Logger().Print("close connection with control server")
		c.conn.Close()
	}

	dialer := *websocket.DefaultDialer
	dialer.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
	}

	c.ctx.Logger().Print("(re)connect to control server")

	insecure := true
	conn, err := net_ns.WebsocketConnect(c.ctx, c.controlServerURL, insecure, http.Header{
		PROCESS_TOKEN_HEADER: []string{string(c.token)},
	})

	if err != nil {
		return err
	}

	c.conn = conn
	return nil
}

func (c *ControlClient) StartControl() {
	defer func() { //teardown
		if conn := c.Conn(); conn != nil {
			conn.Close()
		}
	}()

	go func() {
		ctx := c.ctx.BoundChild()
		ticker := time.NewTicker(HEARTBEAT_INTERVAL)
		defer ticker.Stop()

		for t := range ticker.C {

			select {
			case <-ctx.Done():
				return
			default:
			}

			conn := c.Conn()

			if conn.IsClosedOrClosing() {
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

			c.ctx.Logger().Print("send hearbeat to control server ", t)
			conn.WriteMessage(ctx, websocket.PingMessage, data)
		}
	}()

	for {
		conn := c.Conn()
		if conn.IsClosedOrClosing() {
			return
		}

		select {
		case <-c.ctx.Done():
			return
		default:
			msgType, p, err := conn.ReadMessage(c.ctx)
			isEof := errors.Is(err, io.EOF)
			isWebsocketUnexpectedClose := websocket.IsUnexpectedCloseError(err)
			isClosedWebsocket := errors.Is(err, net_ns.ErrClosingOrClosedWebsocketConn)
			isNetReaderr := utils.Implements[*net.OpError](err) && err.(*net.OpError).Op == "read"

			if isEof || isWebsocketUnexpectedClose || isClosedWebsocket || isNetReaderr {
				c.connect()
				continue
			}

			c.handleMessage(msgType, p, err)
		}
	}
}

func (c *ControlClient) handleMessage(messageType net_ns.WebsocketMessageType, p []byte, err error) {
	switch messageType {
	case net_ns.WebsocketBinaryMessage:
	case net_ns.WebsocketTextMessage:
	case net_ns.WebsocketPingMessage:
		c.conn.WriteMessage(c.ctx, net_ns.WebsocketPongMessage, nil)
	case net_ns.WebsocketPongMessage:
	}
}

type heartbeat struct {
	Time time.Time `json:"time"`
}
