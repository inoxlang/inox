package globals

import (
	"testing"

	"github.com/inoxlang/inox/internal/core"
	"github.com/stretchr/testify/assert"
)

func TestMinOf(t *testing.T) {
	assert.Equal(t, core.Int(1), MinOf(nil, core.Int(1)))
	assert.Equal(t, core.Int(1), MinOf(nil, core.Int(1), core.Int(2)))
	assert.Equal(t, core.Int(1), MinOf(nil, core.Int(2), core.Int(1)))
}

func TestMaxOf(t *testing.T) {
	assert.Equal(t, core.Int(1), MaxOf(nil, core.Int(1)))
	assert.Equal(t, core.Int(2), MaxOf(nil, core.Int(1), core.Int(2)))
	assert.Equal(t, core.Int(2), MaxOf(nil, core.Int(2), core.Int(1)))
}

func TestMinMaxOf(t *testing.T) {
	min, max := MinMaxOf(nil, core.Int(1))
	assert.Equal(t, core.Int(1), min)
	assert.Equal(t, core.Int(1), max)

	min, max = MinMaxOf(nil, core.Int(1), core.Int(2))
	assert.Equal(t, core.Int(1), min)
	assert.Equal(t, core.Int(2), max)

	min, max = MinMaxOf(nil, core.Int(2), core.Int(1))
	assert.Equal(t, core.Int(1), min)
	assert.Equal(t, core.Int(2), max)
}
