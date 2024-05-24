package core

//TODO

// import (
// 	"errors"

// 	pprint "github.com/inoxlang/inox/internal/prettyprint"
// 	utils "github.com/inoxlang/inox/internal/utils/common"
// 	"github.com/muesli/termenv"
// )

// var (
// 	TEST_CASE_RESULT_DARK_MODE_PRETTY_PRINT_CONFIG = &PrettyPrintConfig{
// 		PrettyPrintConfig: pprint.PrettyPrintConfig{
// 			MaxDepth:                    7,
// 			Colorize:                    true,
// 			Colors:                      &pprint.DEFAULT_DARKMODE_PRINT_COLORS,
// 			Compact:                     false,
// 			Indent:                      []byte{' ', ' '},
// 			PrintDecodedTopLevelStrings: true,
// 		},
// 	}
// 	TEST_CASE_RESULT_LIGTH_MODE_PRETTY_PRINT_CONFIG = &PrettyPrintConfig{
// 		PrettyPrintConfig: pprint.PrettyPrintConfig{
// 			MaxDepth:                    7,
// 			Colorize:                    true,
// 			Colors:                      &pprint.DEFAULT_LIGHTMODE_PRINT_COLORS,
// 			Compact:                     false,
// 			Indent:                      []byte{' ', ' '},
// 			PrintDecodedTopLevelStrings: true,
// 		},
// 	}
// )

// type TestCaseResult struct {
// 	Error          error //may wrap an *AssertionError
// 	AssertionError *AssertionError
// 	TestCase       *TestCase

// 	Success                bool   `json:"success"`
// 	DarkModePrettyMessage  string //colorized
// 	LightModePrettyMessage string //colorized
// 	Message                string
// }

// func NewTestCaseResult(ctx *Context, executionResult Value, executionError error, testCase *TestCase) (*TestCaseResult, error) {

// 	var assertionError *AssertionError
// 	isAssertionError := errors.As(executionError, &assertionError)

// 	result := &TestCaseResult{
// 		Error:          executionError,
// 		AssertionError: assertionError,
// 		Success:        executionError == nil,
// 		TestCase:       testCase,
// 	}

// 	red := string(GetFullColorSequence(termenv.ANSIBrightRed, false))
// 	if executionError != nil {
// 		if isAssertionError && assertionError.isTestAssertion {
// 			prefix := red + "FAIL:" + string(ANSI_RESET_SEQUENCE) + " "

// 			result.DarkModePrettyMessage = prefix + assertionError.PrettySPrint(TEST_CASE_RESULT_DARK_MODE_PRETTY_PRINT_CONFIG)
// 			result.LightModePrettyMessage = prefix + assertionError.PrettySPrint(TEST_CASE_RESULT_LIGTH_MODE_PRETTY_PRINT_CONFIG)
// 			result.Message = utils.StripANSISequences(result.DarkModePrettyMessage)
// 		} else {
// 			result.Message = "FAIL: unexpected error: " + utils.StripANSISequences(executionError.Error())
// 		}
// 	} else { //set success message
// 		if testCase.formattedPosition != "" {
// 			result.Message = testCase.formattedPosition
// 		}

// 		result.DarkModePrettyMessage = string(GetFullColorSequence(termenv.ANSIBrightGreen, false)) +
// 			"PASS" + string(ANSI_RESET_SEQUENCE) + " "
// 		result.LightModePrettyMessage = string(GetFullColorSequence(termenv.ANSIGreen, false)) +
// 			"PASS" + string(ANSI_RESET_SEQUENCE) + " "
// 		result.Message = "PASS "
// 	}

// 	name := testCase.name
// 	if name == "" {
// 		name = testCase.formattedPosition
// 		if name == "" {
// 			name = "?"
// 		}
// 	}

// 	result.forEachNotEmptyMessage(func(s string, isDarkMode, isLightMode bool) string {
// 		header := "TEST " + name

// 		if isDarkMode {
// 			color := string(DEFAULT_DARKMODE_DISCRETE_COLOR)
// 			if !result.Success {
// 				color = red
// 			}
// 			header = color + header + ANSI_RESET_SEQUENCE_STRING
// 		} else if isLightMode {
// 			color := string(DEFAULT_LIGHMODE_DISCRETE_COLOR)
// 			if !result.Success {
// 				color = red
// 			}
// 			header = color + header + ANSI_RESET_SEQUENCE_STRING
// 		}

// 		return header + "\n" + s
// 	})

// 	return result, nil
// }

// func (r *TestCaseResult) forEachNotEmptyMessage(fn func(s string, isDarkMode, isLightMode bool) string) {
// 	if r.DarkModePrettyMessage != "" {
// 		r.DarkModePrettyMessage = fn(r.DarkModePrettyMessage, true, false)
// 	}
// 	if r.LightModePrettyMessage != "" {
// 		r.LightModePrettyMessage = fn(r.LightModePrettyMessage, true, false)
// 	}
// 	if r.Message != "" {
// 		r.Message = fn(r.Message, false, false)
// 	}
// }

