package projectserver

import (
	"fmt"

	"github.com/inoxlang/inox/internal/projectserver/jsonrpc"
)

func fmtInternalError(s string, args ...any) jsonrpc.ResponseError {
	return jsonrpc.ResponseError{
		Code:    jsonrpc.InternalError.Code,
		Message: fmt.Sprintf(s, args...),
	}
}
