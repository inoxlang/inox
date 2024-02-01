package core

import (
	"errors"
	"time"
)

const (
	ONE_HOUR        = Duration(time.Hour)
	ONE_MINUTE      = Duration(time.Minute)
	ONE_SECOND      = Duration(time.Second)
	ONE_MILLISECOND = Duration(time.Millisecond)
	MAX_DURATION    = Duration(1<<63 - 1)
)

var (
	PROCESS_BEGIN_TIME = time.Now().UTC()

	ErrNegDuration = errors.New("negative duration")
	ErrInvalidYear = errors.New("invalid year")
	ErrInvalidDate = errors.New("invalid date")
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

func (y Year) Validate() error {
	//Check that the underlying time is the start of the year.
	goTime := time.Time(y)
	year := time.Date(goTime.Year(), 1, 1, 0, 0, 0, 0, goTime.Location())
	if !goTime.Equal(year) {
		return ErrInvalidYear
	}
	return nil
}

// Date implements Value.
type Date time.Time

func (d Date) Validate() error {
	//Check that the underlying time is the start of the day.
	goTime := time.Time(d)
	date := time.Date(goTime.Year(), goTime.Month(), goTime.Day(), 0, 0, 0, 0, goTime.Location())
	if !goTime.Equal(date) {
		return ErrInvalidDate
	}
	return nil
}

// See stdlib's time.Time, DateTime implements Value.
type DateTime time.Time

func (t DateTime) AsGoTime() time.Time {
	return time.Time(t)
}

// RelativeTimeInstant64 is a number of milliseconds since PROCESS_BEGIN_TIME.
type RelativeTimeInstant64 int64

// GetRelativeTimeInstant64 returns the current RelativeTimeInstant64 (number of milliseconds since PROCESS_BEGIN_TIME).
func GetRelativeTimeInstant64() RelativeTimeInstant64 {
	delta := time.Since(PROCESS_BEGIN_TIME).Milliseconds()
	return RelativeTimeInstant64(delta)
}

// Time returns an UTC-located time.
func (i RelativeTimeInstant64) Time() time.Time {
	return PROCESS_BEGIN_TIME.Add(time.Duration(i) * time.Millisecond).UTC()
}
