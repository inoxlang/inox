package hsparse

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dop251/goja"
	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/utils"
)

const (
	MAX_SOURCE_CODE_LENGTH_IF_SLOW_PARSE = 1000
	HYPERSCRIPT_PARSING_FUNCTION_NAME    = "parseHyperScript"
)

var (
	//go:embed parse-hyperscript.js
	HYPERSCRIPT_PARSER_JS      string
	HYPERSCRIPT_PARSER_PROGRAM *goja.Program
	HYPERSCRIPT_PARSER_VM      *goja.Runtime

	ErrInputStringTooLong = errors.New("input string is too long")
	ErrNotParsed          = errors.New("input string is too long")
)

func init() {
	HYPERSCRIPT_PARSER_PROGRAM = goja.MustCompile("parse-hyperscript.js", HYPERSCRIPT_PARSER_JS, false)

	HYPERSCRIPT_PARSER_VM = goja.New()
	utils.Must(HYPERSCRIPT_PARSER_VM.RunProgram(HYPERSCRIPT_PARSER_PROGRAM))
}

// parseHyperScriptSlow uses the original parser written in JS to parse HyperScript code.
func parseHyperScriptSlow(ctx context.Context, source string) (*hscode.ParsingResult, *hscode.ParsingError, error) {

	if len(source) > MAX_SOURCE_CODE_LENGTH_IF_SLOW_PARSE {
		return &hscode.ParsingResult{NodeData: map[string]any{}}, nil, nil
	}

	if len(source) > DEFAULT_MAX_SOURCE_CODE_LENGTH {
		return nil, nil, ErrInputStringTooLong
	}

	runtime := HYPERSCRIPT_PARSER_VM
	input := runtime.ToValue(source)
	global := runtime.GlobalObject()
	global.Set("input", input)

	callResult, err := runtime.RunString(HYPERSCRIPT_PARSING_FUNCTION_NAME + `()`)
	if err != nil {
		return nil, nil, err
	}

	object := callResult.ToObject(runtime)

	criticalError := object.Get("criticalError")
	if criticalError != nil {
		return nil, nil, errors.New(criticalError.Export().(string))
	}

	errorJSON := object.Get("errorJSON")
	if errorJSON != nil {
		_json := errorJSON.Export().(string)
		var err hscode.ParsingError
		unmarshallingErr := json.Unmarshal([]byte(_json), &err)
		if unmarshallingErr != nil {
			return nil, nil, fmt.Errorf("internal error: %w", unmarshallingErr)
		}

		return nil, &err, nil
	}

	outputJSON := object.Get("outputJSON")
	if outputJSON != nil {
		_json := outputJSON.Export().(string)
		var parsingResult hscode.ParsingResult

		unmarshallingErr := json.Unmarshal([]byte(_json), &parsingResult)
		if unmarshallingErr != nil {
			return nil, nil, fmt.Errorf("internal error: %w", unmarshallingErr)
		}

		parsingResult.TokensNoWhitespace = utils.FilterSlice(parsingResult.Tokens, isNotWhitespaceToken)

		return &parsingResult, nil, nil
	}

	return nil, nil, nil
}

func isNotWhitespaceToken(e hscode.Token) bool {
	return e.Type != "WHITESPACE"
}
