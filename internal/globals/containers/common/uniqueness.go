package common

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"

	"github.com/inoxlang/inox/internal/core"
	"github.com/inoxlang/inox/internal/core/symbolic"
	"github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
)

const (
	URL_UNIQUENESS_IDENT          = core.Identifier("url")
	REPR_UNIQUENESS_IDENT         = core.Identifier("repr")
	TRANSIENT_ID_UNIQUENESS_IDENT = core.Identifier("transient-id")
)

var (
	ErrFailedGetUniqueKeyNoURL                                  = errors.New("failed to get unique key for value since it has no URL")
	ErrFailedGetUniqueKeyNoProps                                = errors.New("failed to get unique key for value since it has no properties")
	ErrFailedGetUniqueKeyPropMissing                            = errors.New("failed to get unique key for value since the property is missing")
	ErrPropertyBasedUniquenessRequireValuesToHaveTheProperty    = errors.New("property-based uniqueness requires values to have the property")
	ErrReprBasedUniquenessRequireValuesToBeImmutable            = errors.New("representation-based uniqueness requires values to be immutable")
	ErrTransientIDBasedUniquenessRequireValuesToHaveTransientID = errors.New("transient id-based uniqueness requires values to have a transient id")
	ErrUrlBasedUniquenessRequireValuesToBeUrlHolders            = errors.New("URL-based uniqueness requires values to be URL holders")
	ErrContainerShouldHaveURL                                   = errors.New("container should have a URL")

	UniqueKeyReprConfig = &core.ReprConfig{AllVisible: true}

	URL_UNIQUENESS_SYMB_IDENT          = symbolic.NewIdentifier(URL_UNIQUENESS_IDENT.UnderlyingString())
	REPR_UNIQUENESS_SYMB_IDENT         = symbolic.NewIdentifier(REPR_UNIQUENESS_IDENT.UnderlyingString())
	TRANSIENT_ID_UNIQUENESS_SYMB_IDENT = symbolic.NewIdentifier(TRANSIENT_ID_UNIQUENESS_IDENT.UnderlyingString())

	EXPECTED_SYMB_VALUE_FOR_UNIQUENESS = fmt.Sprintf("#%s, #%s, #%s or a property name is expected",
		URL_UNIQUENESS_IDENT,
		REPR_UNIQUENESS_IDENT,
		TRANSIENT_ID_UNIQUENESS_IDENT,
	)
)

type UniquenessConstraint struct {
	Type         UniquenessConstraintType
	PropertyName core.PropertyName //set if UniquePropertyValue
}

func NewReprUniqueness() *UniquenessConstraint {
	return &UniquenessConstraint{
		Type: UniqueRepr,
	}
}

func NewTransientIdUniqueness() *UniquenessConstraint {
	return &UniquenessConstraint{
		Type: UniqueTransientID,
	}
}

func NewURLUniqueness() *UniquenessConstraint {
	return &UniquenessConstraint{
		Type: UniqueURL,
	}
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
		case TRANSIENT_ID_UNIQUENESS_IDENT:
			uniqueness.Type = UniqueTransientID
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

func UniquenessConstraintFromSymbolicValue(val symbolic.Value, elementPattern symbolic.Pattern) (UniquenessConstraint, error) {
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
		if !val.HasConcreteName() {
			return UniquenessConstraint{}, errors.New(EXPECTED_SYMB_VALUE_FOR_UNIQUENESS)
		}

		switch val.Name() {
		case string(URL_UNIQUENESS_IDENT):
			_, ok := elem.(symbolic.UrlHolder)
			if !ok {
				return UniquenessConstraint{}, ErrUrlBasedUniquenessRequireValuesToBeUrlHolders
			}

			return UniquenessConstraint{Type: UniqueURL}, nil
		case string(REPR_UNIQUENESS_IDENT):
			if elementPattern.SymbolicValue().IsMutable() {
				return UniquenessConstraint{}, ErrReprBasedUniquenessRequireValuesToBeImmutable
			}
			return UniquenessConstraint{Type: UniqueRepr}, nil

		case string(TRANSIENT_ID_UNIQUENESS_IDENT):
			return UniquenessConstraint{Type: UniqueTransientID}, nil
		default:
			return UniquenessConstraint{}, errors.New(EXPECTED_SYMB_VALUE_FOR_UNIQUENESS)
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
	case UniqueTransientID:
		return TRANSIENT_ID_UNIQUENESS_IDENT
	case UniquePropertyValue:
		return c.PropertyName
	default:
		panic(core.ErrUnreachable)
	}
}

func (c UniquenessConstraint) ToSymbolicValue() symbolic.Value {
	switch c.Type {
	case UniqueTransientID:
		return TRANSIENT_ID_UNIQUENESS_SYMB_IDENT
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
	UniqueTransientID
)

type KeyRetrievalParams struct {
	Value                   core.Serializable
	Config                  UniquenessConstraint
	Container               core.Value
	Stream                  *jsoniter.Stream
	JSONSerializationConfig core.JSONSerializationConfig
}

// GetUniqueKey computes the key of the provided value. For UniqueRepr and UniquePropertyValue uniqueness
// the key is written to the provided stream, so the returned string should be cloned before being stored.
// GetUniqueKey does not support retrieving the key for UniqueAddress uniqueness.
func GetUniqueKey(ctx *core.Context, args KeyRetrievalParams) string {
	config := args.Config
	container := args.Container
	v := args.Value

	switch config.Type {
	case UniqueTransientID:
		panic(errors.New("transient id based uniqueness does not use keys"))
	case UniqueRepr:
		if v.IsMutable() {
			panic(core.ErrReprOfMutableValueCanChange)
		}

		bufLen := len(args.Stream.Buffer())

		// representation is context-dependent -> possible issues
		err := v.WriteJSONRepresentation(ctx, args.Stream, args.JSONSerializationConfig, 0)
		if err != nil {
			panic(err)
		}

		return utils.BytesAsString(args.Stream.Buffer()[bufLen:])
	case UniqueURL:
		url, err := core.UrlOf(ctx, v)
		if err != nil {
			panic(ErrFailedGetUniqueKeyNoURL)
		}

		containerURL, ok := container.(core.UrlHolder).URL()
		if !ok {
			panic(core.ErrUnreachable)
		}

		elementURL := url.UnderlyingString()
		key, found := strings.CutPrefix(elementURL, string(containerURL))
		if !found {
			panic(core.ErrUnreachable)
		}
		if key[0] == '/' {
			key = key[1:]
		}
		_, err = ulid.ParseStrict(key)
		if err != nil {
			panic(core.ErrUnreachable)
		}
		return key
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

		bufLen := len(args.Stream.Buffer())
		// representation is context-dependent -> possible issues
		err := propVal.(core.Serializable).WriteJSONRepresentation(ctx, args.Stream, args.JSONSerializationConfig, 0)
		if err != nil {
			panic(err)
		}

		return utils.BytesAsString(args.Stream.Buffer()[bufLen:])
	default:
		panic(core.ErrUnreachable)
	}
}

func GetElementPathKeyFromKey(key string, uniqueness UniquenessConstraintType) core.ElementKey {
	if uniqueness == UniqueURL {
		return core.MustElementKeyFrom(key) //ULID
	}
	hash := sha256.Sum256(utils.StringAsBytes(key))
	return core.MustElementKeyFrom(core.ElementKeyEncoding.EncodeToString(hash[:]))
}
