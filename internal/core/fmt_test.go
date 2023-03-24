package internal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDateFormat(t *testing.T) {
	ctx := NewContext(ContextConfig{})
	NewGlobalState(ctx)

	layout := time.UnixDate
	format := NewDateFormat(layout)
	date := Date(time.Date(2022, 1, 1, 1, 1, 1, 0, time.UTC))

	//check formatting
	res, err := Fmt(ctx, format, date)
	if !assert.NoError(t, err) {
		return
	}

	s := string(res.(Str))
	assert.Equal(t, time.Time(date).Format(layout), s)

	//check parsing
	parsed, err := format.Parse(ctx, s)
	if !assert.NoError(t, err) {
		return
	}
	assert.Equal(t, date, parsed)
}
