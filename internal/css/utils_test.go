package css

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoesClassListStartWithUppercaseLetter(t *testing.T) {
	assert.True(t, DoesClassListStartWithUppercaseLetter("A"))

	assert.False(t, DoesClassListStartWithUppercaseLetter(""))
	assert.False(t, DoesClassListStartWithUppercaseLetter(" "))
	assert.False(t, DoesClassListStartWithUppercaseLetter("a"))
	assert.False(t, DoesClassListStartWithUppercaseLetter(" a"))
	assert.False(t, DoesClassListStartWithUppercaseLetter(" A"))
	assert.False(t, DoesClassListStartWithUppercaseLetter("a A"))
}

func TestGetFirstClassNameInList(t *testing.T) {
	name, ok := GetFirstClassNameInList("a")
	assert.Equal(t, "a", name)
	assert.True(t, ok)

	name, ok = GetFirstClassNameInList("a b")
	assert.Equal(t, "a", name)
	assert.True(t, ok)

	_, ok = GetFirstClassNameInList("")
	assert.False(t, ok)

	_, ok = GetFirstClassNameInList(" ")
	assert.False(t, ok)
}
