package staticcheck

import (
	"github.com/inoxlang/inox/internal/core/inoxmod"
)

type Scheme interface {
}

type Host interface {
	inoxmod.ResourceName
	Name() string
}

type URL interface {
	inoxmod.ResourceName
	HasQueryOrFragment() bool
}
