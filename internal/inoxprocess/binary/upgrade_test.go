package binary

import (
	_ "embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	//go:embed fake-inox-amd64.tar.gz
	FAKE_INOX_ARCHIVE []byte
)

func TestInstallInoxBinary(t *testing.T) {
	if testing.Short() {
		return
	}

	dir := t.TempDir()

	binaryPath := filepath.Join(dir, "inox")
	oldPath := filepath.Join(dir, "oldinox")
	tempPath := filepath.Join(dir, "tempinox")

	prevInoxBinaryBytes := []byte("prev")
	err := os.WriteFile(binaryPath, prevInoxBinaryBytes, 0700)

	if !assert.NoError(t, err) {
		return
	}

	err = installInoxBinary(inoxBinaryInstallation{
		path:           binaryPath,
		oldpath:        oldPath,
		tempPath:       tempPath,
		downloadURL:    "https://github.com/inoxlang/inox/releases/download/weekly/inox-weekly-linux-amd64.tar.gz",
		checkExecution: false,
		gzippedTarball: FAKE_INOX_ARCHIVE,
	})

	if !assert.NoError(t, err) {
		return
	}

	if !assert.FileExists(t, binaryPath) {
		return
	}
	assert.FileExists(t, oldPath)
	assert.NoFileExists(t, tempPath)

	currentInoxBinaryBytes, err := os.ReadFile(binaryPath)
	if !assert.NoError(t, err) {
		return
	}

	assert.Equal(t, "new\n", string(currentInoxBinaryBytes))
}
