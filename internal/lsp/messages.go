package internal

import (
	"encoding/json"

	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
	"github.com/inoxlang/inox/internal/lsp/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

func NewShowMessage(typ defines.MessageType, text string) jsonrpc.NotificationMessage {
	return jsonrpc.NotificationMessage{
		BaseMessage: jsonrpc.BaseMessage{
			Jsonrpc: JSONRPC_VERSION,
		},
		Method: "window/showMessage",
		Params: utils.Must(json.Marshal(defines.ShowMessageParams{
			Type:    typ,
			Message: text,
		})),
	}
}
