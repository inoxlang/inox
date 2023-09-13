package net_ns

import (
	"net"
	"testing"
	"time"

	core "github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/permkind"
	"github.com/stretchr/testify/assert"
)

func TestTcpConn(t *testing.T) {
	localhost, err := net.ResolveTCPAddr("tcp", "localhost:0")
	assert.NoError(t, err)

	makeListener := func() (*net.TCPListener, chan (int), core.Host) {
		listener, err := net.ListenTCP("tcp", localhost)
		assert.NoError(t, err)
		stop := make(chan int)

		go func() {
			conn, err := listener.AcceptTCP()
			if err != nil {
				t.Log(err)
				return
			}
			<-stop
			conn.Close()
		}()

		return listener, stop, core.Host("://" + listener.Addr().String())
	}

	t.Run("after a failed read due to a closed connection the total number of allowed TCP connections should increase", func(t *testing.T) {
		_, stop, host := makeListener()

		ctx := core.NewContext(core.ContextConfig{
			Permissions: []core.Permission{
				core.RawTcpPermission{Kind_: permkind.Read, Domain: host},
				core.RawTcpPermission{Kind_: permkind.WriteStream, Domain: host},
			},
			Limits: []core.Limit{{Name: TCP_SIMUL_CONN_TOTAL_LIMIT_NAME, Kind: core.TotalLimit, Value: 1}},
		})

		conn, err := tcpConnect(ctx, host)
		assert.NoError(t, err)

		//we check that there are no tokens left
		total, err := ctx.GetTotal(TCP_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)

		stop <- 0
		time.Sleep(time.Millisecond)
		conn.read(ctx)

		//after the failed read the TcpConn should have given back the tokens
		total, err = ctx.GetTotal(TCP_SIMUL_CONN_TOTAL_LIMIT_NAME)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)
	})

}
