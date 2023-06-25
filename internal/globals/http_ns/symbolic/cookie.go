package http_ns

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"

	"github.com/inoxlang/inox/internal/utils"
)

var optionalHostPattern = symbolic.NewOptionalPattern(
	utils.Must(core.HOST_PATTERN.ToSymbolicValue(nil, nil)).(symbolic.Pattern),
)

func NewCookieObject() *symbolic.Object {
	obj := symbolic.NewObject(map[string]symbolic.SymbolicValue{
		"name":   symbolic.ANY_STR,
		"value":  symbolic.ANY_STR,
		"domain": symbolic.Nil,
	}, nil, map[string]symbolic.Pattern{
		"domain": optionalHostPattern,
	})

	return obj
}
