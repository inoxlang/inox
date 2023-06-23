package internal

import (
	"fmt"

	"github.com/inoxlang/inox/internal/lsp/jsonrpc"
)

func fmtInternalError(s string, args ...any) jsonrpc.ResponseError {
	return jsonrpc.ResponseError{
		Code:    jsonrpc.InternalError.Code,
		Message: fmt.Sprintf(s, args...),
	}
}
