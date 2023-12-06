package binary

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFetchTags(t *testing.T) {
	tags, err := FetchTags()
	if !assert.NoError(t, err) {
		return
	}

	devFound := false

	for _, tag := range tags {
		if tag.Name == "dev" {
			devFound = true
			break
		}
	}

	assert.True(t, devFound)
}
