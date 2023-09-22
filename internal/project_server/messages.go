package project_server

import (
	"encoding/json"

	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
	"github.com/inoxlang/inox/internal/project_server/lsp/defines"
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
