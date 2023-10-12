package core

import (
	"errors"

	pprint "github.com/inoxlang/inox/internal/pretty_print"
	"github.com/inoxlang/inox/internal/utils"
)

var (
	TEST_CASE_RESULT_DARK_MODE_PRETTY_PRINT_CONFIG = &PrettyPrintConfig{
		PrettyPrintConfig: pprint.PrettyPrintConfig{
			MaxDepth:                    7,
			Colorize:                    true,
			Colors:                      &pprint.DEFAULT_DARKMODE_PRINT_COLORS,
			Compact:                     false,
			Indent:                      []byte{' ', ' '},
			PrintDecodedTopLevelStrings: true,
		},
	}
	TEST_CASE_RESULT_LIGTH_MODE_PRETTY_PRINT_CONFIG = &PrettyPrintConfig{
		PrettyPrintConfig: pprint.PrettyPrintConfig{
			MaxDepth:                    7,
			Colorize:                    true,
			Colors:                      &pprint.DEFAULT_LIGHTMODE_PRINT_COLORS,
			Compact:                     false,
			Indent:                      []byte{' ', ' '},
			PrintDecodedTopLevelStrings: true,
		},
	}
)

type TestCaseResult struct {
	error          error //may wrap an *AssertionError
	assertionError *AssertionError
	testCase       *TestCase

	Success                bool   `json:"success"`
	DarkModePrettyMessage  string //colorized
	LightModePrettyMessage string //colorized
	Message                string
}

func NewTestCaseResult(ctx *Context, executionResult Value, executionError error, testCase *TestCase) (*TestCaseResult, error) {

	var assertionError *AssertionError
	isAssertionError := errors.As(executionError, &assertionError)

	result := &TestCaseResult{
		error:          executionError,
		assertionError: assertionError,
		Success:        executionError == nil,
		testCase:       testCase,
	}

	if executionError != nil {
		if isAssertionError && assertionError.isTestAssertion {
			result.DarkModePrettyMessage = assertionError.PrettySPrint(TEST_CASE_RESULT_DARK_MODE_PRETTY_PRINT_CONFIG)
			result.LightModePrettyMessage = assertionError.PrettySPrint(TEST_CASE_RESULT_LIGTH_MODE_PRETTY_PRINT_CONFIG)
			result.Message = utils.StripANSISequences(result.DarkModePrettyMessage)
		} else {
			result.Message = utils.StripANSISequences(executionError.Error())
		}
		result.Message = "FAIL " + result.Message
	} else { //set success message
		if testCase.formattedPosition != "" {
			result.Message = "OK " + testCase.formattedPosition
		} else {
			result.Message = "OK "
		}
	}

	return result, nil
}

type TestSuiteResult struct {
	testSuite       *TestSuite
	caseResults     []*TestCaseResult
	subSuiteResults []*TestSuiteResult

	Success bool

	DarkModePrettyMessage  string //colorized
	LightModePrettyMessage string //colorized
	Message                string
}

func NewTestSuiteResult(ctx *Context, testCaseResults []*TestCaseResult, subSuiteResults []*TestSuiteResult, testSuite *TestSuite) (*TestSuiteResult, error) {
	suiteResult := &TestSuiteResult{
		testSuite:       testSuite,
		caseResults:     testCaseResults,
		subSuiteResults: subSuiteResults,
		Success:         true,
	}

	allHaveDarkModeMessage := true
	allHaveLightModeMessage := true

	for _, caseResult := range testCaseResults {
		if !caseResult.Success {
			suiteResult.Success = false
		}
		if caseResult.DarkModePrettyMessage == "" {
			allHaveDarkModeMessage = false
		}
		if caseResult.LightModePrettyMessage == "" {
			allHaveLightModeMessage = false
		}
	}

	for _, subSuiteResult := range subSuiteResults {
		if !subSuiteResult.Success {
			suiteResult.Success = false
		}
		if subSuiteResult.DarkModePrettyMessage == "" {
			allHaveDarkModeMessage = false
		}
		if subSuiteResult.LightModePrettyMessage == "" {
			allHaveLightModeMessage = false
		}
	}

	//build dark mode message
	if allHaveDarkModeMessage {
		for _, caseResult := range testCaseResults {
			suiteResult.DarkModePrettyMessage += caseResult.DarkModePrettyMessage + "\n\n"
		}

		for _, subSuiteResult := range subSuiteResults {
			suiteResult.DarkModePrettyMessage += subSuiteResult.DarkModePrettyMessage + "\n\n"
		}
	}

	//build light mode message
	if allHaveLightModeMessage {
		for _, caseResult := range testCaseResults {
			suiteResult.LightModePrettyMessage += caseResult.LightModePrettyMessage + "\n\n"
		}

		for _, subSuiteResult := range subSuiteResults {
			suiteResult.LightModePrettyMessage += subSuiteResult.LightModePrettyMessage + "\n\n"
		}
	}

	//build message
	for _, caseResult := range testCaseResults {
		suiteResult.Message += caseResult.Message + "\n\n"
	}

	for _, subSuiteResult := range subSuiteResults {
		suiteResult.Message += subSuiteResult.Message + "\n\n"
	}

	return suiteResult, nil
}

func (r *TestSuiteResult) MostAdaptedMessage(colorized bool, darkBackground bool) string {
	if !colorized {
		return r.Message
	}
	if darkBackground && r.DarkModePrettyMessage != "" {
		return r.DarkModePrettyMessage
	}

	if r.LightModePrettyMessage != "" {
		return r.LightModePrettyMessage
	}
	return r.Message
}
