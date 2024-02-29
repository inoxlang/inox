package install

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

func TestInstall(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	//Install the binary.
	dir := t.TempDir()
	location := filepath.Join(dir, "deno")

	err := Install(location)

	if !assert.NoError(t, err) {
		return
	}
	if !assert.FileExists(t, location) {
		return
	}

	stat := utils.Must(os.Stat(location))
	assert.Equal(t, WANTED_FILE_PERMISSIONS, stat.Mode().Perm())
	mtime := stat.ModTime()

	//Installing again should have no effect.
	err = Install(location)

	assert.NoError(t, err)
	if !assert.FileExists(t, location) {
		return
	}
	assert.Equal(t, WANTED_FILE_PERMISSIONS, stat.Mode().Perm())

	stat = utils.Must(os.Stat(location))
	assert.Equal(t, WANTED_FILE_PERMISSIONS, stat.Mode().Perm())
	assert.Equal(t, mtime, stat.ModTime())

	//Modifications to the binary's permissions should be detected.
	os.Chmod(location, 0o600)

	err = Install(location)

	assert.ErrorContains(t, err, "its permissions (unix) are too wide")

	//Modifications to the binary should be detected.
	f, err := os.OpenFile(location, os.O_WRONLY|os.O_APPEND, 0)
	if !assert.NoError(t, err) {
		return
	}

	defer f.Close()

	_, err = f.Write([]byte("x"))
	if !assert.NoError(t, err) {
		return
	}

	os.Chmod(location, WANTED_FILE_PERMISSIONS)

	err = Install(location)

	assert.ErrorContains(t, err, "has not the expected checksum")
}
