package core

import (
	"errors"
	"time"
)

var (
	ErrNegDuration = errors.New("negative duration")
)

// See stdlib's time.Duration, Duration implements Value.
type Duration time.Duration

func (d Duration) Validate() error {
	if d < 0 {
		return ErrNegDuration
	}
	return nil
}

// Year implements Value.
type Year time.Time

// Date implements Value.
type Date time.Time

// See stdlib's time.Time, DateTime implements Value.
type DateTime time.Time
