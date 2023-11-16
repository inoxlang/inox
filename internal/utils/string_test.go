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

	t.Run("irrelevant results", func(t *testing.T) {
		_, _, ok := FindClosestString(context.Background(), []string{"opt"}, "o", 2)
		if !assert.False(t, ok) {
			return
		}

		_, _, ok = FindClosestString(context.Background(), []string{"opt"}, "p", 2)
		if !assert.False(t, ok) {
			return
		}

		_, _, ok = FindClosestString(context.Background(), []string{"opt"}, "t", 2)
		if !assert.False(t, ok) {
			return
		}

		_, _, ok = FindClosestString(context.Background(), []string{"opt"}, "pa", 2)
		if !assert.False(t, ok) {
			return
		}

	})
}

func TestIdentLines(t *testing.T) {
	assert.Equal(t, "", IndentLines("", "ab"))
	assert.Equal(t, "abx", IndentLines("x", "ab"))
	assert.Equal(t, "abx\nabx", IndentLines("x\nx", "ab"))
	assert.Equal(t, "abx\r\nabx", IndentLines("x\r\nx", "ab"))
	assert.Equal(t, "abx\n\rabx", IndentLines("x\n\rx", "ab"))

	//empty line in the middle
	assert.Equal(t, "abx\n\nabx", IndentLines("x\n\nx", "ab"))
}

func TestMinMaxPossibleRuneCount(t *testing.T) {

	min, max := MinMaxPossibleRuneCount(4)
	assert.Equal(t, 1, min)
	assert.Equal(t, 4, max)

	min, max = MinMaxPossibleRuneCount(5)
	assert.Equal(t, 2, min)
	assert.Equal(t, 5, max)

	min, max = MinMaxPossibleRuneCount(6)
	assert.Equal(t, 2, min)
	assert.Equal(t, 6, max)

	min, max = MinMaxPossibleRuneCount(7)
	assert.Equal(t, 2, min)
	assert.Equal(t, 7, max)

	min, max = MinMaxPossibleRuneCount(8)
	assert.Equal(t, 2, min)
	assert.Equal(t, 8, max)

	min, max = MinMaxPossibleRuneCount(9)
	assert.Equal(t, 3, min)
	assert.Equal(t, 9, max)
}
