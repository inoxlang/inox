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

	DENO_SERVICE_TS = HYPERSCRIPT_PARSER_JS + "\n" + DENO_SERVICE_BASE_TS

	serviceULID              ulid.ULID
	parseHyperscriptWithDeno func(ctx context.Context, input string) (json.RawMessage, error)
	serviceLock              sync.Mutex

	ErrDenoServiceNotAvailable = errors.New("deno service not available")
)

func StartHyperscriptParsingService(
	startService func(program string) (ulid.ULID, error),
	_parseHyperscriptWithDeno func(context context.Context, input string) (json.RawMessage, error),
) error {
	serviceLock.Lock()
	defer serviceLock.Unlock()

	if parseHyperscriptWithDeno != nil {
		return fmt.Errorf("service already started")
	}

	ulid, err := startService(DENO_SERVICE_TS)
	if err != nil {
		return err
	}
	serviceULID = ulid
	parseHyperscriptWithDeno = _parseHyperscriptWithDeno
	return nil
}

func tryParseHyperScriptWithDenoService(ctx context.Context, source string) (*hscode.ParsingResult, *hscode.ParsingError, error) {
	serviceLock.Lock()
	parseHyperscriptWithDeno := parseHyperscriptWithDeno
	serviceLock.Unlock()

	if parseHyperscriptWithDeno == nil {
		return nil, nil, ErrDenoServiceNotAvailable
	}

	if len(source) > DEFAULT_MAX_SOURCE_CODE_LENGTH {
		return nil, nil, ErrInputStringTooLong
	}

	rawJSON, err := parseHyperscriptWithDeno(ctx, source)

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

func getDenoServiceID() (ulid.ULID, bool) {
	serviceLock.Lock()
	defer serviceLock.Unlock()

	return serviceULID, serviceULID != (ulid.ULID{})
}
