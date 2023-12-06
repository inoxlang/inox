package compressarch

import (
	_ "embed"
	"io"
	"io/fs"
	"testing"

	"github.com/inoxlang/inox/internal/utils"
	"github.com/stretchr/testify/assert"
)

var (
	//go:embed simple.tar
	SIMPLE_TAR_BYTES []byte
)

func TestUntar(t *testing.T) {
	var entries []fs.FileInfo
	var contents []string

	UntarInMemory(SIMPLE_TAR_BYTES, func(info fs.FileInfo, reader io.Reader) error {
		entries = append(entries, info)
		if !info.IsDir() {
			contents = append(contents, string(utils.Must(io.ReadAll(reader))))
		} else {
			contents = append(contents, "")
		}
		return nil
	})

	if !assert.Len(t, entries, 3) {
		return
	}

	for i, entry := range entries {
		switch entry.Name() {
		case "file1.txt":
			assert.False(t, entry.IsDir())
			assert.Equal(t, "file1\n", contents[i])
		case "file2.txt":
			assert.False(t, entry.IsDir())
			assert.Equal(t, "file2\n", contents[i])
		case "dir":
			assert.True(t, entry.IsDir())
			assert.Empty(t, contents[i])
		}
	}
}
