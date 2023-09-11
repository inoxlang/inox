package utils

import (
	"fmt"
)

func ConvertPanicValueToError(v any) error {
	if err, ok := v.(error); ok {
		return err
	}

	return fmt.Errorf("%#v", v)
}
