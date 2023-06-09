package internal

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	ErrFailedGetUniqueKeyNoURL       = errors.New("failed to get unique key for value since it has no URL")
	ErrFailedGetUniqueKeyNoProps     = errors.New("failed to get unique key for value since it has no properties")
	ErrFailedGetUniqueKeyPropMissing = errors.New("failed to get unique key for value since the property is missing")
)

type UniquenessConstraint struct {
	Type         UniquenessConstraintType
	PropertyName core.PropertyName //set if UniquePropertyValue
	Repr         *core.ReprConfig  //set if UniqueRepr
}

type UniquenessConstraintType int

const (
	UniqueRepr UniquenessConstraintType = iota + 1
	UniqueURL
	UniquePropertyValue
)

func getUniqueKey(ctx *core.Context, v core.Value, config UniquenessConstraint) string {
	var key string
	switch config.Type {
	case UniqueRepr:
		// representation is context-dependent -> possible issues
		key = string(core.MustGetRepresentationWithConfig(v, config.Repr, ctx))
	case UniqueURL:
		url, err := core.UrlOf(ctx, v)
		if err != nil {
			panic(ErrFailedGetUniqueKeyNoURL)
		}
		key = url.UnderlyingString()
	case UniquePropertyValue:
		iprops, ok := v.(core.IProps)
		if !ok {
			panic(ErrFailedGetUniqueKeyNoProps)
		}
		propNames := iprops.PropertyNames(ctx)
		if !utils.SliceContains(propNames, string(config.PropertyName)) {
			panic(fmt.Errorf("%w: %s", ErrFailedGetUniqueKeyPropMissing, config.PropertyName))
		}
		//ToC / Tos ??
		propVal := iprops.Prop(ctx, config.PropertyName.UnderlyingString())
		repr := core.MustGetRepresentationWithConfig(propVal, config.Repr, ctx)
		key = string(repr)
	}
	return key
}
