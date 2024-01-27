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
	obj := symbolic.NewInexactObject(map[string]symbolic.Serializable{
		"name":   symbolic.ANY_STRING,
		"value":  symbolic.ANY_STRING,
		"domain": symbolic.Nil,
	}, nil, map[string]symbolic.Pattern{
		"domain": optionalHostPattern,
	})

	return obj
}
