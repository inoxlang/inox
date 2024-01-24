package core

import (
	"errors"
	"time"
)

var (
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
