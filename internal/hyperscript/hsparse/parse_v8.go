package hsparse

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/tommie/v8go"
)

const (
	MAX_SOURCE_CODE_LENGTH = 100_000
	TIMEOUT                = 200 * time.Millisecond
)

var (
	//go:embed parse-hyperscript.js
	HYPERSCRIPT_PARSER_JS string
	vmPool                = make(chan *jsVM, 10)

	errInternal = errors.New("internal error")
)

type jsVM struct {
	ctx           *v8go.Context
	parseFunction *v8go.Function
}

func ParseHyperScriptProgram(ctx context.Context, source string) (parsingResult *hscode.ParsingResult, parsingErr *hscode.ParsingError, criticalError error) {
	return _parseHyperScript(ctx, source, false)
}

func ParseHyperScriptExpression(ctx context.Context, source string) (parsingResult *hscode.ParsingResult, parsingErr *hscode.ParsingError, criticalError error) {
	return _parseHyperScript(ctx, source, true)
}

func _parseHyperScript(ctx context.Context, source string, parseExpr bool) (parsingResult *hscode.ParsingResult, parsingErr *hscode.ParsingError, criticalError error) {

	if len(source) > MAX_SOURCE_CODE_LENGTH {
		return nil, nil, errors.New("source code is too long")
	}

	var vm *jsVM
	select {
	case vm = <-vmPool:
	default:
		//Create a new VM and declare the functions and classes.
		vm = &jsVM{
			ctx: v8go.NewContext(),
		}
		_, err := vm.ctx.RunScript(HYPERSCRIPT_PARSER_JS, "definitions.js")
		if err != nil {
			return nil, nil, errInternal
		}

		val, err := vm.ctx.Global().Get("parseHyperScript")
		if err != nil {
			return nil, nil, errInternal
		}

		vm.parseFunction, err = val.AsFunction()
		if err != nil {
			return nil, nil, errInternal
		}
	}

	resultChan := make(chan *v8go.Value, 1)
	errChan := make(chan error, 1)

	isolate := vm.ctx.Isolate()
	jsString, err := v8go.NewValue(isolate, source)
	if err != nil {
		return nil, nil, errInternal
	}
	defer func() {
		jsString.Release()
	}()

	argObjectTempl := v8go.NewObjectTemplate(isolate)
	argObjectTempl.Set("input", jsString)
	if parseExpr {
		True := utils.Must(v8go.NewValue(isolate, true))
		argObjectTempl.Set("parseExpression", True)

		defer func() {
			True.Release()
		}()
	}

	argObject, err := argObjectTempl.NewInstance(vm.ctx)

	if err != nil {
		return nil, nil, fmt.Errorf("%w: failed to create argument object", errInternal)
	}

	go func() {
		defer utils.Recover()
		result, err := vm.parseFunction.Call(v8go.Null(isolate), argObject)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- result
	}()

	var jsResult *v8go.Value

	select {
	case <-errChan:
		return nil, nil, errInternal
	case jsResult = <-resultChan:
		defer func() {
			jsResult.Release()

			// Put back the vm in the pool AFTER having read the results.
			select {
			case vmPool <- vm:
			default:
			}

		}()
	case <-time.After(TIMEOUT):
		vm := vm.ctx.Isolate()
		vm.TerminateExecution()
		return nil, nil, errors.New("timeout")
	}

	resultObject, err := jsResult.AsObject()

	if err != nil {
		return nil, nil, errInternal
	}

	criticalErrorString, _ := resultObject.Get("criticalError")

	if criticalErrorString != nil {
		defer criticalErrorString.Release()
	}

	if criticalErrorString != nil && !criticalErrorString.IsUndefined() {
		return nil, nil, errors.New(criticalErrorString.String())
	}

	errorJSON, _ := resultObject.Get("errorJSON")

	if errorJSON != nil {
		defer errorJSON.Release()
	}

	if errorJSON != nil && !errorJSON.IsUndefined() {
		_json := errorJSON.String()
		var err hscode.ParsingError
		unmarshallingErr := json.Unmarshal([]byte(_json), &err)
		if unmarshallingErr != nil {
			return nil, nil, fmt.Errorf("internal error: %w", unmarshallingErr)
		}

		err.TokensNoWhitespace = utils.FilterSlice(err.Tokens, isNotWhitespaceToken)

		return nil, &err, nil
	}

	outputJSON, _ := resultObject.Get("outputJSON")

	if outputJSON != nil {
		defer outputJSON.Release()
	}

	if outputJSON != nil && !outputJSON.IsUndefined() {
		_json := outputJSON.String()
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
