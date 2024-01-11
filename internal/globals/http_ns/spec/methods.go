package spec

import (
	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	METHODS_WITH_NO_BODY = []string{"GET", "HEAD", "OPTIONS"}
	METHODS              = []string{"GET", "HEAD", "OPTIONS", "PUT", "POST", "PATCH", "DELETE"}
	FS_ROUTING_METHODS   = []string{"GET", "OPTIONS", "POST", "PATCH", "PUT", "DELETE"}

	METHOD_PATTERN = core.NewUnionPattern(utils.MapSlice(METHODS, func(s string) core.Pattern {
		return core.NewExactValuePattern(core.Identifier(s))
	}), nil)
)
