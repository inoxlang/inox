package internal

import (
	"errors"
	"reflect"
	"strconv"

	parse "github.com/inoxlang/inox/internal/parse"
)

const (
	URL_METADATA_KEY  = "_url_"
	MIME_METADATA_KEY = "_mime_"
)

var (
	ErrValueNoURL = errors.New("value has not an URL")
	ErrValueNoId  = errors.New("value has not an identifier")
)

func UrlOf(ctx *Context, v Value) (URL, error) {
	switch val := v.(type) {
	case *Object:
		if val.url != "" {
			return val.url, nil
		}
	}
	return "", ErrValueNoURL
}

func IdOf(ctx *Context, v Value) Identifier {
	u, err := UrlOf(ctx, v)
	if err == nil {
		return Identifier("&" + Str(u))
	}

	if IsSimpleInoxVal(v) {
		return Identifier(GetRepresentation(v, ctx))
	}

	rval := reflect.ValueOf(v)
	switch rval.Kind() {
	case reflect.Pointer, reflect.Map:
		return Identifier("&" + strconv.FormatUint(uint64(rval.Pointer()), 16))
	case reflect.Slice:
		ptr := strconv.FormatUint(uint64(rval.Pointer()), 16)
		length := strconv.FormatUint(uint64(rval.Len()), 16)
		return Identifier("&" + ptr + "-" + length)
	}

	panic(ErrValueNoId)
}

func initializeMetaproperties(v Value, props []*parse.ObjectMetaProperty) {
	obj, ok := v.(*Object)

	if !ok {
		panic(errors.New("metaproperty init: only objects are supported"))
	}

	for _, prop := range props {
		switch prop.Name() {
		case CONSTRAINTS_KEY:
			initializeConstraintMetaproperty(obj, prop.Initialization)
		case VISIBILITY_KEY:
			initializeVisibilityMetaproperty(obj, prop.Initialization)
		}
	}
}
