package containers_common

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	URL_UNIQUENESS_IDENT  = core.Identifier("url")
	REPR_UNIQUENESS_IDENT = core.Identifier("repr")
)

var (
	ErrFailedGetUniqueKeyNoURL       = errors.New("failed to get unique key for value since it has no URL")
	ErrFailedGetUniqueKeyNoProps     = errors.New("failed to get unique key for value since it has no properties")
	ErrFailedGetUniqueKeyPropMissing = errors.New("failed to get unique key for value since the property is missing")

	UniqueKeyReprConfig = &core.ReprConfig{AllVisible: true}

	URL_UNIQUENESS_SYMB_IDENT  = symbolic.NewIdentifier("url")
	REPR_UNIQUENESS_SYMB_IDENT = symbolic.NewIdentifier("repr")
)

type UniquenessConstraint struct {
	Type         UniquenessConstraintType
	PropertyName core.PropertyName //set if UniquePropertyValue
}

func UniquenessConstraintFromValue(val core.Value) (UniquenessConstraint, bool) {
	var uniqueness UniquenessConstraint
	switch u := val.(type) {
	case core.Identifier:
		switch u {
		case URL_UNIQUENESS_IDENT:
			uniqueness.Type = UniqueURL
		case REPR_UNIQUENESS_IDENT:
			uniqueness.Type = UniqueRepr
		default:
			return UniquenessConstraint{}, false
		}
	case core.PropertyName:
		uniqueness.Type = UniquePropertyValue
		uniqueness.PropertyName = u
	default:
		return UniquenessConstraint{}, false
	}

	return uniqueness, true
}

func UniquenessConstraintFromSymbolicValue(val symbolic.SymbolicValue) (UniquenessConstraint, bool) {
	switch val := val.(type) {
	case *symbolic.PropertyName:
		if val.Name() == "" {
			return UniquenessConstraint{}, false
		}
		return UniquenessConstraint{Type: UniquePropertyValue, PropertyName: core.PropertyName(val.Name())}, true
	case *symbolic.Identifier:
		switch val.Name() {
		case "url":
			return UniquenessConstraint{Type: UniqueURL}, true
		case "repr":
			return UniquenessConstraint{Type: UniqueRepr}, true
		}
	}
	return UniquenessConstraint{}, false
}

func (c UniquenessConstraint) ToValue() core.Serializable {
	switch c.Type {
	case UniqueRepr:
		return REPR_UNIQUENESS_IDENT
	case UniqueURL:
		return URL_UNIQUENESS_IDENT
	case UniquePropertyValue:
		return c.PropertyName
	default:
		panic(core.ErrUnreachable)
	}
}

func (c UniquenessConstraint) ToSymbolicValue() symbolic.SymbolicValue {
	switch c.Type {
	case UniqueRepr:
		return REPR_UNIQUENESS_SYMB_IDENT
	case UniqueURL:
		return URL_UNIQUENESS_SYMB_IDENT
	case UniquePropertyValue:
		return symbolic.NewPropertyName(string(c.PropertyName))
	default:
		panic(core.ErrUnreachable)
	}
}

func (c UniquenessConstraint) Equal(otherConstraint UniquenessConstraint) bool {
	if c.Type != otherConstraint.Type {
		return false
	}

	//TODO: check Repr config

	if c.Type == UniquePropertyValue && c.PropertyName != otherConstraint.PropertyName {
		return false
	}

	return true
}

type UniquenessConstraintType int

const (
	UniqueRepr UniquenessConstraintType = iota + 1
	UniqueURL
	UniquePropertyValue
)

func GetUniqueKey(ctx *core.Context, v core.Serializable, config UniquenessConstraint) string {
	var key string
	switch config.Type {
	case UniqueRepr:
		if v.IsMutable() {
			panic(core.ErrReprOfMutableValueCanChange)
		}
		// representation is context-dependent -> possible issues
		key = string(core.MustGetRepresentationWithConfig(v, UniqueKeyReprConfig, ctx))
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
		repr := core.MustGetRepresentationWithConfig(propVal.(core.Serializable), UniqueKeyReprConfig, ctx)
		key = string(repr)
	}
	return key
}
