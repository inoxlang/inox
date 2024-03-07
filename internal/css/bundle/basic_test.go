package bundle

import (
	"context"
	"testing"

	"github.com/go-git/go-billy/v5/util"
	"github.com/inoxlang/inox/internal/css"
	"github.com/inoxlang/inox/internal/globals/fs_ns"
	"github.com/stretchr/testify/assert"
)

func TestBundle(t *testing.T) {

	t.Run("empty file", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(``), 0600)

		bundleStylesheet, err := Bundle(context.Background(), BundlingParams{
			InputFile:  "/main.css",
			Filesystem: fls,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Empty(t, bundleStylesheet.Children)
	})

	t.Run("non-empty file", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`a {}`), 0600)

		bundleStylesheet, err := Bundle(context.Background(), BundlingParams{
			InputFile:  "/main.css",
			Filesystem: fls,
		})

		if !assert.NoError(t, err) {
			return
		}

		assert.Len(t, bundleStylesheet.Children, 1)
	})

	t.Run("local import", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import "other.css"; a {}`), 0600)
		util.WriteFile(fls, "/other.css", []byte(`div {}`), 0600)

		bundleStylesheet, err := Bundle(context.Background(), BundlingParams{
			InputFile:  "/main.css",
			Filesystem: fls,
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, bundleStylesheet.Children, 2) {
			return
		}

		assert.Equal(t, css.Ruleset, bundleStylesheet.Children[0].Type)
		assert.Equal(t, css.Ruleset, bundleStylesheet.Children[1].Type)
	})

	t.Run("nested local import", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import "other1.css"; html {}`), 0600)
		util.WriteFile(fls, "/other1.css", []byte(`@import "other2.css"; a {}`), 0600)
		util.WriteFile(fls, "/other2.css", []byte(`div {}`), 0600)

		bundleStylesheet, err := Bundle(context.Background(), BundlingParams{
			InputFile:  "/main.css",
			Filesystem: fls,
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, bundleStylesheet.Children, 3) {
			return
		}

		assert.Equal(t, css.Ruleset, bundleStylesheet.Children[0].Type)
		assert.Equal(t, css.Ruleset, bundleStylesheet.Children[1].Type)
		assert.Equal(t, css.Ruleset, bundleStylesheet.Children[2].Type)
	})

	t.Run("file imported twice", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import "other1.css"; @import "other2.css"; html {}`), 0600)
		util.WriteFile(fls, "/other1.css", []byte(`@import "other2.css"; a {}`), 0600)
		util.WriteFile(fls, "/other2.css", []byte(`div {}`), 0600)

		bundleStylesheet, err := Bundle(context.Background(), BundlingParams{
			InputFile:  "/main.css",
			Filesystem: fls,
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, bundleStylesheet.Children, 3) {
			return
		}

		if !assert.Equal(t, css.Ruleset, bundleStylesheet.Children[0].Type) {
			return
		}
		assert.Equal(t, "div", bundleStylesheet.Children[0].Children[0].Children[0].Data)
		assert.Equal(t, css.Ruleset, bundleStylesheet.Children[1].Type)
		assert.Equal(t, css.Ruleset, bundleStylesheet.Children[2].Type)
	})

	t.Run("file imported twice", func(t *testing.T) {
		fls := fs_ns.NewMemFilesystem(1_000)

		util.WriteFile(fls, "/main.css", []byte(`@import "other2.css"; @import "other1.css"; html {}`), 0600)
		util.WriteFile(fls, "/other1.css", []byte(`@import "other2.css"; a {}`), 0600)
		util.WriteFile(fls, "/other2.css", []byte(`div {}`), 0600)

		bundleStylesheet, err := Bundle(context.Background(), BundlingParams{
			InputFile:  "/main.css",
			Filesystem: fls,
		})

		if !assert.NoError(t, err) {
			return
		}

		if !assert.Len(t, bundleStylesheet.Children, 3) {
			return
		}

		if !assert.Equal(t, css.Ruleset, bundleStylesheet.Children[0].Type) {
			return
		}
		assert.Equal(t, "div", bundleStylesheet.Children[0].Children[0].Children[0].Data)
		assert.Equal(t, css.Ruleset, bundleStylesheet.Children[1].Type)
		assert.Equal(t, css.Ruleset, bundleStylesheet.Children[2].Type)
	})
}
