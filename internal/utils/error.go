package utils

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

func ConvertPanicValueToError(v any) error {
	if err, ok := v.(error); ok {
		return err
	}

	return fmt.Errorf("%#v", v)
}

// CombineErrors combines errors into a single error with a multiline message.
func CombineErrors(errs ...error) error {

	if len(errs) == 0 {
		return nil
	}

	finalErrBuff := bytes.NewBuffer(nil)

	for _, err := range errs {
		if err != nil {
			finalErrBuff.WriteString(err.Error())
			finalErrBuff.WriteRune('\n')
		}
	}

	return errors.New(strings.TrimRight(finalErrBuff.String(), "\n"))
}

// CombineErrorsWithPrefixMessage combines errors into a single error with a multiline message.
func CombineErrorsWithPrefixMessage(prefixMsg string, errs ...error) error {
	err := CombineErrors(errs...)
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", prefixMsg, err)
}
