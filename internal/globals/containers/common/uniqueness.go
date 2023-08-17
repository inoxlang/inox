package containers_common

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
)

const (
	URL_UNIQUENESS_IDENT  = core.Identifier("url")
	REPR_UNIQUENESS_IDENT = core.Identifier("repr")
)

var (
	ErrFailedGetUniqueKeyNoURL                               = errors.New("failed to get unique key for value since it has no URL")
	ErrFailedGetUniqueKeyNoProps                             = errors.New("failed to get unique key for value since it has no properties")
	ErrFailedGetUniqueKeyPropMissing                         = errors.New("failed to get unique key for value since the property is missing")
	ErrPropertyBasedUniquenessRequireValuesToHaveTheProperty = errors.New("property-based uniqueness requires values to have the property")
	ErrReprBasedUniquenessRequireValuesToBeImmutable         = errors.New("representation-based uniqueness requires values to be immutable")
	ErrUrlBasedUniquenessRequireValuesToBeUrlHolders         = errors.New("URL-based uniqueness requires values to be URL holders")
	ErrContainerShouldHaveURL                                = errors.New("container should have a URL")

	UniqueKeyReprConfig = &core.ReprConfig{AllVisible: true}

	URL_UNIQUENESS_SYMB_IDENT  = symbolic.NewIdentifier(URL_UNIQUENESS_IDENT.UnderlyingString())
	REPR_UNIQUENESS_SYMB_IDENT = symbolic.NewIdentifier(REPR_UNIQUENESS_IDENT.UnderlyingString())

	EXPECTED_SYMB_VALUE_FOR_UNIQUENESS = "#url, #repr or a property name is expected"
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

func UniquenessConstraintFromSymbolicValue(val symbolic.SymbolicValue, elementPattern symbolic.Pattern) (UniquenessConstraint, error) {
	elem := elementPattern.SymbolicValue()
	switch val := val.(type) {
	case *symbolic.PropertyName:
		propertyName := val.Name()
		if propertyName == "" {
			return UniquenessConstraint{}, errors.New(EXPECTED_SYMB_VALUE_FOR_UNIQUENESS)
		}
		iprops, ok := symbolic.AsIprops(elem).(symbolic.IProps)
		if !ok || !symbolic.HasRequiredOrOptionalProperty(iprops, propertyName) || symbolic.IsPropertyOptional(iprops, propertyName) {
			return UniquenessConstraint{}, ErrPropertyBasedUniquenessRequireValuesToHaveTheProperty
		}

		return UniquenessConstraint{
			Type:         UniquePropertyValue,
			PropertyName: core.PropertyName(propertyName),
		}, nil
	case *symbolic.Identifier:
		if !val.HasConcreteName() || (val.Name() != "url" && val.Name() != "repr") {
			return UniquenessConstraint{}, errors.New(EXPECTED_SYMB_VALUE_FOR_UNIQUENESS)
		}

		switch val.Name() {
		case URL_UNIQUENESS_IDENT.UnderlyingString():
			_, ok := elem.(symbolic.UrlHolder)
			if !ok {
				return UniquenessConstraint{}, ErrUrlBasedUniquenessRequireValuesToBeUrlHolders
			}

			return UniquenessConstraint{Type: UniqueURL}, nil
		case REPR_UNIQUENESS_IDENT.UnderlyingString():
			if elementPattern.SymbolicValue().IsMutable() {
				return UniquenessConstraint{}, ErrReprBasedUniquenessRequireValuesToBeImmutable
			}
			return UniquenessConstraint{Type: UniqueRepr}, nil
		}
	}
	return UniquenessConstraint{}, errors.New(EXPECTED_SYMB_VALUE_FOR_UNIQUENESS)
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

func (c UniquenessConstraint) AddUrlIfNecessary(ctx *core.Context, container core.UrlHolder, element core.Value) {
	if c.Type == UniqueURL {
		holder, ok := element.(core.UrlHolder)
		if !ok {
			panic(errors.New("elements should be URL holders"))
		}

		_, ok = holder.URL()
		if ok { //element already has a URL

			return
		}
		containerURL, ok := container.URL()
		if !ok {
			panic(ErrContainerShouldHaveURL)
		}

		url := containerURL.ToDirURL().AppendAbsolutePath(core.Path("/" + ulid.Make().String()))
		utils.PanicIfErr(holder.SetURLOnce(ctx, url))
	}
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
