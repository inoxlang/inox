package projectserver

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestTestingMethods(t *testing.T) {

	t.SkipNow()

	ctx, client, ok := createTestServerAndClient(t, os.Stdout)
	if !ok {
		return
	}

	defer ctx.CancelGracefully()
	defer client.close()

	client.sendRequest(jsonrpc.RequestMessage{
		Method: "initialize",
		Params: utils.Must(json.Marshal(defines.InitializeParams{})),
	})

	time.Sleep(time.Second)

	msg, ok := client.dequeueLastMessage()
	if !assert.True(t, ok) {
		return
	}

	resp := msg.(jsonrpc.ResponseMessage)

	t.Log(resp.Error)
}
