package core

import (
	"math"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNextInt64Float64(t *testing.T) {
	nextInt := nextInt64Float64(reflect.ValueOf(Int(0)))
	assert.Equal(t, Int(1), nextInt.Interface())

	nextFloat := nextInt64Float64(reflect.ValueOf(Float(0)))
	assert.Equal(t, Float(math.Nextafter(0, 1)), nextFloat.Interface())
}
