package projectserver

import (
	"encoding/json"

	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
	"github.com/inoxlang/inox/internal/projectserver/lsp/defines"
	"github.com/inoxlang/inox/internal/utils"
)

func NewShowMessage(typ defines.MessageType, text string) jsonrpc.NotificationMessage {
	return jsonrpc.NotificationMessage{
		Method: "window/showMessage",
		Params: utils.Must(json.Marshal(defines.ShowMessageParams{
			Type:    typ,
			Message: text,
		})),
	}
}
