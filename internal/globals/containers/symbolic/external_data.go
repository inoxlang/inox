package containers

import (
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/globals/containers/common"
)

var (
	externalData ExternalData
)

type ExternalData struct {
	CreateConcreteSetPattern    func(concreteCtw symbolic.ConcreteContext, uniqueness common.UniquenessConstraint, elementPattern any) any
	CreateConcreteMapPattern    func(concreteCtw symbolic.ConcreteContext, keyPattern any, valuePattern any) any
	CreateConcreteThreadPattern func(concreteCtw symbolic.ConcreteContext, elementPattern any) any
}

func SetExternalData(data ExternalData) {
	externalData = data
}
