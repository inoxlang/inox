package cloudflared

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDownloadLatestBinaryFromGithub(t *testing.T) {
	bytes, err := DownloadLatestBinaryFromGithub()
	if !assert.NoError(t, err) {
		return
	}

	assert.Greater(t, len(bytes), 1_000_000)
}
