package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	jsoniter "github.com/inoxlang/inox/internal/jsoniter"
	"github.com/inoxlang/inox/internal/utils"
)

func ToJSON(ctx *Context, v Serializable, pattern *OptionalParam[Pattern]) String {
	var patt Pattern
	if pattern == nil {
		patt = ANYVAL_PATTERN
	} else {
		patt = pattern.Value
	}

	return ToJSONWithConfig(ctx, v, JSONSerializationConfig{
		ReprConfig: &ReprConfig{},
		Pattern:    patt,
		Location:   "/",
	})
}

func ToJSONWithConfig(ctx *Context, v Serializable, config JSONSerializationConfig) String {
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)
	if err := v.WriteJSONRepresentation(ctx, stream, config, 0); err != nil {
		panic(err)
	}
	if stream.Error != nil {
		panic(stream.Error)
	}
	return String(stream.Buffer())
}

func ToPrettyJSON(ctx *Context, v Serializable, pattern *OptionalParam[Pattern]) String {
	s := ToJSON(ctx, v, pattern)
	var unmarshalled interface{}
	err := json.Unmarshal([]byte(s), &unmarshalled)
	if err != nil {
		panic(errors.New("failed to serialize value to JSON"))
	}
	b, err := utils.MarshalIndentJsonNoHTMLEspace(unmarshalled, "", " ")

	if err != nil {
		panic(err)
	}
	return String(b)
}

func ToJSONVal(ctx *Context, v Serializable) interface{} {
	s := ToJSON(ctx, v, nil)
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
		return String(val)
	default:
		panic(fmt.Errorf("cannot convert value of type %T to Inox Value", val))
	}
}

func parseJson(ctx *Context, v any) (any, error) {
	var b []byte

	switch val := v.(type) {
	case GoBytes:
		b = val.UnderlyingBytes()
	case GoString:
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

func AsJSON(ctx *Context, v Serializable) String {
	stream := jsoniter.NewStream(jsoniter.ConfigDefault, nil, 0)
	asJSON(ctx, v, stream)
	return String(stream.Buffer())
}

func asJSON(ctx *Context, v Serializable, w *jsoniter.Stream) {
	switch v := v.(type) {
	case *Object:
		w.WriteObjectStart()

		i := 0
		v.ForEachEntry(func(k string, propVal Serializable) error {
			if i != 0 {
				w.WriteMore()
			}
			i++
			w.WriteObjectField(k)
			asJSON(ctx, propVal, w)
			return nil
		})

		w.WriteObjectEnd()
	case *List:
		w.WriteArrayStart()

		for i := 0; i < v.Len(); i++ {
			if i != 0 {
				w.WriteMore()
			}
			asJSON(ctx, v.At(ctx, i).(Serializable), w)
		}

		w.WriteArrayEnd()
	case StringLike:
		w.WriteString(v.GetOrBuildString())
	case Int:
		w.WriteInt64(int64(v.Int64()))
	case Float:
		w.WriteFloat64(float64(v))
	case Bool:
		w.WriteBool(bool(v))
	case NilT:
		w.WriteNil()
	default:
		panic(fmt.Errorf("unexpected value %s, `asjson` only supports objects, lists, integers, floats, bools and string-likes", Stringify(v, ctx)))
	}
}
