package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArrayPool(t *testing.T) {
	elemSize := 4 //int32

	t.Run("invalid configurations", func(t *testing.T) {
		_, err := NewArrayPool[int32](-1, 0)
		if !assert.ErrorIs(t, err, ErrInvalidPoolConfig) {
			return
		}

		_, err = NewArrayPool[int32](0, -1)
		if !assert.ErrorIs(t, err, ErrInvalidPoolConfig) {
			return
		}

		_, err = NewArrayPool[int32](0, 0)
		if !assert.ErrorIs(t, err, ErrInvalidPoolConfig) {
			return
		}

		_, err = NewArrayPool[int32](4, 0)
		if !assert.ErrorIs(t, err, ErrInvalidPoolConfig) {
			return
		}

		_, err = NewArrayPool[int32](8, 0)
		if !assert.ErrorIs(t, err, ErrInvalidPoolConfig) {
			return
		}

		// not enough elements to make an array of length 2
		_, err = NewArrayPool[int32](4, 2)
		if !assert.ErrorIs(t, err, ErrInvalidPoolConfig) {
			return
		}
	})

	validConfigCases := []struct{ byteCount, arrayLen, expectedTotalArrayCount int }{
		{8, 2, 1},
		{16, 2, 2},
		{32, 2, 4},
		{32, 4, 2},

		// not enough bytes to store an additional element
		{9, 2, 1},
		{10, 2, 1},
		{11, 2, 1},
	}

	for _, testCase := range validConfigCases {
		t.Run(fmt.Sprintf("byte count = %d, array length = %d", testCase.byteCount, testCase.arrayLen), func(t *testing.T) {

			t.Run("get array", func(t *testing.T) {
				pool, err := NewArrayPool[int32](testCase.byteCount, testCase.arrayLen)
				if !assert.NoError(t, err) {
					return
				}

				assert.Equal(t, testCase.expectedTotalArrayCount, pool.TotalArrayCount())
				assert.Equal(t, pool.TotalArrayCount(), pool.AvailableArrayCount())
				avail := pool.AvailableArrayCount()

				array, err := pool.GetArray()
				if !assert.NoError(t, err) {
					return
				}

				assert.Equal(t, testCase.byteCount/elemSize/pool.arrayLen, pool.TotalArrayCount()) //invariant
				assert.Equal(t, avail-1, pool.AvailableArrayCount())

				assert.Len(t, array, testCase.arrayLen)
				for i := range array {
					_ = array[i]
				}
			})

			t.Run("get too many arrays", func(t *testing.T) {
				pool, _ := NewArrayPool[int32](testCase.byteCount, testCase.arrayLen)

				totalArrayCount := pool.TotalArrayCount()

				for i := 0; i <= totalArrayCount; i++ {
					array, err := pool.GetArray()

					if i < totalArrayCount {
						if !assert.NoError(t, err) {
							return
						}
						assert.Equal(t, totalArrayCount-i-1, pool.AvailableArrayCount())

						assert.Len(t, array, testCase.arrayLen)
						for i := range array {
							_ = array[i]
						}
					} else {
						if !assert.ErrorIs(t, err, ErrFullPool) {
							return
						}
						assert.Nil(t, array)
					}
				}
			})

			t.Run("get the maximum number of arrays and release them in same order", func(t *testing.T) {
				pool, _ := NewArrayPool[int32](testCase.byteCount, testCase.arrayLen)

				totalArrayCount := pool.TotalArrayCount()
				arrays := make([][]int32, totalArrayCount)

				for i := 0; i < totalArrayCount; i++ {
					arrays[i], _ = pool.GetArray()
				}

				for i := 0; i < totalArrayCount; i++ {
					err := pool.ReleaseArray(arrays[i])

					if !assert.NoError(t, err) {
						return
					}
				}
			})

			t.Run("get the maximum number of arrays and release them in reverse order", func(t *testing.T) {
				pool, _ := NewArrayPool[int32](testCase.byteCount, testCase.arrayLen)

				totalArrayCount := pool.TotalArrayCount()
				arrays := make([][]int32, totalArrayCount)

				for i := 0; i < totalArrayCount; i++ {
					arrays[i], _ = pool.GetArray()
				}

				for i := totalArrayCount - 1; i >= 0; i-- {
					err := pool.ReleaseArray(arrays[i])

					if !assert.NoError(t, err) {
						return
					}
				}
			})
		})
	}

}
