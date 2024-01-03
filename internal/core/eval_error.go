package core

import (
	"errors"
	"fmt"

	"github.com/inoxlang/inox/internal/parse"
)

const (
	S_PATH_EXPR_PATH_LIMITATION                         = "path should not contain the substring '..'"
	S_PATH_SLICE_VALUE_LIMITATION                       = "path slices should have a string or path value"
	S_PATH_INTERP_RESULT_LIMITATION                     = "result of a path interpolation should not contain any of the following substrings: '..', '\\', '*', '?'"
	S_URL_PATH_INTERP_RESULT_LIMITATION                 = "result of a URL path interpolation should not contain any of the following substrings: '..', '\\', '*', '?', '#'"
	S_QUERY_PARAM_VALUE_LIMITATION                      = "value of query parameter should not contain '&' nor '#'"
	S_URL_EXPR_PATH_START_LIMITATION                    = "path should not start with ':'"
	S_URL_EXPR_PATH_LIMITATION                          = "path should not contain any of the following substrings '..', '#', '?"
	S_URL_EXPR_UNEXPECTED_HOST_IN_PARSED_URL_AFTER_EVAL = "unexpected host in parsed URL after evaluation"
	S_INVALID_URL_ENCODED_STRING                        = "invalid URL encoded string"
	S_INVALID_URL_ENCODED_PATH                          = "invalid URL encoded path"
)

var (
	ErrStackOverflow            = errors.New("stack overflow")
	ErrIndexOutOfRange          = errors.New("index out of range")
	ErrInsertionIndexOutOfRange = errors.New("insertion index out of range")
	ErrNegativeLowerIndex       = errors.New("negative lower index")
	ErrUnreachable              = errors.New("unreachable")

	ErrCannotSetValOfIndexKeyProp = errors.New("cannot set value of index key property")
	ErrCannotPopFromEmptyList     = errors.New("cannot pop from an empty list")

	//integer
	ErrIntOverflow          = errors.New("integer overflow")
	ErrIntUnderflow         = errors.New("integer underflow")
	ErrNegationWithOverflow = errors.New("integer negation with overflow")
	ErrIntDivisionByZero    = errors.New("integer division by zero")

	//floating point
	ErrFloatOverflow      = errors.New("float overflow")
	ErrFloatUnderflow     = errors.New("float underflow")
	ErrNaNinfinityOperand = errors.New("NaN or (+|-)infinity operand in floating point operation")
	ErrNaNinfinityResult  = errors.New("result of floating point operation is NaN or (+|-)infinity")

	//quantty
	ErrNegQuantityNotSupported = errors.New("negative quantities are not supported")

	ErrCannotEvaluateCompiledFunctionInTreeWalkEval = errors.New("cannot evaluate compiled function in a tree walk evaluation")
	ErrInvalidQuantity                              = errors.New("invalid quantity")
	ErrQuantityLooLarge                             = errors.New("quantity is too large")

	ErrDirPathShouldEndInSlash     = errors.New("directory's path should end with '/'")
	ErrFilePathShouldNotEndInSlash = errors.New("regular file's path should not end with '/'")

	ErrModifyImmutable           = errors.New("cannot modify an immutable value")
	ErrCannotSetProp             = errors.New("cannot set property")
	ErrCannotLockUnsharableValue = errors.New("cannot lock unsharable value")
	ErrAttemptToSetCaptureGlobal = errors.New("attempt to set a captured global")

	ErrNotImplemented    = errors.New("not implemented and won't be implemented in the near future")
	ErrNotImplementedYet = errors.New("not implemented yet")
	ErrInvalidDirPath    = errors.New("invalid dir path")
	ErrInvalidNonDirPath = errors.New("invalid non-dir path")
	ErrURLAlreadySet     = errors.New("url already set")

	ErrLThreadIsDone = errors.New("lthread is done")

	ErrSelfNotDefined = errors.New("self not defined")

	ErrNotEnoughCliArgs                 = errors.New("not enough CLI arguments")
	ErrMissinggRuntimeTypecheckSymbData = errors.New("impossible to perform runtime typecheck because symbolic data is missing")
	ErrPrecisionLoss                    = errors.New("precision loss")

	ErrValueInExactPatternValueShouldBeImmutable = errors.New("the value in an exact value pattern should be immutable")

	ErrValueHasNoProperties = errors.New("value has no properties")

	ErrNotInDebugMode       = errors.New("not in debug mode")
	ErrStepNonPausedProgram = errors.New("impossible to step in the execution of a non-paused program")
)

func FormatErrPropertyDoesNotExist(name string, v Value) error {
	return fmt.Errorf("property .%s of value %#v does not exist", name, v)
}

func FormatRuntimeTypeCheckFailed(pattern Pattern, ctx *Context) error {
	return fmt.Errorf("runtime type check failed: value does not match the pattern %s", Stringify(pattern, ctx))
}

func fmtTooManyPositionalArgs(positionalArgCount, positionalParamCount int) string {
	return fmt.Sprintf("too many positional arguments were provided (%d), at most %d positional arguments are expected", positionalArgCount, positionalParamCount)
}

func fmtUnknownArgument(name string) string {
	return fmt.Sprintf("unknown argument -%s", name)
}

func FormatIndexableShouldHaveLen(length int) string {
	return fmt.Sprintf("indexable should have a length of %d", length)
}

type LocatedEvalError struct {
	error
	Message  string
	Location parse.SourcePositionStack
}

func (e LocatedEvalError) Unwrap() error {
	return e.error
}

func (err LocatedEvalError) MessageWithoutLocation() string {
	return err.Message
}

func (err LocatedEvalError) LocationStack() parse.SourcePositionStack {
	return err.Location
}
