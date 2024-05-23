package utils

import (
	"crypto/rand"
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandFloat(t *testing.T) {

	t.Run("-math.MaxFloat64 .. math.MaxFloat64", func(t *testing.T) {
		//check infinity is never returned
		for i := 0; i < 1_000_000; i++ {
			seed := Must(rand.Int(rand.Reader, big.NewInt(math.MaxInt64))).Uint64()

			result := RandFloat(-math.MaxFloat64, math.MaxFloat64, seed)
			if !assert.False(t, math.IsInf(result, 0)) {
				return
			}
		}
	})

	t.Run("-0.5*math.MaxFloat64 .. 0.5*math.MaxFloat64", func(t *testing.T) {
		lower := -math.MaxInt64 * 0.5
		upper := math.MaxInt64 * 0.5

		//check returned value is not ouf of bounds
		for i := 0; i < 1_000_000; i++ {
			seed := Must(rand.Int(rand.Reader, big.NewInt(math.MaxInt64))).Uint64()

			result := RandFloat(lower, upper, seed)
			if !assert.False(t, math.IsInf(result, 0)) {
				return
			}

			if !assert.True(t, result >= lower) {
				return
			}
			if !assert.True(t, result <= upper) {
				return
			}
		}
	})

}
