package containers

import containers_common "github.com/inoxlang/inox/internal/globals/containers/common"

var (
	externalData ExternalData
)

type ExternalData struct {
	CreateConcreteSetPattern func(uniqueness containers_common.UniquenessConstraint, elementPattern any) any
}

func SetExternalData(data ExternalData) {
	externalData = data
}
