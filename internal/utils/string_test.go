package utils

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindClosestString(t *testing.T) {
	t.Run("ok", func(t *testing.T) {
		s, dist, ok := FindClosestString(context.Background(), []string{"aaa", "bba", "cca"}, "aa", 2)
		if !assert.True(t, ok) {
			return
		}

		assert.Equal(t, 1, dist)
		assert.Equal(t, "aaa", s)
	})

	t.Run("maxDifferences should be respected", func(t *testing.T) {
		_, _, ok := FindClosestString(context.Background(), []string{"aaaaa"}, "aa", 2)
		if !assert.False(t, ok) {
			return
		}
	})
}
