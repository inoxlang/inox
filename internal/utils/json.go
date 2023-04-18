package utils

import (
	"bytes"
	"encoding/json"
)

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
