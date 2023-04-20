package internal

import (
	"errors"
	"fmt"
)

const (
	S_PATH_EXPR_PATH_LIMITATION      = "path should not contain the substring '..'"
	S_PATH_SLICE_VALUE_LIMITATION    = "path slices should have a string or path value"
	S_PATH_INTERP_RESULT_LIMITATION  = "result of a path interpolation should not contain any of the following substrings: '..', '\\', '*', '?'"
	S_QUERY_PARAM_VALUE_LIMITATION   = "value of query parameter should not contain '&' nor '#'"
	S_URL_EXPR_PATH_START_LIMITATION = "path should not start with ':'"
	S_URL_EXPR_PATH_LIMITATION       = "path should not contain the substring '..'"
)

var (
	ErrStackOverflow            = errors.New("stack overflow")
	ErrIndexOutOfRange          = errors.New("index out of range")
	ErrInsertionIndexOutOfRange = errors.New("insertion index out of range")
	ErrNegativeLowerIndex       = errors.New("negative lower index")
	ErrUnreachable              = errors.New("unreachable")

	//integer
	ErrIntOverflow       = errors.New("integer overflow")
	ErrIntUnderflow      = errors.New("integer underflow")
	ErrIntDivisionByZero = errors.New("integer division by zero")

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

	ErrNotImplemented    = errors.New("not implemented and won't be implemented in the near future")
	ErrNotImplementedYet = errors.New("not implemented yet")
	ErrInvalidDirPath    = errors.New("invalid dir path")
	ErrInvalidNonDirPath = errors.New("invalid non-dir path")
	ErrURLAlreadySet     = errors.New("url already set")

	ErrRoutineIsDone                = errors.New("routine is done")
	ErrAlreadyAttachedToSupersystem = errors.New("value is already attached to a super system")
	ErrNotAttachedToSupersystem     = errors.New("value is not attached to a super system")

	ErrSelfNotDefined = errors.New("self not defined")

	ErrNotEnoughCliArgs = errors.New("not enough CLI arguments")
)

func FormatErrPropertyDoesNotExist(name string, v Value) error {
	return fmt.Errorf("property .%s of value %#v does not exist", name, v)
}
