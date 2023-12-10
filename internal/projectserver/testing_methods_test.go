package projectserver

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestTestingMethods(t *testing.T) {
	ctx, client, ok := createTestServerAndClient(t)
	if !ok {
		return
	}

	defer ctx.CancelGracefully()
	defer client.close()

	client.sendRequest(jsonrpc.RequestMessage{
		Method: "initialize",
		Params: utils.Must(json.Marshal(defines.InitializeParams{})),
	})

	time.Sleep(time.Millisecond)

	msg, ok := client.dequeueLastMessage()
	if !assert.True(t, ok) {
		return
	}

	resp := msg.(jsonrpc.ResponseMessage)

	t.Log(resp.Error)
}
