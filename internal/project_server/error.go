package project_server

import (
	"fmt"

	"github.com/inoxlang/inox/internal/project_server/jsonrpc"
)

func fmtInternalError(s string, args ...any) jsonrpc.ResponseError {
	return jsonrpc.ResponseError{
		Code:    jsonrpc.InternalError.Code,
		Message: fmt.Sprintf(s, args...),
	}
}
