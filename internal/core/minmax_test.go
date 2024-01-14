package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMinOf(t *testing.T) {
	assert.Equal(t, Int(1), MinOf(nil, Int(1)))
	assert.Equal(t, Int(1), MinOf(nil, Int(1), Int(2)))
	assert.Equal(t, Int(1), MinOf(nil, Int(2), Int(1)))
}

func TestMaxOf(t *testing.T) {
	assert.Equal(t, Int(1), MaxOf(nil, Int(1)))
	assert.Equal(t, Int(2), MaxOf(nil, Int(1), Int(2)))
	assert.Equal(t, Int(2), MaxOf(nil, Int(2), Int(1)))
}

func TestMinMaxOf(t *testing.T) {
	min, max := MinMaxOf(nil, Int(1))
	assert.Equal(t, Int(1), min)
	assert.Equal(t, Int(1), max)

	min, max = MinMaxOf(nil, Int(1), Int(2))
	assert.Equal(t, Int(1), min)
	assert.Equal(t, Int(2), max)

	min, max = MinMaxOf(nil, Int(2), Int(1))
	assert.Equal(t, Int(1), min)
	assert.Equal(t, Int(2), max)
}