// type TestSuiteResult struct {
// 	TestSuite       *TestSuite
// 	CaseResults     []*TestCaseResult
// 	SubSuiteResults []*TestSuiteResult

// 	Success bool

// 	DarkModePrettyMessage  string //colorized
// 	LightModePrettyMessage string //colorized
// 	Message                string
// }

// func NewTestSuiteResult(ctx *Context, testCaseResults []*TestCaseResult, subSuiteResults []*TestSuiteResult, testSuite *TestSuite) (*TestSuiteResult, error) {
// 	suiteResult := &TestSuiteResult{
// 		TestSuite:       testSuite,
// 		CaseResults:     testCaseResults,
// 		SubSuiteResults: subSuiteResults,
// 		Success:         true,
// 	}

// 	allHaveDarkModeMessage := true
// 	allHaveLightModeMessage := true

// 	for _, caseResult := range testCaseResults {
// 		if !caseResult.Success {
// 			suiteResult.Success = false
// 		}
// 		if caseResult.DarkModePrettyMessage == "" {
// 			allHaveDarkModeMessage = false
// 		}
// 		if caseResult.LightModePrettyMessage == "" {
// 			allHaveLightModeMessage = false
// 		}
// 	}

// 	for _, subSuiteResult := range subSuiteResults {
// 		if !subSuiteResult.Success {
// 			suiteResult.Success = false
// 		}
// 		if subSuiteResult.DarkModePrettyMessage == "" {
// 			allHaveDarkModeMessage = false
// 		}
// 		if subSuiteResult.LightModePrettyMessage == "" {
// 			allHaveLightModeMessage = false
// 		}
// 	}

// 	//build dark mode message
// 	if allHaveDarkModeMessage {
// 		for _, caseResult := range testCaseResults {
// 			suiteResult.DarkModePrettyMessage += caseResult.DarkModePrettyMessage + "\n\n"
// 		}

// 		for _, subSuiteResult := range subSuiteResults {
// 			suiteResult.DarkModePrettyMessage += subSuiteResult.DarkModePrettyMessage
// 		}
// 	}

// 	//build light mode message
// 	if allHaveLightModeMessage {
// 		for _, caseResult := range testCaseResults {
// 			suiteResult.LightModePrettyMessage += caseResult.LightModePrettyMessage + "\n\n"
// 		}

// 		for _, subSuiteResult := range subSuiteResults {
// 			suiteResult.LightModePrettyMessage += subSuiteResult.LightModePrettyMessage
// 		}
// 	}

// 	//build message
// 	for _, caseResult := range testCaseResults {
// 		suiteResult.Message += caseResult.Message + "\n\n"
// 	}

// 	for _, subSuiteResult := range subSuiteResults {
// 		suiteResult.Message += subSuiteResult.Message + "\n\n"
// 	}

// 	name := suiteResult.TestSuite.nameFromMeta
// 	if name == "" {
// 		if testSuite.module != nil {
// 			name = "(no name) " + testSuite.module.MainChunk.GetFormattedNodeLocation(testSuite.module.MainChunk.Node)
// 		} else {
// 			name = "(anonymous)"
// 		}
// 	}
// 	suiteResult.forEachNotEmptyMessage(func(s string, darkMode, lightMode bool) string {
// 		header := "TEST SUITE " + name
// 		if darkMode {
// 			header = string(DEFAULT_DARKMODE_DISCRETE_COLOR) + header + ANSI_RESET_SEQUENCE_STRING
// 		} else if lightMode {
// 			header = string(DEFAULT_DARKMODE_DISCRETE_COLOR) + header + ANSI_RESET_SEQUENCE_STRING
// 		}

// 		return header + "\n\n" + utils.IndentLines(s, "   ")
// 	})

// 	return suiteResult, nil
// }

// func (r *TestSuiteResult) MostAdaptedMessage(colorized bool, darkBackground bool) string {
// 	if !colorized {
// 		return r.Message
// 	}
// 	if darkBackground && r.DarkModePrettyMessage != "" {
// 		return r.DarkModePrettyMessage
// 	}

// 	if r.LightModePrettyMessage != "" {
// 		return r.LightModePrettyMessage
// 	}
// 	return r.Message
// }

// func (r *TestSuiteResult) forEachNotEmptyMessage(fn func(s string, isDarkMode, isLightMode bool) string) {
// 	if r.DarkModePrettyMessage != "" {
// 		r.DarkModePrettyMessage = fn(r.DarkModePrettyMessage, true, false)
// 	}
// 	if r.LightModePrettyMessage != "" {
// 		r.LightModePrettyMessage = fn(r.LightModePrettyMessage, true, false)
// 	}
// 	if r.Message != "" {
// 		r.Message = fn(r.Message, false, false)
// 	}
// }
