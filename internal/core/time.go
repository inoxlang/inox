package internal

import "time"

// See stdlib's time.Duration, Duration implements Value.
type Duration time.Duration

// See stdlib's time.DuratioTime, Date implements Value.
type Date time.Time
