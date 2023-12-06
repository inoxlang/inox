package binary

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetLatestReleases(t *testing.T) {

	tags, err := FetchTags()
	if !assert.NoError(t, err) {
		return
	}

	releases, err := GetLatestNReleases(tags, 2)
	if !assert.NoError(t, err) {
		return
	}

	if !assert.Len(t, releases, 2) {
		return
	}

	assert.NotEmpty(t, releases[0].TagName)
	assert.NotEmpty(t, releases[1].TagName)

	assert.NotEmpty(t, releases[0].Name)
	assert.NotEmpty(t, releases[1].Name)
}
