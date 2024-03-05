package hsparse

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	_ "embed"

	"github.com/inoxlang/inox/internal/hyperscript/hscode"
	"github.com/inoxlang/inox/internal/utils"
	"github.com/oklog/ulid/v2"
)

var (
	//go:embed deno-service-base.ts
	DENO_SERVICE_BASE_TS string

	//Typescript code of the hyperscript parsing service.
	DENO_SERVICE_TS = HYPERSCRIPT_PARSER_JS + "\n" + DENO_SERVICE_BASE_TS

	serviceULID               ulid.ULID
	_parseHyperscriptWithDeno func(ctx context.Context, input string, serviceULID ulid.ULID) (json.RawMessage, error)
	serviceLock               sync.Mutex

	ErrDenoServiceNotAvailable = errors.New("deno service not available")
)

// StartHyperscriptParsingService starts the parsing service by calling $startService and
// registers $parseHyperscriptWithDeno. It saves the service ULID for future invocations of $parseHyperscriptWithDeno.
func StartHyperscriptParsingService(
	startService func(program string) (ulid.ULID, error),
	parseHyperscriptWithDeno func(context context.Context, input string, ulid ulid.ULID) (json.RawMessage, error),
) error {
	serviceLock.Lock()
	defer serviceLock.Unlock()

	if _parseHyperscriptWithDeno != nil {
		return fmt.Errorf("service already started")
	}

	ulid, err := startService(DENO_SERVICE_TS)
	if err != nil {
		return err
	}
	serviceULID = ulid
	_parseHyperscriptWithDeno = parseHyperscriptWithDeno
	return nil
}

func tryParseHyperScriptWithDenoService(ctx context.Context, source string) (*hscode.ParsingResult, *hscode.ParsingError, error) {
	serviceLock.Lock()
	parseHyperscriptWithDeno := _parseHyperscriptWithDeno
	serviceID := serviceULID
	serviceLock.Unlock()

	if parseHyperscriptWithDeno == nil {
		return nil, nil, ErrDenoServiceNotAvailable
	}

	if len(source) > DEFAULT_MAX_SOURCE_CODE_LENGTH {
		return nil, nil, ErrInputStringTooLong
	}

	rawJSON, err := parseHyperscriptWithDeno(ctx, source, serviceID)

	if err != nil {
		return nil, nil, err
	}

	functionResult := map[string]any{}
	unmarshallingErr := json.Unmarshal(rawJSON, &functionResult)

	if unmarshallingErr != nil {
		return nil, nil, fmt.Errorf("internal error: %w", unmarshallingErr)
	}

	criticalError := functionResult["criticalError"]
	if criticalError != nil {
		return nil, nil, errors.New(criticalError.(string))
	}

	errorJSON := functionResult["errorJSON"]
	if errorJSON != nil {
		_json := errorJSON.(string)
		var err hscode.ParsingError
		unmarshallingErr := json.Unmarshal([]byte(_json), &err)
		if unmarshallingErr != nil {
			return nil, nil, fmt.Errorf("internal error: %w", unmarshallingErr)
		}

		err.TokensNoWhitespace = utils.FilterSlice(err.Tokens, isNotWhitespaceToken)

		return nil, &err, nil
	}

	outputJSON := functionResult["outputJSON"]
	if outputJSON != nil {
		_json := outputJSON.(string)
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
