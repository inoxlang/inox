package gen

import (
	"errors"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/afs"
	"github.com/stretchr/testify/assert"
)

func TestPackageWriteTo(t *testing.T) {

	t.Run("base case", func(t *testing.T) {
		pkg := NewPkg("main")
		file := NewFile("main")

		pkg.AddFile("main.go", file.F)

		fls := newMemFilesystem()

		err := pkg.WriteTo(fls, "/")

		if !assert.NoError(t, err) {
			return
		}

		content, err := util.ReadFile(fls, "/main.go")
		if !assert.NoError(t, err) {
			return
		}

		assert.Regexp(t, "package main.*", string(content))
	})

}

func newMemFilesystem() afs.Filesystem {
	fs := memfs.New()

	return afs.AddAbsoluteFeature(fs, func(path string) (string, error) {
		if path[0] == '/' {
			return path, nil
		}
		return "", errors.New("cannot determine absolute path")
	})
}
