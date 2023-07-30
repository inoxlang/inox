package commonfmt

import (
	"errors"
	"fmt"
)

func FmtByteCount(count int64, _3digitGroupCount int) (string, error) {
	format := "%dB"
	var v = int64(count)

	singleGroup := _3digitGroupCount == 1
	if _3digitGroupCount != 1 && _3digitGroupCount > 0 {
		return "", errors.New("only one 3-digit group is supported for now")
	}

	switch {
	case count >= 1_000_000_000_000 && (singleGroup || count%1_000_000_000_000 == 0):
		format = "%dTB"
		v /= 1_000_000_000_000
	case count >= 1_000_000_000 && (singleGroup || count%1_000_000_000 == 0):
		format = "%dGB"
		v /= 1_000_000_000
	case count >= 1_000_000 && (singleGroup || count%1_000_000 == 0):
		format = "%dMB"
		v /= 1_000_000
	case count >= 1000 && (singleGroup || count%1_000 == 0):
		format = "%dkB"
		v /= 1_000
	case count >= 0:
		break
	case count < 0:
		return "", errors.New("invalid byte rate")
	}
	return fmt.Sprintf(format, v), nil
}
