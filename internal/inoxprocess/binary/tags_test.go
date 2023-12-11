package binary

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFetchTags(t *testing.T) {

	if testing.Short() {
		return
	}

	tags, err := FetchTags()
	if !assert.NoError(t, err) {
		return
	}

	devFound := false

	for _, tag := range tags {
		if tag.Name == "dev" {
			devFound = true
			assert.Nil(t, tag.Version)
			break
		}
	}

	assert.True(t, devFound)
}
