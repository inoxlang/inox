package parse

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dop251/goja"
)

var (
	//go:embed parse-hyperscript.js
	HYPERSCRIPT_PARSER_JS      string
	HYPERSCRIPT_PARSER_PROGRAM *goja.Program
)

func init() {
	HYPERSCRIPT_PARSER_PROGRAM = goja.MustCompile("parse-hyperscript.js", HYPERSCRIPT_PARSER_JS, false)
}

type ParsingError struct {
	Message        string  `json:"message"`
	MessageAtToken string  `json:"messageAtToken"`
	Token          Token   `json:"token"`
	Tokens         []Token `json:"tokens"`
}

func (e ParsingError) Error() string {
	return e.Message
}

func parseHyperscript(source string) (_ any, parsingErr error, criticalErr error) {
	runtime := goja.New()

	input := runtime.ToValue(source)

	global := runtime.GlobalObject()
	global.Set("input", input)

	_, err := runtime.RunProgram(HYPERSCRIPT_PARSER_PROGRAM)
	if err != nil {
		return nil, nil, err
	}

	criticalError := global.Get("criticalError")
	if criticalError != nil {
		return nil, nil, errors.New(criticalError.Export().(string))
	}

	errorJSON := global.Get("errorJSON")
	if errorJSON != nil {
		_json := errorJSON.Export().(string)
		var err ParsingError
		unmarshallingErr := json.Unmarshal([]byte(_json), &err)
		if unmarshallingErr != nil {
			return nil, nil, fmt.Errorf("internal error: %w", unmarshallingErr)
		}

		return nil, &err, nil
	}

	return nil, nil, nil
}
