package containers

import "github.com/inoxlang/inox/internal/globals/containers/common"

var (
	externalData ExternalData
)

type ExternalData struct {
	CreateConcreteSetPattern    func(uniqueness common.UniquenessConstraint, elementPattern any) any
	CreateConcreteMapPattern    func(keyPattern any, valuePattern any) any
	CreateConcreteThreadPattern func(elementPattern any) any
}

func SetExternalData(data ExternalData) {
	externalData = data
}
