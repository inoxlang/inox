package binary

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestDownloadArchive(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	assetInfo, archiveInfo, err := GetArchiveAssetInfo()
	if !assert.NoError(t, err) {
		return
	}

	_, p, err := DownloadArchive(archiveInfo, assetInfo)

	if !assert.NoError(t, err) {
		return
	}

	expectedHash := utils.Must(hex.DecodeString(ARCHIVE_MATRIX["linux"]["amd64"].checksum))
	hash := sha256.Sum256(p)

	assert.Equal(t, expectedHash, hash[:])
}
