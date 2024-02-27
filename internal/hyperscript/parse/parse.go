package parse

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dop251/goja"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	//go:embed parse-hyperscript.js
	HYPERSCRIPT_PARSER_JS      string
	HYPERSCRIPT_PARSER_PROGRAM *goja.Program
)

func init() {
	HYPERSCRIPT_PARSER_PROGRAM = goja.MustCompile("parse-hyperscript.js", HYPERSCRIPT_PARSER_JS, false)
}

type ParsingResult struct {
	Node               any     `json:"node"`
	Tokens             []Token `json:"tokens"`
	TokensNoWhitespace []Token `json:"tokensNoWhitespace"`
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

func parseHyperscript(source string) (result *ParsingResult, parsingErr error, criticalErr error) {
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

	outputJSON := global.Get("outputJSON")
	if outputJSON != nil {
		_json := outputJSON.Export().(string)
		var parsingResult ParsingResult

		unmarshallingErr := json.Unmarshal([]byte(_json), &parsingResult)
		if unmarshallingErr != nil {
			return nil, nil, fmt.Errorf("internal error: %w", unmarshallingErr)
		}

		parsingResult.TokensNoWhitespace = utils.FilterSlice(parsingResult.Tokens, func(e Token) bool {
			return e.Type != "WHITESPACE"
		})

		return &parsingResult, nil, nil
	}

	return nil, nil, nil
}
