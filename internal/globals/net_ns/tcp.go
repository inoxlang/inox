package net_ns

import (
	"errors"
	"io"
	"net"
	"sync/atomic"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
)

type TcpConn struct {
	initialCtx *core.Context
	host       core.Host
	conn       *net.TCPConn
	closed     int32 //prevent giving back tokens
}

func (conn *TcpConn) GetGoMethod(name string) (*core.GoFunction, bool) {
	switch name {
	case "read":
		return core.WrapGoMethod(conn.read), true
	case "write":
		return core.WrapGoMethod(conn.write), true
	case "close":
		return core.WrapGoMethod(conn.close), true
	}
	return nil, false
}

func (conn *TcpConn) Prop(ctx *core.Context, name string) core.Value {
	method, ok := conn.GetGoMethod(name)
	if !ok {
		panic(core.FormatErrPropertyDoesNotExist(name, conn))
	}
	return method
}

func (*TcpConn) SetProp(ctx *core.Context, name string, value core.Value) error {
	return core.ErrCannotSetProp
}

func (*TcpConn) PropertyNames(ctx *core.Context) []string {
	return []string{"read", "write", "close"}
}

func (conn *TcpConn) read(ctx *core.Context) (*core.ByteSlice, error) {
	if atomic.LoadInt32(&conn.closed) != 0 {
		return &core.ByteSlice{}, errors.New("closed")
	}

	perm := core.RawTcpPermission{
		Kind_:  permkind.Read,
		Domain: conn.host,
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return &core.ByteSlice{}, err
	}

	buff := make([]byte, 1<<16)
	n, err := conn.conn.Read(buff)

	if err == io.EOF {
		conn.close(ctx)
	}

	return &core.ByteSlice{Bytes: buff[:n], IsDataMutable: true}, err
}

func (conn *TcpConn) write(ctx *core.Context, data core.Readable) error {
	if atomic.LoadInt32(&conn.closed) != 0 {
		return errors.New("closed")
	}

	perm := core.RawTcpPermission{
		Kind_:  permkind.WriteStream,
		Domain: conn.host,
	}

	if err := ctx.CheckHasPermission(perm); err != nil {
		return err
	}

	reader := data.Reader()
	_, err := io.Copy(conn.conn, reader)

	if err != nil {
		if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
			conn.close(ctx)
		}
	}

	return err
}

func (conn *TcpConn) close(ctx *core.Context) {
	if atomic.LoadInt32(&conn.closed) != 0 {
		return
	}
	atomic.StoreInt32(&conn.closed, 1)

	conn.initialCtx.GiveBack(TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, 1)
	conn.conn.Close()
}

//
