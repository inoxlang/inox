package utils

import (
	"math"

	"golang.org/x/exp/rand"
)

func RandFloat(low, high float64, seed uint64) float64 {
	pseudoRand := rand.New(rand.NewSource(seed))

	float := pseudoRand.Float64()
	rangeLength := high - low

	if math.IsInf(rangeLength, 0) {
		if float < 0.5 {
			return -float * math.MaxFloat64
		}
		return float * math.MaxFloat64
	}

	return low + float*rangeLength
}
