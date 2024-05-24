package gen

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPackageWriteTo(t *testing.T) {

	t.Run("base case", func(t *testing.T) {
		dir := t.TempDir()

		pkg := NewPkg("main")
		file := NewFile("main")

		pkg.AddFile("main.go", file.F)

		err := pkg.WriteTo(dir)

		if !assert.NoError(t, err) {
			return
		}

		content, err := os.ReadFile(filepath.Join(dir, "/main.go"))
		if !assert.NoError(t, err) {
			return
		}

		assert.Regexp(t, "package main.*", string(content))
	})

}
