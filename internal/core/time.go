package core

import "time"

// See stdlib's time.Duration, Duration implements Value.
type Duration time.Duration

// Year implements Value.
type Year time.Time

// Date implements Value.
type Date time.Time

// See stdlib's time.Time, DateTime implements Value.
type DateTime time.Time
