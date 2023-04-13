package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"

	"github.com/buger/jsonparser"
)

func ToJSON(ctx *Context, v Value) Str {
	return ToJSONWithConfig(ctx, v, &ReprConfig{})
}

func ToJSONWithConfig(ctx *Context, v Value, config *ReprConfig) Str {
	if v.HasJSONRepresentation(map[uintptr]int{}, config) {
		var buff bytes.Buffer
		if err := v.WriteJSONRepresentation(ctx, &buff, map[uintptr]int{}, config); err != nil {
			panic(err)
		}
		return Str(buff.String())
	}
	panic(ErrNoRepresentation)
}

func ToPrettyJSON(ctx *Context, v Value) Str {
	s := ToJSON(ctx, v)
	var unmarshalled interface{}
	json.Unmarshal([]byte(s), &unmarshalled)
	b, err := MarshalIndentJsonNoHTMLEspace(unmarshalled, "", " ")

	if err != nil {
		log.Panicln("tojson:", err)
	}
	return Str(b)
}

func ToJSONVal(ctx *Context, v Value) interface{} {

	s := ToJSON(ctx, v)
	var jsonVal interface{}
	err := json.Unmarshal([]byte(s), &jsonVal)
	if err != nil {
		log.Panicln("from json:", err)
	}

	return jsonVal
}

func ConvertJSONValToInoxVal(ctx *Context, v any, immutable bool) Value {
	switch val := v.(type) {
	case map[string]any:
		if immutable {
			valMap := ValMap{}
			for key, value := range val {
				valMap[key] = ConvertJSONValToInoxVal(ctx, value, immutable)
			}
			return NewRecordFromMap(valMap)
		} else {
			object := &Object{}
			for key, value := range val {
				object.SetProp(ctx, key, ConvertJSONValToInoxVal(ctx, value, immutable))
			}
			return object
		}
	case []any:
		l := make([]Value, len(val))
		for i, e := range val {
			l[i] = ConvertJSONValToInoxVal(ctx, e, immutable)
		}
		if immutable {
			return NewTuple(l)
		}
		return &List{underylingList: &ValueList{elements: l}}
	case int:
		return Int(val)
	case float64:
		return Float(val)
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

	return ConvertJSONValToInoxVal(ctx, result, false), nil
}

func MarshalJsonNoHTMLEspace(v any) ([]byte, error) {
	return marshalJsonNoHTMLEspace(v)
}

func marshalJsonNoHTMLEspace(v any, encoderOptions ...func(encoder *json.Encoder)) ([]byte, error) {
	//create encoder

	buf := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	for _, opt := range encoderOptions {
		opt(encoder)
	}

	//encode

	err := encoder.Encode(v)

	if err != nil {
		return nil, err
	}
	bytes := buf.Bytes()
	//remove newline
	return bytes[:len(bytes)-1], nil
}

func MarshalIndentJsonNoHTMLEspace(v any, prefix, indent string) ([]byte, error) {
	return marshalJsonNoHTMLEspace(v, func(encoder *json.Encoder) {
		encoder.SetIndent(prefix, indent)
	})
}

//

func NewMutationFromJSON(data []byte) (Mutation, error) {
	var mutation Mutation

	err := jsonparser.ObjectEach(data, func(key, value []byte, dataType jsonparser.ValueType, offset int) error {
		if len(key) == 0 {
			return ErrEmptyMutationPrefixSymbol
		}

		panic("!")
	})

	if err != nil {
		return Mutation{}, fmt.Errorf("failed to create mutation from json: %w", err)
	}

	return mutation, nil
}
