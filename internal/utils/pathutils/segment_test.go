package pathutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestForEachAbsolutePathSegment(t *testing.T) {

	ForEachAbsolutePathSegment("/", func(string, int, int) error {
		assert.Fail(t, "")
		return nil
	})

	i := 0
	ForEachAbsolutePathSegment("/a", func(segment string, _, _ int) error {
		switch i {
		case 0:
			assert.Equal(t, "a", segment)
		default:
			assert.Fail(t, "")
		}
		i++
		return nil
	})

	i = 0

	ForEachAbsolutePathSegment("/a/", func(segment string, _, _ int) error {
		switch i {
		case 0:
			assert.Equal(t, "a", segment)
		default:
			assert.Fail(t, "")
		}
		i++
		return nil
	})

	i = 0
	ForEachAbsolutePathSegment("/a/b", func(segment string, _, _ int) error {
		switch i {
		case 0:
			assert.Equal(t, "a", segment)
		case 1:
			assert.Equal(t, "b", segment)
		default:
			assert.Fail(t, "")
		}
		i++
		return nil
	})

	i = 0
	ForEachAbsolutePathSegment("/a/b/", func(segment string, _, _ int) error {
		switch i {
		case 0:
			assert.Equal(t, "a", segment)
		case 1:
			assert.Equal(t, "b", segment)
		default:
			assert.Fail(t, "")
		}
		i++
		return nil
	})

	i = 0
	ForEachAbsolutePathSegment("//b/", func(segment string, _, _ int) error {
		switch i {
		case 0:
			assert.Equal(t, "b", segment)
		default:
			assert.Fail(t, "")
		}
		i++
		return nil
	})

	i = 0
	ForEachAbsolutePathSegment("/a//", func(segment string, _, _ int) error {
		switch i {
		case 0:
			assert.Equal(t, "a", segment)
		default:
			assert.Fail(t, "")
		}
		i++
		return nil
	})
}

func TestGetPathSegments(t *testing.T) {
	assert.Empty(t, GetPathSegments("/"))
	assert.Empty(t, GetPathSegments("//"))
	assert.Equal(t, []string{"a"}, GetPathSegments("/a"))
	assert.Equal(t, []string{"a"}, GetPathSegments("a"))
	assert.Equal(t, []string{"a"}, GetPathSegments("/a/"))
	assert.Equal(t, []string{"a"}, GetPathSegments("a/"))
	assert.Equal(t, []string{"a"}, GetPathSegments("//a"))
	assert.Equal(t, []string{"a"}, GetPathSegments("//a/"))
	assert.Equal(t, []string{"dir", "a"}, GetPathSegments("/dir/a"))
	assert.Equal(t, []string{"dir", "a"}, GetPathSegments("//dir/a"))
	assert.Equal(t, []string{"dir", "a"}, GetPathSegments("/dir//a"))
	assert.Equal(t, []string{"dir", "a"}, GetPathSegments("/dir//a/"))
	assert.Equal(t, []string{"dir", "a"}, GetPathSegments("/dir//a//"))
}
