package core

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/utils"
)

func ToJSON(ctx *Context, v Serializable) Str {
	return ToJSONWithConfig(ctx, v, JSONSerializationConfig{
		ReprConfig: &ReprConfig{},
		Pattern:    ANYVAL_PATTERN,
		Location:   "/",
	})
}

func ToJSONWithConfig(ctx *Context, v Serializable, config JSONSerializationConfig) Str {
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)
	if err := v.WriteJSONRepresentation(ctx, stream, config, 0); err != nil {
		panic(err)
	}
	if stream.Error != nil {
		panic(stream.Error)
	}
	return Str(stream.Buffer())
}

func ToPrettyJSON(ctx *Context, v Serializable) Str {
	s := ToJSON(ctx, v)
	var unmarshalled interface{}
	json.Unmarshal([]byte(s), &unmarshalled)
	b, err := utils.MarshalIndentJsonNoHTMLEspace(unmarshalled, "", " ")

	if err != nil {
		log.Panicln("tojson:", err)
	}
	return Str(b)
}

func ToJSONVal(ctx *Context, v Serializable) interface{} {
	s := ToJSON(ctx, v)
	var jsonVal interface{}
	err := json.Unmarshal([]byte(s), &jsonVal)
	if err != nil {
		log.Panicln("from json:", err)
	}

	return jsonVal
}

func ConvertJSONValToInoxVal(v any, immutable bool) Serializable {
	switch val := v.(type) {
	case nil:
		return Nil
	case map[string]any:
		if immutable {
			valMap := ValMap{}
			for key, value := range val {
				valMap[key] = ConvertJSONValToInoxVal(value, immutable)
			}
			return NewRecordFromMap(valMap)
		} else {
			valMap := ValMap{}
			for key, value := range val {
				valMap[key] = ConvertJSONValToInoxVal(value, immutable)
			}
			return NewObjectFromMapNoInit(valMap)
		}
	case []any:
		l := make([]Serializable, len(val))
		for i, e := range val {
			l[i] = ConvertJSONValToInoxVal(e, immutable)
		}
		if immutable {
			return NewTuple(l)
		}
		return &List{underlyingList: &ValueList{elements: l}}
	case int:
		return Int(val)
	case float64:
		return Float(val)
	case json.Number:
		if strings.Contains(val.String(), ".") {
			float, err := val.Float64()
			if err != nil {
				panic(fmt.Errorf("failed to parse float `%s`: %w", val.String(), err))
			}
			return Float(float)
		}
		integer, err := val.Int64()
		if err != nil {
			panic(fmt.Errorf("failed to parse integer `%s`: %w", val.String(), err))
		}
		return Int(integer)
	case bool:
		return Bool(val)
	case string:
		return Str(val)
	default:
		panic(fmt.Errorf("cannot convert value of type %T to Inox Value", val))
	}
}

func parseJson(ctx *Context, v any) (any, error) {
	var b []byte

	switch val := v.(type) {
	case WrappedBytes:
		b = val.UnderlyingBytes()
	case WrappedString:
		b = []byte(val.UnderlyingString())
	default:
		return "", fmt.Errorf("cannot parse non string|bytes: %T", val)
	}

	var result interface{}
	if err := json.Unmarshal(b, &result); err != nil {
		return nil, err
	}

	return ConvertJSONValToInoxVal(result, false), nil
}
