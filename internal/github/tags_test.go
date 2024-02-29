package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFetchTags(t *testing.T) {

	const repo = "inoxlang/inox"

	if testing.Short() {
		return
	}

	tags, err := FetchTags(repo)
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
